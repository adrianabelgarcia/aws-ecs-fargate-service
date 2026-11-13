variable "name" {
  description = "Service name. Also the container name, task family, log group suffix and CodeDeploy resource prefix. Capped so blue/green target group names stay within 32 characters."
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{2,23}$", var.name))
    error_message = "name must be 3-24 lowercase alphanumeric characters or hyphens, starting with a letter."
  }
}

variable "environment" {
  description = "Deployment environment. Used in alarm and dashboard names."
  type        = string

  validation {
    condition     = contains(["staging", "prod"], var.environment)
    error_message = "environment must be staging or prod."
  }
}

variable "cluster_name" {
  description = "Name of the ECS cluster that hosts the service."
  type        = string
}

variable "vpc_id" {
  description = "VPC ID for the target groups and task security group."
  type        = string
}

variable "vpc_cidr" {
  description = "VPC CIDR, used for DNS egress from tasks to the VPC resolver."
  type        = string
}

variable "private_subnet_ids" {
  description = "Private subnets for Fargate tasks."
  type        = list(string)
}

variable "endpoints_security_group_id" {
  description = "Security group of the interface VPC endpoints. Tasks may egress to it on 443."
  type        = string
}

variable "s3_prefix_list_id" {
  description = "Prefix list of the S3 gateway endpoint. Tasks may egress to it on 443 for ECR image layers."
  type        = string
}

variable "alb_arn" {
  description = "ARN of the ALB created by the edge module."
  type        = string
}

variable "alb_arn_suffix" {
  description = "ARN suffix of the ALB, used as the LoadBalancer metric dimension."
  type        = string
}

variable "alb_security_group_id" {
  description = "Security group of the ALB. Task ingress is restricted to it."
  type        = string
}

variable "certificate_arn" {
  description = "ACM certificate ARN for the HTTPS listeners. Must cover the public hostname."
  type        = string

  validation {
    condition     = can(regex("^arn:aws[a-zA-Z-]*:acm:[a-z0-9-]+:[0-9]{12}:certificate/", var.certificate_arn))
    error_message = "certificate_arn must be an ACM certificate ARN."
  }
}

variable "ssl_policy" {
  description = "TLS security policy for the HTTPS listeners. TLS 1.2 minimum, TLS 1.3 preferred."
  type        = string
  default     = "ELBSecurityPolicy-TLS13-1-2-2021-06"
}

variable "test_listener_port" {
  description = "Port of the CodeDeploy test listener that routes to the green target group during deployments."
  type        = number
  default     = 8443
}

variable "repository_arn" {
  description = "ECR repository ARN. The execution role may pull only from this repository."
  type        = string
}

variable "kms_key_arn" {
  description = "Platform CMK. Encrypts the log group and alarm topic, and is used by the execution role to decrypt secrets."
  type        = string
}

variable "container_image" {
  description = "Full image reference, including tag or digest. Used for the initial task definition; CodeDeploy registers new revisions on each release."
  type        = string
}

variable "container_port" {
  description = "Port the API listens on inside the container."
  type        = number
  default     = 8080
}

variable "task_cpu" {
  description = "Fargate task CPU units. Must be a valid Fargate size."
  type        = number
  default     = 256

  validation {
    condition     = contains([256, 512, 1024, 2048, 4096], var.task_cpu)
    error_message = "task_cpu must be one of 256, 512, 1024, 2048, 4096."
  }
}

variable "task_memory" {
  description = "Fargate task memory in MiB. Must be valid for task_cpu."
  type        = number
  default     = 512
}

variable "environment_variables" {
  description = "Plain (non-secret) environment variables for the container."
  type        = map(string)
  default     = {}
}

variable "secrets" {
  description = "Map of container environment variable name to Secrets Manager or SSM Parameter Store ARN. Values are resolved by ECS at task start and never stored in the task definition."
  type        = map(string)
  default     = {}

  validation {
    condition     = alltrue([for arn in values(var.secrets) : can(regex("^arn:aws[a-zA-Z-]*:(secretsmanager|ssm):", arn))])
    error_message = "Every secrets value must be a Secrets Manager or SSM Parameter Store ARN."
  }
}

