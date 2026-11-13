# 0001: ECS on Fargate instead of EKS for this workload

- Status: Accepted
- Date: 2026-11-13
- Deciders: Platform / SRE

## Context and Problem Statement

We need to run a stateless HTTP API with blue/green releases, autoscaling and
per-service IAM in two environments (staging and prod). The team is small and
the platform carries one service today. Should the runtime be Amazon EKS or
Amazon ECS on AWS Fargate?

## Decision Drivers

- Operational load: who patches nodes, runs the autoscaler, upgrades control planes and add-ons
- Cost floor for a low-traffic service
- Isolation: each task should have its own network identity and IAM role without node-level tricks
- Release mechanics: blue/green with automatic rollback should be a first-class feature
- Portability: avoiding lock-in matters less than shipping reliably

## Considered Options

1. ECS on Fargate
2. EKS with managed node groups
3. EKS with Fargate profiles
4. EC2-backed ECS

## Decision Outcome

Chosen: **ECS on Fargate**.

Fargate removes node management entirely. Each task gets an ENI in a private
subnet, so security groups and IAM roles apply per task. The ECS
`CODE_DEPLOY` deployment controller provides blue/green releases with
automatic rollback on CloudWatch alarms, which is the release model we want.

EKS was rejected for this workload because:

- An EKS control plane costs about $73 per cluster per month before any workload runs. Two clusters make that a fixed cost with no benefit to a single service.
- Node groups, the VPC CNI, add-ons and Kubernetes upgrades are a permanent operational load. None of it pays for itself with one service.
- Blue/green on Kubernetes needs a controller such as Argo Rollouts, which is another component to run and secure.

### Consequences

- Good: no nodes to patch, fewer moving parts, native CodeDeploy integration, per-task IAM and security groups.
- Good: capacity strategy (FARGATE and FARGATE_SPOT) is a first-class setting.
- Bad: no DaemonSets, no privileged workloads, no GPUs on Fargate, and tighter coupling to AWS APIs.
- Bad: fewer ecosystem tools than Kubernetes (service mesh, operators).

### Revisit when

- More than roughly ten services share a platform team, or they need cross-service scheduling and policy engines such as OPA Gatekeeper.
- A workload needs GPUs, DaemonSet agents, or a custom CNI.
- A multi-cloud requirement appears.

## Pros and Cons of the Options

### ECS on Fargate (chosen)

- Good: no node layer, per-task networking and IAM, lowest fixed cost.
- Bad: AWS-specific, less extensibility.

### EKS with managed node groups

- Good: full Kubernetes ecosystem, portable manifests.
- Bad: control-plane fee, node lifecycle, add-on upgrades, more on-call surface.

### EKS with Fargate profiles

- Good: no nodes.
- Bad: same control-plane fee, Fargate-on-EKS limits (no DaemonSets, slower pod start), and a separate blue/green controller is still needed.

### EC2-backed ECS

- Good: cheaper per vCPU at sustained high utilisation, full host access.
- Bad: we would own AMIs, patching, and instance draining. That is the work Fargate removes.

## Links

- [ADR 0002](0002-codedeploy-blue-green-over-rolling.md) for release mechanics
- [ADR 0003](0003-fargate-spot-strategy.md) for capacity mix
