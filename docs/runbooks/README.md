# Runbooks

Each runbook follows the same shape: when to use it, what the alarm means,
how to diagnose, how to mitigate, and how to confirm recovery.

Replace the example values before use:

```bash
export AWS_REGION=us-east-1
export ENV=prod                       # or staging
export CLUSTER=items-api-$ENV
export SERVICE=items-api
export APP=items-api                  # CodeDeploy application
export GROUP=items-api-$ENV           # CodeDeploy deployment group
export BASE_URL=https://items.acme-demo.dev

# Values that come from Terraform outputs. Run from the repository root.
export ALB_SUFFIX=$(terraform -chdir=terraform/envs/$ENV output -raw alb_arn_suffix)
export VPC_ID=$(terraform -chdir=terraform/envs/$ENV output -raw vpc_id)
export PRIVATE_SUBNETS=$(terraform -chdir=terraform/envs/$ENV output -json private_subnet_ids | jq -r 'join(" ")')
export TG_ARN=$(terraform -chdir=terraform/envs/$ENV output -json target_group_arns | jq -r '.blue')   # or .green, whichever is live
```

`TG_ARN` must be the target group that the production listener forwards to.
Check it with `aws elbv2 describe-listeners --load-balancer-arn ...` if a
deployment is in progress.

| Runbook | Use when |
|---------|----------|
| [Deployment rollback](deployment-rollback.md) | A release is bad, or a CodeDeploy deployment is stuck |
| [5xx spike](5xx-spike.md) | `5xx-fast-burn` or `5xx-slow-burn` is in ALARM |
| [Tasks failing to start](tasks-stuck-provisioning.md) | Tasks stay in PROVISIONING or PENDING, or stop immediately |
| [Secret rotation](secret-rotation.md) | The API token must change, or a secret may have leaked |
