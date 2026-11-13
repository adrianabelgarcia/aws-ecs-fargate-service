variable "name" {
  description = "Name prefix for the cluster, repository and KMS alias."
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{2,40}$", var.name))
    error_message = "name must be 3-41 lowercase alphanumeric characters or hyphens, starting with a letter."
  }
}

variable "repository_name" {
  description = "ECR repository name. Defaults to name when null."
  type        = string
  default     = null
}

variable "ecr_keep_last_images" {
  description = "Number of images retained per repository. Must exceed the longest rollback window; CodeDeploy rolls back to a prior task definition that references an older tag."
  type        = number
  default     = 50

  validation {
    condition     = var.ecr_keep_last_images >= 10
    error_message = "ecr_keep_last_images must be at least 10 to keep rollback targets."
  }
}

variable "fargate_base" {
  description = "Base count of tasks pinned to FARGATE (on-demand) in the cluster default strategy."
  type        = number
  default     = 0
}

variable "fargate_weight" {
  description = "Relative weight of on-demand FARGATE tasks in the cluster default strategy."
  type        = number
  default     = 1
}

variable "fargate_spot_weight" {
  description = "Relative weight of FARGATE_SPOT tasks in the cluster default strategy."
  type        = number
  default     = 1
}

variable "kms_deletion_window_days" {
  description = "Waiting period before a KMS key is deleted after scheduling deletion."
  type        = number
  default     = 30

  validation {
    condition     = var.kms_deletion_window_days >= 7 && var.kms_deletion_window_days <= 30
    error_message = "kms_deletion_window_days must be between 7 and 30."
  }
}
