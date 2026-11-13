variable "name" {
  description = "Name prefix for the ALB, WAF and log resources."
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{2,24}$", var.name))
    error_message = "name must be 3-25 lowercase alphanumeric characters or hyphens (ALB names are capped at 32 characters)."
  }
}

variable "vpc_id" {
  description = "VPC that hosts the ALB."
  type        = string
}

variable "vpc_cidr" {
  description = "VPC CIDR. Egress from the ALB is restricted to this range on the target port."
  type        = string
}

variable "public_subnet_ids" {
  description = "Public subnets for the internet-facing ALB. At least two AZs are required."
  type        = list(string)

  validation {
    condition     = length(var.public_subnet_ids) >= 2
    error_message = "An ALB requires at least two public subnets."
  }
}

variable "target_port" {
  description = "Container port on the tasks. The ALB may only egress to this port inside the VPC."
  type        = number
}

variable "test_listener_port" {
  description = "Port for the CodeDeploy test listener that serves the green target group during a deployment."
  type        = number
  default     = 8443
}

variable "test_listener_allowed_cidrs" {
  description = "CIDRs allowed to reach the test listener. Empty by default: the test listener is closed until you allow your CI runners or VPN."
  type        = list(string)
  default     = []
}

variable "enable_deletion_protection" {
  description = "Prevents the ALB from being deleted through the API or console. Enable in prod."
  type        = bool
  default     = true
}

variable "access_log_retention_days" {
  description = "Days to keep ALB access logs in S3 before expiry."
  type        = number
  default     = 90
}

variable "waf_rate_limit" {
  description = "Requests per 5-minute window from a single IP before WAF blocks it."
  type        = number
  default     = 2000

  validation {
    condition     = var.waf_rate_limit >= 100
    error_message = "waf_rate_limit must be at least 100 (the WAF minimum is 100)."
  }
}

variable "waf_log_retention_days" {
  description = "Retention for WAF request logs in CloudWatch Logs."
  type        = number
  default     = 365
}

variable "kms_key_arn" {
  description = "Platform CMK, used for WAF log encryption. ALB access logs cannot use KMS (see access_logs.tf)."
  type        = string
}
