# 0003: Fargate Spot capacity strategy per environment

- Status: Accepted
- Date: 2026-11-13
- Deciders: Platform / SRE

## Context and Problem Statement

Fargate Spot is roughly 70% cheaper than on-demand Fargate, but AWS can
reclaim a Spot task with a two-minute warning. How much of each environment
should run on Spot, and how do we keep the service available when Spot
capacity is short or a task is reclaimed?

## Decision Drivers

- Cost: Fargate is most of the compute bill
- Availability: prod must keep its SLO through reclaims and Spot shortages
- Simplicity: the strategy should be one setting that operators can read
- Graceful termination: tasks must drain on SIGTERM

## Considered Options

1. Fargate on-demand only
2. Spot-heavy everywhere
3. Per-environment mix with an on-demand base

## Decision Outcome

Chosen: **option 3.**

| Environment | Strategy | Rationale |
|-------------|----------|-----------|
| staging | FARGATE base 1 (weight 1), FARGATE_SPOT weight 4 | Staging can tolerate interruption. Beyond the one on-demand base task, roughly four of every five tasks land on Spot. |
| prod | FARGATE base 2 (weight 1), FARGATE_SPOT weight 1 | The base of two on-demand tasks keeps capacity through a Spot shortage. Beyond the base, load splits evenly. |

The base is a count, not a percentage. Two on-demand tasks always run in prod,
so a Spot shortage degrades capacity but cannot take the service down.

How interruptions are handled:

- ECS sends SIGTERM and the app's shutdown sequence starts (see `internal/app/app.go`): readiness goes to 503, the ALB stops routing, in-flight requests finish, and the process exits within `stopTimeout` (30 s).
- Spot tasks are spread across three AZs in prod. Losing one AZ of Spot capacity does not remove a third of the fleet.
- Autoscaling restores the desired count on the other capacity provider when Spot is unavailable.
- The `FARGATE` base is part of the service's capacity provider strategy, so it is never reduced by a Spot shortage.

### Consequences

- Good: staging costs about $12 per month in compute (two tasks) against about $18 all-on-demand, and prod about $41 (three tasks) against about $54 all-on-demand, at list prices.
- Good: a Spot reclaim is treated as a normal, drained shutdown and does not page anyone unless the 5xx or unhealthy-host alarms fire.
- Bad: during a Spot shortage, capacity above the base can be missing for a while. Autoscaling max and alarms make that visible.
- Bad: two capacity types complicate capacity forecasting. The dashboard shows running tasks by service, not by provider.

### Revisit when

- Spot reclaims exceed the SLO error budget. The first response is to raise the on-demand base, not to remove Spot.
- Traffic grows enough that a Savings Plan for Fargate is cheaper than the Spot mix.

## Pros and Cons of the Options

### On-demand only

- Good: maximum stability.
- Bad: highest cost for no reliability gain that the base-plus-Spot model does not already deliver.

### Spot-heavy everywhere

- Good: lowest cost.
- Bad: prod could lose most capacity in one Spot shortage. Unacceptable for an availability SLO.

### Per-environment mix (chosen)

- Good: cost where interruption is tolerable, floor where it is not.
- Bad: two configurations to keep in sync (they live in `envs/*/*.tfvars`, and the cluster default strategy is derived from the same variable).

## Links

- [SLO](../slo.md)
- [Runbook: tasks stuck in PROVISIONING](../runbooks/tasks-stuck-provisioning.md)
