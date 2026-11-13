variable "name" {
  description = "Name prefix applied to every network resource, for example items-api-staging."
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{2,40}$", var.name))
    error_message = "name must be 3-41 lowercase alphanumeric characters or hyphens, starting with a letter."
  }
}

variable "vpc_cidr" {
  description = "IPv4 CIDR block for the VPC. Must be a /16 so that /20 subnets can be carved out per tier."
  type        = string

  validation {
    condition     = can(cidrhost(var.vpc_cidr, 0)) && endswith(var.vpc_cidr, "/16")
    error_message = "vpc_cidr must be a valid IPv4 /16 block."
  }
}

variable "zone_ids" {
  description = "Availability zone IDs eligible for subnets, in preference order. The first az_count entries are used."
  type        = list(string)
  default     = ["use1-az1", "use1-az2", "use1-az4"]

  validation {
    condition     = length(var.zone_ids) >= 3 && alltrue([for z in var.zone_ids : can(regex("^[a-z]{2,4}[0-9]?-az[0-9]+$", z))])
    error_message = "zone_ids must contain at least three valid zone IDs such as use1-az1."
  }
}

variable "az_count" {
  description = "Number of availability zones to span. Three in prod for quorum-grade resilience, two in staging."
  type        = number
  default     = 3

  validation {
    condition     = var.az_count >= 2 && var.az_count <= 3
    error_message = "az_count must be 2 or 3."
  }
}

variable "flow_log_retention_days" {
  description = "Retention for VPC flow logs in CloudWatch Logs."
  type        = number
  default     = 365
}

variable "flow_log_kms_key_arn" {
  description = "KMS key ARN used to encrypt the flow log group."
  type        = string
}
