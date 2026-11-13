# Runbook: Tasks failing to start or stuck in PROVISIONING

**Symptoms:** `runningCount` is below `desiredCount`, tasks stay in
`PROVISIONING` or `PENDING`, or tasks start and stop within a minute. The
`unhealthy-hosts` alarm may fire at the same time.

**Severity:** P2 when running tasks still serve the SLO. P1 when running tasks
drop to zero or the service is degraded.

## 1. Read the service events and stop reasons

```bash
aws ecs describe-services --cluster "$CLUSTER" --services "$SERVICE" \
  --query 'services[0].{desired:desiredCount,running:runningCount,pending:pendingCount,events:events[0:8].message}' \
  --output yaml

aws ecs list-tasks --cluster "$CLUSTER" --service-name "$SERVICE" --desired-status STOPPED --max-items 5
aws ecs describe-tasks --cluster "$CLUSTER" --tasks TASK_ARN_1 TASK_ARN_2 \
  --query 'tasks[].{status:lastStatus,stopped:stoppedReason,containers:containers[].{name:name,reason:reason}}'
```

Match the stop reason to a branch below.

## 2. Branch by message

### `CannotPullContainerError` or `i/o timeout` on the registry

The task cannot reach ECR. Check the endpoints and the task security group.

```bash
aws ec2 describe-vpc-endpoints --filters Name=vpc-id,Values="$VPC_ID" \
  --query 'VpcEndpoints[].{service:ServiceName,state:State,subnets:SubnetIds}' --output table
```

- `ecr.api`, `ecr.dkr`, `logs`, `secretsmanager` must all be `available` in every private subnet's AZ.
- The S3 gateway endpoint must be attached to the private route tables.
- Confirm the endpoint security group still allows 443 from the VPC CIDR.
- Confirm the task security group egresses to the endpoint security group on 443.

### `ResourceInitializationError: unable to pull secrets` or `AccessDenied` on Secrets Manager

- The secret exists, has a value (`AWSCURRENT`), and the ARN in the task definition matches it:

  ```bash
  aws secretsmanager describe-secret --secret-id "items-api-${ENV}/api-token" \
    --query '{arn:ARN,deleted:DeletedDate,versions:VersionIdsToStages}'
  ```

- A secret created by Terraform has no value until an operator sets one. This is the most common cause on a first deployment. Set it with the [secret rotation runbook](secret-rotation.md) and start tasks again.
- The execution role's `kms:Decrypt` condition requires `kms:ViaService = secretsmanager.<region>.amazonaws.com`. If someone changed the key policy, restore it through Terraform.

### `CannotStartContainerError` or the container exits immediately

- Check the application log group for the stopped task:

  ```
  fields @timestamp, msg, err
  | sort @timestamp desc
  | limit 20
  ```

- `invalid configuration` lines mean an environment variable or the `API_TOKEN` secret is missing or too short. In prod, `APP_ENV=prod` requires `API_TOKEN` of at least 16 characters. The process exits with code 2 by design, so a bad config never receives traffic.
- A read-only root filesystem error means the code tries to write outside `/tmp`. The task definition sets `readonlyRootFilesystem`, so fix the code, not the setting.

### `RESOURCE:ENI` or no capacity available

Fargate could not place the task. Subnets might be out of IP addresses.

```bash
aws ec2 describe-subnets --subnet-ids $PRIVATE_SUBNETS \
  --query 'Subnets[].{id:SubnetId,az:AvailabilityZone,free:AvailableIpAddressCount}' --output table
```

Each Fargate task uses one IP. A `/20` subnet has about 4,000 usable addresses,
which is ample. A smaller subnet, or a leak of ENIs from a failed deployment, is
the usual cause. Free addresses by stopping leftover tasks from failed deployments.

### `FARGATE_SPOT` capacity unavailable (or stuck tasks in staging)

Spot capacity can be short. The on-demand base keeps prod running. In staging,
it is acceptable to wait. To force on-demand while Spot recovers, temporarily
set `FARGATE_SPOT` weight to 0 in the environment's tfvars and apply. Revert after.

### Tasks start but fail `/readyz`

The ALB will mark them unhealthy and replace them. Read the application log for
the failing route. Readiness fails by design during shutdown, so a task that
keeps stopping right after start usually points at a dependency or config issue.

## 3. Mitigate while investigating

- If running tasks still serve traffic, leave them alone. ECS keeps retrying to fill the desired count.
- If a release started the failures, roll back with the [deployment rollback runbook](deployment-rollback.md). That is faster than debugging a broken image.
- If the cause is a capacity provider, prod has its on-demand base and should keep serving.

## 4. Verify

```bash
aws ecs describe-services --cluster "$CLUSTER" --services "$SERVICE" \
  --query 'services[0].{desired:desiredCount,running:runningCount}'
aws elbv2 describe-target-health --target-group-arn "$TG_ARN" \
  --query 'TargetHealthDescriptions[].TargetHealth.State'
```

`running` equals `desired` and every target is `healthy`.

## Related

- [ADR 0004: VPC endpoints over NAT](../adr/0004-vpc-endpoints-over-nat.md)
- [ADR 0003: Fargate Spot strategy](../adr/0003-fargate-spot-strategy.md)
- [5xx spike](5xx-spike.md)
