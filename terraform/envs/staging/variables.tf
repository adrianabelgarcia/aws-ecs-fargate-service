variable "aws_region" {
  description = "AWS region for every resource in this environment."
  type        = string
  default     = "us-east-1"

  validation {
    condition     = can(regex("^[a-z]{2}(-gov)?-[a-z]+-[0-9]$", var.aws_region))
    error_message = "aws_region must be a valid AWS region name."
  }
}

variable "expected_account_id" {
  description = "AWS account ID this root must run in. Enforced by the provider's allowed_account_ids."
  type        = string

  validation {
    condition     = can(regex("^[0-9]{12}$", var.expected_account_id))
    error_message = "expected_account_id must be a 12-digit account ID."
  }
}

variable "environment" {
  description = "Environment name. Fixed to staging for this root."
  type        = string
  default     = "staging"

  validation {
    condition     = var.environment == "staging"
    error_message = "This root is staging only."
  }
}

variable "service_name" {
  description = "Service name, shared with the application repository."
  type        = string
  default     = "items-api"
}

variable "vpc_cidr" {
  description = "VPC CIDR for this environment. Each environment uses a distinct range so peering stays possible."
  type        = string
}

variable "zone_ids" {
  description = "Availability zone IDs for subnet placement. Pinned by ID so a region change cannot shift subnets."
  type        = list(string)
  default     = ["use1-az1", "use1-az2", "use1-az4"]
}

variable "az_count" {
  description = "Number of availability zones."
  type        = number
  default     = 2
}

variable "domain_name" {
  description = "Public hostname for the service, for example items.staging.acme-demo.dev."
  type        = string
}

variable "certificate_arn" {
  description = "ACM certificate ARN for domain_name, in the same region as the ALB."
  type        = string
}

variable "route53_zone_id" {
  description = "Hosted zone ID for domain_name. Leave empty to manage DNS outside this root."
  type        = string
  default     = ""
}

variable "bootstrap_image_tag" {
  description = "Image tag used for the first task definition. CI deploys real SHA tags afterwards."
  type        = string
  default     = "bootstrap"
}

variable "test_listener_allowed_cidrs" {
  description = "CIDRs allowed to reach the CodeDeploy test listener. Empty keeps it closed."
  type        = list(string)
  default     = []
}

variable "deletion_protection" {
  description = "Protects the ALB from deletion."
  type        = bool
  default     = false
}

variable "alarm_email_endpoints" {
  description = "Email subscriptions for alarm notifications. Supply through a private tfvars file, never commit real addresses."
  type        = list(string)
  default     = []
}

variable "capacity_provider_strategy" {
  description = "Service capacity strategy. Staging is spot-heavy to keep cost low."
  type = list(object({
    capacity_provider = string
    base              = number
    weight            = number
  }))
  default = [
    { capacity_provider = "FARGATE", base = 1, weight = 1 },
    { capacity_provider = "FARGATE_SPOT", base = 0, weight = 4 },
  ]
}

variable "deployment_config_name" {
  description = "CodeDeploy deployment configuration."
  type        = string
  default     = "CodeDeployDefault.ECSAllAtOnce"
}

variable "task_cpu" {
  description = "Fargate task CPU units."
  type        = number
  default     = 256
}

variable "task_memory" {
  description = "Fargate task memory in MiB."
  type        = number
  default     = 512
}

variable "desired_count" {
  description = "Initial task count."
  type        = number
  default     = 2
}

variable "min_capacity" {
  description = "Minimum tasks under autoscaling."
  type        = number
  default     = 2
}

variable "max_capacity" {
  description = "Maximum tasks under autoscaling."
  type        = number
  default     = 10
}

variable "log_retention_days" {
  description = "CloudWatch retention for application, WAF and flow logs."
  type        = number
  default     = 365
}
