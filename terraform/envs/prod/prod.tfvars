aws_region          = "us-east-1"
expected_account_id = "222222222222"
environment         = "prod"
az_count            = 3

vpc_cidr        = "10.30.0.0/16"
domain_name     = "items.acme-demo.dev"
certificate_arn = "arn:aws:acm:us-east-1:222222222222:certificate/00000000-0000-0000-0000-000000000000"
route53_zone_id = ""

task_cpu      = 512
task_memory   = 1024
desired_count = 3
min_capacity  = 3
max_capacity  = 20

capacity_provider_strategy = [
  { capacity_provider = "FARGATE", base = 2, weight = 1 },
  { capacity_provider = "FARGATE_SPOT", base = 0, weight = 1 },
]
deployment_config_name = "CodeDeployDefault.ECSCanary10Percent5Minutes"
deletion_protection    = true
log_retention_days     = 365
