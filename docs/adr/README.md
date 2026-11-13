# Architecture Decision Records

Decisions use the [MADR](https://adr.github.io/madr/) template. Each record is
immutable once accepted. A reversal is a new record that supersedes the old one.

| ADR | Title | Status |
|-----|-------|--------|
| [0001](0001-fargate-over-eks.md) | ECS on Fargate instead of EKS for this workload | Accepted |
| [0002](0002-codedeploy-blue-green-over-rolling.md) | CodeDeploy blue/green instead of rolling updates | Accepted |
| [0003](0003-fargate-spot-strategy.md) | Fargate Spot capacity strategy per environment | Accepted |
| [0004](0004-vpc-endpoints-over-nat.md) | VPC interface endpoints instead of a NAT gateway | Accepted |
