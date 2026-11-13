# 0004: VPC interface endpoints instead of a NAT gateway

- Status: Accepted
- Date: 2026-11-13
- Deciders: Platform / SRE

## Context and Problem Statement

Fargate tasks in private subnets need to reach AWS APIs: ECR for images,
CloudWatch Logs for logs, Secrets Manager for secrets, and S3 for image layers.
The usual answer is a NAT gateway. Should tasks reach those APIs through NAT,
or through VPC endpoints (AWS PrivateLink and a gateway endpoint)?

## Decision Drivers

- Monthly fixed cost per AZ
- Data processing cost per GB
- Egress surface: what a compromised task can reach on the internet
- Operational complexity

## Considered Options

1. NAT gateway per AZ
2. Interface endpoints for ECR (api and dkr), CloudWatch Logs and Secrets Manager, plus the S3 gateway endpoint, with no NAT
3. A single NAT gateway shared across AZs

## Decision Outcome

Chosen: **option 2.** The network module (`terraform/modules/network`) creates
no NAT gateway. The tasks have no route to the internet.

Cost comparison in us-east-1, list prices, 730 hours per month:

| | NAT gateway | Interface endpoints |
|---|---|---|
| Fixed, staging (2 AZs) | 2 × $32.85 = $65.70 | 4 endpoints × 2 AZs × $7.30 = $58.40 |
| Fixed, prod (3 AZs) | 3 × $32.85 = $98.55 | 4 endpoints × 3 AZs × $7.30 = $87.60 |
| Data processing | $0.045 per GB | $0.01 per GB (S3 gateway endpoint: free) |

Fixed costs are similar, and endpoints win on data processing, which grows with
image pulls and log volume. The decisive factor is egress: with no NAT there is
no path from a task to the public internet, so a compromised task cannot
exfiltrate data to an arbitrary host. Tasks reach only the AWS services they
need, over private DNS names.

Other controls that depend on this decision:

- The task security group allows egress only to the endpoint security group on 443, the S3 prefix list on 443, and the VPC resolver on 53.
- The endpoint security group accepts 443 only from the VPC CIDR.
- The default security group and default NACL are emptied, so nothing relies on them.
- VPC flow logs capture ALL traffic, so denied or unexpected egress is visible.

### Consequences

- Good: no internet egress path, lower data-processing cost, one fewer component to run.
- Good: endpoint policies can restrict which buckets and repositories a VPC may reach.
- Bad: every new AWS API a task calls needs a new endpoint (for example `kms` when a secret uses a customer-managed key outside Secrets Manager's own use, or `ssm` for Parameter Store). Each costs about $7.30 per AZ per month.
- Bad: third-party APIs cannot be reached. A workload that needs one needs either a NAT or an egress proxy, and that is a new ADR.
- Bad: interface endpoints add a dependency on the endpoint's health. A regional endpoint incident blocks image pulls and log delivery, and the runbook covers that.

## Pros and Cons of the Options

### NAT gateway per AZ

- Good: reaches any internet endpoint, simple to reason about.
- Bad: allows general egress, higher data-processing cost.

### Interface endpoints and gateway endpoint (chosen)

- Good: no internet route, private DNS, lower data cost, auditable by endpoint policy.
- Bad: per-endpoint, per-AZ fixed cost, and one endpoint per AWS API used.

### Shared NAT across AZs

- Good: lowest fixed cost of the NAT options.
- Bad: a single-AZ dependency, and still general egress.

## Links

- [Architecture](../architecture.md) (network diagram)
- [Runbook: tasks stuck in PROVISIONING](../runbooks/tasks-stuck-provisioning.md)
