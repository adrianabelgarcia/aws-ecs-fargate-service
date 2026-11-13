# Service Level Objectives

Service: `items-api`. Environment for the SLOs: prod. Staging uses the same
alarms with the same thresholds, so a staging alarm is a rehearsal of prod.

## 1. Objectives

| SLO | SLI (what is measured) | Target | Window |
|-----|------------------------|--------|--------|
| Availability | Share of requests that do not return 5xx, measured at the ALB | **99.9%** | rolling 30 days |
| Latency | Share of 1-minute periods in which p99 target response time is at most 1.0 s | **99%** | rolling 30 days |

Definitions:

- **Availability SLI** = `1 - (HTTPCode_Target_5XX_Count + HTTPCode_ELB_5XX_Count) / RequestCount`, summed over the window.
  4xx responses are client errors and count as successes. A rejected bad request is the service working.
- **Latency SLI** uses `TargetResponseTime` with the `p99` statistic, one-minute periods, and the load balancer dimension. The ALB measures time to the task, so it excludes time spent outside the VPC.
- Both SLIs come from the ALB, the closest point to the user that AWS exposes. A failure between the client and the ALB (DNS, TLS handshake) is outside the SLO.

## 2. Error budget arithmetic

| Quantity | Calculation | Value |
|----------|-------------|-------|
| Error budget (availability) | 1 − 0.999 | 0.1% of requests |
| Budget in minutes of full outage | 0.001 × 30 × 24 × 60 | 43.2 minutes per 30 days |
| Latency budget | 1 − 0.99 | 1% of one-minute periods, about 7.2 hours per 30 days |

**Burn rate** is how fast the budget is consumed relative to the rate that would
exactly exhaust it at the end of the window:

```
burn rate = observed error ratio / (1 − SLO) = observed error ratio / 0.001
```

A burn rate of 1 uses the whole budget in exactly 30 days. A burn rate of 14.4
uses 2% of the monthly budget in one hour (14.4 × 1 h ÷ 720 h = 0.02).

## 3. Burn-rate alarms and their thresholds

The alarms are defined in `terraform/modules/ecs-service/alarms.tf`. The
thresholds are computed from `availability_slo`, `fast_burn_rate` and
`slow_burn_rate` in `terraform/modules/ecs-service/locals.tf`. Changing the SLO
changes the alarms, and nothing else needs to be edited.

| Alarm | Burn rate | Error ratio threshold | Period and evaluation | Budget consumed by the time it fires | Action |
|-------|-----------|-----------------------|-----------------------|--------------------------------------|--------|
| `5xx-fast-burn` | 14.4× | 14.4 × 0.1% = **1.44%** | 1 min periods, 2 of last 3 | about 2% per hour sustained | Page on-call. Stop and roll back any CodeDeploy deployment. |
| `5xx-slow-burn` | 6× | 6 × 0.1% = **0.6%** | 5 min periods, 12 of 12 (one hour) | about 0.8% per hour sustained | Open a ticket. |
| `latency-p99` | n/a | p99 > **1.0 s** | 1 min periods, 2 of last 3 | n/a | Page on-call. Also used as a CodeDeploy rollback alarm. |
| `unhealthy-hosts` | n/a | ≥ **1** unhealthy target for 3 of 3 minutes | 1 min periods | n/a | Page on-call. Also used as a CodeDeploy rollback alarm. |

Design notes:

- **Traffic floor.** The 5xx expression is `IF(requests < 20, 0, 100 × errors ÷ requests)`. Without it, one failing request out of two at 3 a.m. would page. The floor means a 5xx ratio is judged only when there is enough traffic to judge it.
- **Two windows, one per severity.** A fast burn needs a short window to catch an outage in minutes. A slow leak needs a long window to avoid paging on noise. The 1-hour sustained condition approximates the multi-window burn-rate method without needing metric math over rolling sums.
- **Shared alarms for release control.** The same fast-burn, latency and unhealthy alarms stop CodeDeploy deployments. A release that burns budget during its canary is rolled back by the same signal that would wake someone up.
- **Load balancer scope.** The alarms read ALB metrics at the load balancer level, so both target groups count. This keeps alarms valid across the blue/green switch (see [ADR 0002](adr/0002-codedeploy-blue-green-over-rolling.md)).

## 4. Error budget policy

The budget is measured over the rolling 30-day window and reviewed every week.

| Budget remaining | Release policy |
|------------------|----------------|
| More than 50% | Normal cadence. Any change that passes the pipeline can ship. |
| 25% to 50% | Changes to the request path need a note in the PR describing the failure mode and the rollback. Large changes wait for a staging soak of at least one day. |
| Less than 25% | Feature releases pause. Only reliability fixes, security fixes and rollbacks ship. |
| Exhausted | Feature freeze until the budget recovers above 0% at the next review. Reliability work gets priority. A review of the causes is required before the freeze ends. |

Additional rules:

- Any single incident that consumes more than 20% of the monthly budget needs a written postmortem.
- Planned maintenance that breaches the SLO is still counted. There are no exempt windows.
- A deployment rolled back automatically by CodeDeploy counts toward the budget for the minutes it was above the thresholds, and it is reviewed like any other incident.

## 5. How to check the budget

Availability over the last 30 days, from CloudWatch metric math (for example in the Metrics Insights console, or in a dashboard widget):

```
100 * (1 - (SUM(target5xx) + SUM(elb5xx)) / SUM(requests))
```

Where `requests` is `AWS/ApplicationELB RequestCount`, `target5xx` is
`HTTPCode_Target_5XX_Count`, and `elb5xx` is `HTTPCode_ELB_5XX_Count`, each summed
over the window with the `LoadBalancer` dimension set to the prod ALB. A result
below 99.9 means the budget is spent.

Budget consumed in minutes is `(1 − availability) × 43 200` for a 30-day window.

## 6. Limits of these SLOs

- The ALB sees the 5xx it returns. A bug that returns HTTP 200 with a wrong body is not counted. The smoke test and the business-level checks are the tool for that.
- The latency SLI measures the ALB to task round trip, not the client's experience. Client-side RUM would be the next step.
- A regional outage of the ALB itself is outside this service's control. A multi-region design would change the SLO target; see the README.
