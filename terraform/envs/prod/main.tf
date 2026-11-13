# Composition root. The same file serves staging and prod; the environment
# differs only in tfvars and backend.hcl.

locals {
  env_name = "${var.service_name}-${var.environment}"

  # The on-demand base and weights for the cluster default strategy are read
  # from the service strategy so the two never disagree.
  fargate_base        = try([for s in var.capacity_provider_strategy : s.base if s.capacity_provider == "FARGATE"][0], 0)
  fargate_weight      = try([for s in var.capacity_provider_strategy : s.weight if s.capacity_provider == "FARGATE"][0], 1)
  fargate_spot_weight = try([for s in var.capacity_provider_strategy : s.weight if s.capacity_provider == "FARGATE_SPOT"][0], 0)
}

module "platform" {
  source = "../../modules/platform"

  name                = local.env_name
  repository_name     = var.service_name
  fargate_base        = local.fargate_base
  fargate_weight      = local.fargate_weight
  fargate_spot_weight = local.fargate_spot_weight
}

module "network" {
  source = "../../modules/network"

  name                    = local.env_name
  vpc_cidr                = var.vpc_cidr
  az_count                = var.az_count
  zone_ids                = var.zone_ids
  flow_log_retention_days = var.log_retention_days
  flow_log_kms_key_arn    = module.platform.kms_key_arn
}

# The token is a container, not a value. Terraform never sees the secret
# string, so it never reaches state. Operators set the value out-of-band (see
# docs/runbooks/secret-rotation.md).
resource "aws_secretsmanager_secret" "api_token" {
  # checkov:skip=CKV2_AWS_57: Automatic rotation needs a rotation Lambda that calls the token issuer, which this reference service does not ship. Rotation is a documented manual runbook.
  name                    = "${local.env_name}/api-token"
  description             = "Bearer token protecting write endpoints of ${var.service_name}. Value is set out-of-band."
  kms_key_id              = module.platform.kms_key_arn
  recovery_window_in_days = 7
}

module "edge" {
  source = "../../modules/edge"

  name                        = local.env_name
  vpc_id                      = module.network.vpc_id
  vpc_cidr                    = module.network.vpc_cidr
  public_subnet_ids           = module.network.public_subnet_ids
  target_port                 = 8080
  test_listener_allowed_cidrs = var.test_listener_allowed_cidrs
  enable_deletion_protection  = var.deletion_protection
  access_log_retention_days   = 90
  waf_log_retention_days      = var.log_retention_days
  kms_key_arn                 = module.platform.kms_key_arn
}

module "service" {
  source = "../../modules/ecs-service"

  name        = var.service_name
  environment = var.environment

  cluster_name                = module.platform.cluster_name
  vpc_id                      = module.network.vpc_id
  vpc_cidr                    = module.network.vpc_cidr
  private_subnet_ids          = module.network.private_subnet_ids
  endpoints_security_group_id = module.network.endpoints_security_group_id
  s3_prefix_list_id           = module.network.s3_prefix_list_id

  alb_arn               = module.edge.alb_arn
  alb_arn_suffix        = module.edge.alb_arn_suffix
  alb_security_group_id = module.edge.alb_security_group_id
  certificate_arn       = var.certificate_arn
  test_listener_port    = 8443

  repository_arn  = module.platform.repository_arn
  kms_key_arn     = module.platform.kms_key_arn
  container_image = "${module.platform.repository_url}:${var.bootstrap_image_tag}"
  container_port  = 8080

  task_cpu    = var.task_cpu
  task_memory = var.task_memory

  environment_variables = {
    APP_ENV           = var.environment
    SERVICE_NAME      = var.service_name
    LOG_LEVEL         = "info"
    MAX_PAGE_SIZE     = "100"
    SHUTDOWN_TIMEOUT  = "20s"
    READY_DRAIN_DELAY = "5s"
  }

  secrets = {
    API_TOKEN = aws_secretsmanager_secret.api_token.arn
  }

  desired_count              = var.desired_count
  min_capacity               = var.min_capacity
  max_capacity               = var.max_capacity
  capacity_provider_strategy = var.capacity_provider_strategy
  deployment_config_name     = var.deployment_config_name
  log_retention_days         = var.log_retention_days
  alarm_email_endpoints      = var.alarm_email_endpoints
}

resource "aws_route53_record" "service" {
  count = var.route53_zone_id == "" ? 0 : 1

  zone_id = var.route53_zone_id
  name    = var.domain_name
  type    = "A"

  alias {
    name                   = module.edge.alb_dns_name
    zone_id                = module.edge.alb_zone_id
    evaluate_target_health = true
  }
}
