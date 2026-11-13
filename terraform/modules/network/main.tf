# Availability zones are pinned by zone ID (stable across accounts) rather than
# discovered by name, so a new AZ appearing in the region cannot change the
# subnet layout of a running environment.
data "aws_availability_zones" "selected" {
  filter {
    name   = "zone-id"
    values = var.zone_ids
  }

  filter {
    name   = "opt-in-status"
    values = ["opt-in-not-required"]
  }
}

data "aws_region" "current" {}

locals {
  azs = slice(sort(data.aws_availability_zones.selected.names), 0, var.az_count)

  # /20 subnets: public tier in the first block of the VPC, private tier after it.
  # Public tier hosts only the ALB; private tier hosts Fargate tasks and endpoints.
  public_subnets  = [for i, _ in local.azs : cidrsubnet(var.vpc_cidr, 4, i)]
  private_subnets = [for i, _ in local.azs : cidrsubnet(var.vpc_cidr, 4, i + 8)]
}

# The default security group is emptied explicitly so nothing can rely on it.
resource "aws_default_security_group" "default" {
  vpc_id = module.vpc.vpc_id

  # No ingress or egress blocks: the default group allows nothing.
}

# VPC built with the community module. Design notes:
#   - No NAT gateway. Fargate tasks reach AWS APIs (ECR, CloudWatch Logs,
#     Secrets Manager, S3) through VPC endpoints, see endpoints.tf and ADR 0004.
#   - Default security group and default NACL are managed (emptied) so nothing
#     can accidentally rely on them.
#   - Flow logs are enabled for ALL traffic and encrypted with a CMK.
module "vpc" {
  # checkov:skip=CKV_TF_1: Registry module pinned by version constraint and recorded in .terraform.lock.hcl. Commit-SHA pinning applies to git sources.
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 6.7"

  # checkov:skip=CKV2_AWS_12: The default security group is managed by aws_default_security_group.default in this module with no rules. Checkov cannot follow the reference across the module boundary.
  # checkov:skip=CKV2_AWS_19: enable_nat_gateway is false, so the module creates no NAT EIP. Checkov evaluates the module's EIP count without that input and reports an address that is never allocated.

  name = var.name
  cidr = var.vpc_cidr
  azs  = local.azs

  public_subnets  = local.public_subnets
  private_subnets = local.private_subnets

  enable_nat_gateway = false
  single_nat_gateway = false

  enable_dns_hostnames = true
  enable_dns_support   = true

  manage_default_security_group = false
  manage_default_network_acl    = true

  enable_flow_log                                 = true
  create_flow_log_cloudwatch_log_group            = true
  create_flow_log_cloudwatch_iam_role             = true
  flow_log_traffic_type                           = "ALL"
  flow_log_max_aggregation_interval               = 60
  flow_log_cloudwatch_log_group_retention_in_days = var.flow_log_retention_days
  flow_log_cloudwatch_log_group_kms_key_id        = var.flow_log_kms_key_arn
}
