# 0002: CodeDeploy blue/green instead of rolling updates

- Status: Accepted
- Date: 2026-11-13
- Deciders: Platform / SRE

## Context and Problem Statement

A release of the API must not reach users before it is proven healthy, and a
bad release must come back out automatically. ECS offers a rolling deployment
(`ECS` controller with a circuit breaker) and blue/green through the `CODE_DEPLOY`
controller. Which one should prod use?

## Decision Drivers

- Rollback speed: time from alarm to traffic back on the old version
- Blast radius: how many users see the new version before it is judged
- Verification: a place to run checks before live traffic moves
- Alarm integration: automatic stop and rollback driven by the same alarms that page on-call
- Cost during a release

## Considered Options

1. ECS rolling update with the deployment circuit breaker
2. ECS `CODE_DEPLOY` blue/green with a canary configuration
3. Blue/green implemented by hand (two services and a listener swap script)

## Decision Outcome

Chosen: **option 2, CodeDeploy blue/green with a canary in prod and an
all-at-once shift in staging.**

- Two target groups (blue and green) behind one ALB. The new tasks register in the idle group.
- The test listener (port 8443) points at green during a deployment, so checks can run before users see it.
- Prod uses `CodeDeployDefault.ECSCanary10Percent5Minutes`. 10% of traffic moves to green for five minutes. The alarms are watched during that window and the rest of the traffic only moves if they stay green.
- `auto_rollback_configuration` rolls back on `DEPLOYMENT_FAILURE` and `DEPLOYMENT_STOP_ON_ALARM`. The alarms are the same fast-burn 5xx alarm, the p99 latency alarm and the unhealthy-hosts alarm that page on-call.
- After success, the old blue tasks stay for `termination_wait_minutes` (5). Rolling back during that window is a listener change, not a redeploy.

### Consequences

- Good: rollback during the canary is automatic and takes seconds. Old tasks survive the termination wait.
- Good: the canary limits blast radius to about 10% of requests for five minutes in prod.
- Bad: a release needs roughly double capacity while both groups run. Autoscaling max and account quotas must allow it.
- Bad: Terraform and CodeDeploy both touch the service, so the service needs `lifecycle { ignore_changes }` on `task_definition`, `load_balancer` and `desired_count`. Without it, `terraform apply` would undo a traffic shift. The comment in `terraform/modules/ecs-service/service.tf` explains each attribute.
- Bad: the ALB alarms are scoped to the load balancer, not to the green group. A deployment judges the blended traffic of both groups. That is acceptable for a single-service ALB. A per-group alarm would need dynamic recreation during each release.
- Bad: no lifecycle hooks are wired yet. Adding a Lambda `AfterAllowTestTraffic` hook is the next step, and it is listed in the README.

### Why not rolling

ECS rolling updates replace tasks in place behind one target group. There is no
separate verification window, so new and old versions serve traffic together
during the rollout. A circuit breaker stops a bad rollout, but it does not move
traffic away from the new version the way a blue/green switch does, and it does
not give a canary.

## Pros and Cons of the Options

### ECS rolling with circuit breaker

- Good: no extra capacity, simple, native to ECS.
- Bad: mixed versions serve traffic, no canary, rollback means another rollout.

### CODE_DEPLOY blue/green (chosen)

- Good: canary, test listener, instant rollback, alarm-driven stops.
- Bad: double capacity during release, extra lifecycle configuration.

### Hand-rolled blue/green

- Good: full control.
- Bad: we would re-implement CodeDeploy's traffic shifting and its failure handling, and carry the bugs ourselves.

## Links

- [Runbook: deployment rollback](../runbooks/deployment-rollback.md)
- [ADR 0001](0001-fargate-over-eks.md)
