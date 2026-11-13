## Summary

<!-- What changes, and why. Link the issue or ADR when the change is a design decision. -->

## Type of change

- [ ] Application code (`app/`)
- [ ] Infrastructure (`terraform/`)
- [ ] Deployment or pipeline (`deploy/`, `.github/workflows/`)
- [ ] Documentation only

## Risk and rollout

- [ ] No user-visible behaviour change
- [ ] Terraform plan reviewed: no unexpected replacements of the ECS service, listeners or ALB
- [ ] A rollback path exists (CodeDeploy rolls back automatically on alarm; state how a manual rollback works if this changes infrastructure)
- [ ] Alarm thresholds, SLOs or runbooks updated if behaviour changed

## Test plan

- [ ] `make check` passes locally (app tests with coverage gate, lint, render tests, terraform fmt/validate/tflint/checkov)
- [ ] Other:

## Checklist

- [ ] No secrets, account IDs or personal data in the diff
- [ ] Any Checkov skip has an inline justification
