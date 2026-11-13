# Architecture

This document shows the three views a reviewer usually asks for: where the
traffic goes (network), what happens to one request (request flow), and how a
release reaches users (deployment flow). Decisions behind each view are in
[docs/adr](adr/README.md).

## 1. Network

Traffic enters through an internet-facing ALB in public subnets. Fargate tasks
run in private subnets with no internet route. They reach AWS services through
interface endpoints and an S3 gateway endpoint (see [ADR 0004](adr/0004-vpc-endpoints-over-nat.md)).

```mermaid
flowchart TB
  client([Clients])

  subgraph edge["Edge (regional)"]
    waf["AWS WAF web ACL<br/>managed rules, anonymous IP, rate limit"]
  end

  subgraph vpc["VPC /16, three AZs (prod), no NAT"]
    subgraph pub["Public subnets"]
      alb["Application Load Balancer<br/>443 TLS, 80 redirects, test listener 8443"]
    end
    subgraph priv["Private subnets"]
      tasks["Fargate tasks<br/>port 8080, no public IP<br/>read-only root filesystem"]
      vpce["Interface endpoints<br/>ECR API, ECR DKR, CloudWatch Logs, Secrets Manager"]
    end
    s3gw["S3 gateway endpoint<br/>(ECR image layers)"]
    flow["VPC flow logs<br/>ALL traffic, CMK"]
  end

  client -->|HTTPS| waf
  waf --> alb
  alb -->|"8080, from ALB SG only"| tasks
  tasks -->|"443, endpoint SG"| vpce
  tasks -->|"443, S3 prefix list"| s3gw
  vpc -.-> flow
```

Security groups in one line each:

- **ALB SG:** in 443 and 80 from the internet, in 8443 only from allowed CIDRs (empty by default). Out to the VPC CIDR on the container port.
- **Task SG:** in 8080 from the ALB SG only. Out to the endpoint SG on 443, the S3 prefix list on 443, and the VPC resolver on 53.
- **Endpoint SG:** in 443 from the VPC CIDR only.
- **Default SG and default NACL:** emptied. Nothing uses them.

## 2. Request flow

A `GET /api/v1/items` request, and a write that requires the bearer token.

```mermaid
sequenceDiagram
  autonumber
  participant C as Client
  participant A as ALB (WAF attached)
  participant T as Fargate task (API)
  participant L as CloudWatch Logs

  C->>A: HTTPS request (TLS 1.2 minimum, TLS 1.3 preferred)
  Note over A: Drop invalid header fields. Evaluate WAF rules and rate limit.
  alt blocked by WAF
    A-->>C: 403
  else allowed
    A->>T: HTTP 8080 to the active target group
    Note over T: Request ID, security headers, access log, metrics
    alt write endpoint
      Note over T: Constant-time bearer token check (POST only)
    end
    T->>L: JSON access log line (no query string)
    T-->>A: JSON envelope
    A-->>C: Response
  end
```

Operational notes:

- The ALB only forwards to the target group the production listener points at. During a deployment that is blue or green, depending on the phase.
- The access log line carries `route` as the ServeMux pattern (for example `GET /api/v1/items/{id}`), so log and metric cardinality stays bounded.
- Readiness (`/readyz`) returns 503 during shutdown, so the ALB stops sending new requests before the task exits.

## 3. Deployment flow

Each release is a CodeDeploy blue/green deployment. Staging moves all traffic at
once. Prod canaries 10% for five minutes and then shifts the rest. Alarms stop
and roll back either path (see [ADR 0002](adr/0002-codedeploy-blue-green-over-rolling.md)).

```mermaid
sequenceDiagram
  autonumber
  participant Dev as Developer
  participant GH as GitHub Actions
  participant ECR as Amazon ECR
  participant ECS as ECS service
  participant CD as CodeDeploy
  participant ALB as ALB listeners
  participant Al as CloudWatch alarms

  Dev->>GH: merge to main
  GH->>GH: lint, race tests, coverage gate, hadolint
  GH->>GH: build image once, Trivy scan, SBOM
  GH->>ECR: push image tagged with commit SHA
  GH->>ECS: read live task definition, register new revision
  GH->>CD: create deployment with AppSpec
  CD->>ECS: start green tasks in green target group
  CD->>ALB: test listener 8443 to green
  GH->>GH: smoke test staging
  CD->>ALB: prod canary, 10% to green for 5 minutes
  Al-->>CD: fast burn, latency or unhealthy hosts in ALARM?
  alt alarm during deployment
    CD->>ALB: shift traffic back to blue
    CD->>ECS: stop green tasks
  else healthy
    CD->>ALB: shift 100% to green
    CD->>ECS: terminate blue tasks after 5 minutes
  end
```

Two details that reviewers ask about:

- **Why the pipeline registers the task definition itself.** CodeDeploy needs a concrete revision in the AppSpec. `deploy/render-taskdef.sh` starts from the live revision, so secrets, roles and sizing from Terraform carry through. Only the image and `SERVICE_VERSION` change.
- **Why Terraform does not undo a release.** The service ignores `task_definition`, `load_balancer` and `desired_count` in its lifecycle block, so `terraform apply` never reverts a traffic shift or a revision.

## 4. Alarm and release control

```mermaid
flowchart LR
  m["ALB metrics<br/>RequestCount, 5xx, TargetResponseTime, host counts"] --> a1["5xx-fast-burn"]
  m --> a2["5xx-slow-burn"]
  m --> a3["latency-p99"]
  m --> a4["unhealthy-hosts"]
  a1 --> sns["SNS alerts topic (CMK)"]
  a2 --> sns
  a3 --> sns
  a4 --> sns
  a1 -. rollback signal .-> cd["CodeDeploy deployment group"]
  a3 -. rollback signal .-> cd
  a4 -. rollback signal .-> cd
  sns --> oncall["On-call and tickets"]
```

Thresholds and their derivation are in the [SLO document](slo.md).

## Related

- [Runbooks](runbooks/README.md)
- [ADRs](adr/README.md)
