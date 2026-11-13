# Runbook: 5xx spike

**Alarm:** `items-api-<env>-5xx-fast-burn` (page) or `items-api-<env>-5xx-slow-burn` (ticket).
The fast alarm fires when the 5xx ratio stays above `14.4 × 0.1% = 1.44%` for
two of the last three minutes. The slow alarm fires when it stays above
`6 × 0.1% = 0.6%` for an hour. See [the SLO](../slo.md) for the arithmetic.

**Severity:** P1 for the fast alarm. P3 for the slow alarm, unless it is still firing an hour later.

## 1. Confirm the signal is real

Check the ratio on the dashboard `items-api-<env>-service`, panel "5xx percent".
A small request count can produce a noisy ratio. The alarm already ignores
periods with fewer than 20 requests, so a firing alarm is a real spike.

Split target 5xx from ELB 5xx. The two point at different causes:

```bash
END=$(date -u +%FT%TZ); START=$(date -u -v-30M +%FT%TZ 2>/dev/null || date -u -d '30 min ago' +%FT%TZ)
aws cloudwatch get-metric-statistics --namespace AWS/ApplicationELB \
  --metric-name HTTPCode_Target_5XX_Count --dimensions Name=LoadBalancer,Value="$ALB_SUFFIX" \
  --start-time "$START" --end-time "$END" --period 60 --statistics Sum
aws cloudwatch get-metric-statistics --namespace AWS/ApplicationELB \
  --metric-name HTTPCode_ELB_5XX_Count --dimensions Name=LoadBalancer,Value="$ALB_SUFFIX" \
  --start-time "$START" --end-time "$END" --period 60 --statistics Sum
```

- **Target 5xx high:** the application is failing. Go to step 2.
- **ELB 5xx high (502, 503, 504):** the ALB cannot reach healthy targets. Go to step 4.

## 2. Find which routes and requests fail

Run this in CloudWatch Logs Insights against `/ecs/items-api` (the
application log group is `/ecs/items-api`):

```
fields @timestamp, route, status, request_id, duration_ms, err
| filter status >= 500
| stats count() as errors, avg(duration_ms) as avg_ms by route, bin(5m)
| sort errors desc
```

Pull one example with its request ID and search for the stack or error:

```
fields @timestamp, msg, request_id, err, path
| filter msg = "request failed" or msg = "panic recovered"
| sort @timestamp desc
| limit 20
```

Application errors are logged with `request_id`, and the client sees only a generic message.

## 3. Was a release involved?

```bash
aws deploy list-deployments --application-name "$APP" --deployment-group-name "$GROUP" \
  --include-only-statuses InProgress Succeeded --max-items 3
```

If a deployment finished in the last hour, follow the [deployment rollback runbook](deployment-rollback.md). A rollback is usually faster than a fix.

## 4. ELB 5xx: targets unhealthy or missing

```bash
aws elbv2 describe-target-health --target-group-arn "$TG_ARN" \
  --query 'TargetHealthDescriptions[].{target:Target.Id,state:TargetHealth.State,reason:TargetHealth.Reason}'
```

- `Target.FailedHealthChecks`: the readiness path `/readyz` is returning non-200. Check whether the instance is draining or the dependency it reports is down.
- `Target.NotRegistered` or no targets: tasks are not running. See [tasks failing to start](tasks-stuck-provisioning.md).
- Targets healthy but 503 from the ALB: the target group is empty for the active listener. Check the listener default action points at a populated group.

## 5. Mitigate

In order of preference:

1. Roll back the most recent release (step 3).
2. Scale out if the cause is load: raise `max_capacity` in the environment's tfvars and apply, or set the service desired count through autoscaling bounds.
3. Block an abusive client at the edge: add an IP set to the WAF web ACL, or tighten `waf_rate_limit`. Do this only when the traffic pattern is clearly hostile.

Do not disable WAF rules as a mitigation. If a managed rule blocks legitimate traffic, switch that single rule to count mode and open an issue.

## 6. Verify

- `5xx-fast-burn` returns to OK and the ratio is below 0.1% for 15 minutes.
- Error logs for the affected route show no new entries.
- Record the window (start and end) in the incident note; it feeds the error budget calculation in the [SLO](../slo.md).

## Related

- [SLO and burn-rate alarms](../slo.md)
- [Deployment rollback](deployment-rollback.md)
