aws_region          = "us-east-1"
expected_account_id = "111111111111"
environment         = "staging"
az_count            = 2

vpc_cidr        = "10.20.0.0/16"
domain_name     = "items.staging.acme-demo.dev"
certificate_arn = "arn:aws:acm:us-east-1:111111111111:certificate/00000000-0000-0000-0000-000000000000"
route53_zone_id = ""

task_cpu      = 256
task_memory   = 512
desired_count = 2
min_capacity  = 1
max_capacity  = 4

capacity_provider_strategy = [
  { capacity_provider = "FARGATE", base = 1, weight = 1 },
  { capacity_provider = "FARGATE_SPOT", base = 0, weight = 4 },
]
deployment_config_name = "CodeDeployDefault.ECSAllAtOnce"
deletion_protection    = false
log_retention_days     = 90