variable "task_role_policy_json" {
  description = "Optional inline policy for the task role. Leave null when the application makes no AWS API calls."
  type        = string
  default     = null
}

variable "health_check_path" {
  description = "Readiness path used by the ALB target groups. Must return 200 only when the instance accepts traffic."
  type        = string
  default     = "/readyz"
}

variable "health_check_grace_period_seconds" {
  description = "Seconds ECS ignores ALB health failures after a task starts."
  type        = number
  default     = 30
}

variable "stop_timeout_seconds" {
  description = "Seconds between SIGTERM and SIGKILL. Must exceed READY_DRAIN_DELAY plus SHUTDOWN_TIMEOUT in the app."
  type        = number
  default     = 30
}

variable "deregistration_delay_seconds" {
  description = "ALB deregistration delay for target groups."
  type        = number
  default     = 30
}

variable "desired_count" {
  description = "Initial task count. Autoscaling owns the count after creation."
  type        = number
  default     = 2
}

variable "min_capacity" {
  description = "Minimum task count for autoscaling."
  type        = number
  default     = 2
}

variable "max_capacity" {
  description = "Maximum task count for autoscaling."
  type        = number
  default     = 10
}

variable "capacity_provider_strategy" {
  description = "Capacity provider strategy for the service. Prod keeps a FARGATE base for availability and spreads the rest onto FARGATE_SPOT."
  type = list(object({
    capacity_provider = string
    base              = number
    weight            = number
  }))
  default = [
    { capacity_provider = "FARGATE", base = 2, weight = 1 },
  ]
}

variable "deployment_config_name" {
  description = "CodeDeploy deployment configuration. Prod uses a canary (10% for 5 minutes, then the rest) so alarms can stop the rollout early."
  type        = string
  default     = "CodeDeployDefault.ECSAllAtOnce"
}

variable "termination_wait_minutes" {
  description = "Minutes to keep the old (blue) tasks after a successful deployment, so a fast manual rollback is possible."
  type        = number
  default     = 5
}

variable "cpu_target_percent" {
  description = "Target average CPU utilisation for target tracking."
  type        = number
  default     = 60
}

variable "requests_per_target" {
  description = "Target ALB requests per healthy task for target tracking."
  type        = number
  default     = 500
}

variable "availability_slo" {
  description = "Availability SLO as a fraction, for example 0.999. Drives the 5xx alarm thresholds."
  type        = number
  default     = 0.999

  validation {
    condition     = var.availability_slo > 0.9 && var.availability_slo < 1
    error_message = "availability_slo must be between 0.9 and 1 (exclusive)."
  }
}

variable "fast_burn_rate" {
  description = "Burn-rate multiplier for the paging alarm. 14.4x consumes 2% of a 30-day budget in one hour."
  type        = number
  default     = 14.4
}

variable "slow_burn_rate" {
  description = "Burn-rate multiplier for the ticket alarm, sustained for one hour. 6x consumes 5% of a 30-day budget in six hours."
  type        = number
  default     = 6
}

variable "latency_p99_threshold_seconds" {
  description = "p99 target response time that triggers the latency alarm and the CodeDeploy rollback."
  type        = number
  default     = 1
}

variable "unhealthy_host_threshold" {
  description = "Unhealthy hosts across both target groups that trigger the unhealthy alarm."
  type        = number
  default     = 1
}

variable "log_retention_days" {
  description = "CloudWatch log retention. Defaults to one year, which satisfies common audit requirements."
  type        = number
  default     = 365
}

variable "alarm_email_endpoints" {
  description = "Email addresses subscribed to the alarm topic. Each recipient must confirm the subscription."
  type        = list(string)
  default     = []
}
