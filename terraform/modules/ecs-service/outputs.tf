output "service_name" {
  description = "ECS service name."
  value       = aws_ecs_service.this.name
}

output "container_name" {
  description = "Container name inside the task definition. Used by the deploy pipeline."
  value       = local.container_name
}

output "container_port" {
  description = "Container port. Used by the appspec LoadBalancerInfo block."
  value       = var.container_port
}

output "task_definition_family" {
  description = "Task definition family. The pipeline registers new revisions under this family."
  value       = aws_ecs_task_definition.this.family
}

output "codedeploy_application_name" {
  description = "CodeDeploy application name."
  value       = aws_codedeploy_app.this.name
}

output "codedeploy_deployment_group_name" {
  description = "CodeDeploy deployment group name."
  value       = aws_codedeploy_deployment_group.this.deployment_group_name
}

output "task_security_group_id" {
  description = "Security group of the Fargate tasks."
  value       = aws_security_group.tasks.id
}

output "https_listener_arn" {
  description = "Production HTTPS listener ARN."
  value       = aws_lb_listener.https.arn
}

output "log_group_name" {
  description = "CloudWatch log group for the container."
  value       = aws_cloudwatch_log_group.this.name
}

output "alarm_topic_arn" {
  description = "SNS topic that receives alarm notifications."
  value       = aws_sns_topic.alerts.arn
}

output "alarm_names" {
  description = "Names of the CloudWatch alarms created for the service."
  value = [
    aws_cloudwatch_metric_alarm.error_ratio_fast.alarm_name,
    aws_cloudwatch_metric_alarm.error_ratio_slow.alarm_name,
    aws_cloudwatch_metric_alarm.latency_p99.alarm_name,
    aws_cloudwatch_metric_alarm.unhealthy_hosts.alarm_name,
  ]
}

output "dashboard_name" {
  description = "CloudWatch dashboard name."
  value       = aws_cloudwatch_dashboard.this.dashboard_name
}

output "target_group_arns" {
  description = "Blue and green target group ARNs. CodeDeploy swaps which one the listener forwards to."
  value = {
    blue  = aws_lb_target_group.blue.arn
    green = aws_lb_target_group.green.arn
  }
}
