locals {
  container_name = var.name
  log_group_name = "/ecs/${var.name}"

  secret_arns    = [for arn in values(var.secrets) : arn if can(regex("^arn:aws[a-zA-Z-]*:secretsmanager:", arn))]
  parameter_arns = [for arn in values(var.secrets) : arn if can(regex("^arn:aws[a-zA-Z-]*:ssm:", arn))]

  # Burn-rate arithmetic. An SLO of 99.9% allows 0.1% of requests to fail.
  # A burn rate of N consumes budget N times faster than sustainable, so the
  # alarm threshold is N * (1 - SLO) expressed as a percentage.
  error_budget_percent = (1 - var.availability_slo) * 100
  fast_threshold       = var.fast_burn_rate * local.error_budget_percent
  slow_threshold       = var.slow_burn_rate * local.error_budget_percent

  # Blue and green target groups share the service. The ALB-level metrics
  # below aggregate both, which keeps alarms valid across a traffic shift.
  target_group_names = {
    blue  = "${var.name}-blue"
    green = "${var.name}-green"
  }

  alarm_name_prefix = "${var.name}-${var.environment}"
}
