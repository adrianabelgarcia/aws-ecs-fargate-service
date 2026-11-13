output "ecr_repository_url" {
  description = "ECR repository the pipeline pushes to."
  value       = module.platform.repository_url
}

output "ecs_cluster_name" {
  description = "ECS cluster name."
  value       = module.platform.cluster_name
}

output "ecs_service_name" {
  description = "ECS service name."
  value       = module.service.service_name
}

output "container_name" {
  description = "Container name in the task definition."
  value       = module.service.container_name
}

output "container_port" {
  description = "Container port."
  value       = module.service.container_port
}

output "codedeploy_application_name" {
  description = "CodeDeploy application name."
  value       = module.service.codedeploy_application_name
}

output "codedeploy_deployment_group_name" {
  description = "CodeDeploy deployment group name."
  value       = module.service.codedeploy_deployment_group_name
}

output "service_url" {
  description = "Public endpoint for the service."
  value       = "https://${var.domain_name}"
}

output "alb_dns_name" {
  description = "ALB DNS name."
  value       = module.edge.alb_dns_name
}

output "log_group_name" {
  description = "Application log group."
  value       = module.service.log_group_name
}

output "dashboard_name" {
  description = "CloudWatch dashboard name."
  value       = module.service.dashboard_name
}

output "alarm_topic_arn" {
  description = "SNS topic for alarm notifications."
  value       = module.service.alarm_topic_arn
}

output "alb_arn_suffix" {
  description = "ALB ARN suffix, the LoadBalancer dimension for CloudWatch queries in the runbooks."
  value       = module.edge.alb_arn_suffix
}

output "target_group_arns" {
  description = "Blue and green target group ARNs."
  value       = module.service.target_group_arns
}

output "vpc_id" {
  description = "VPC ID."
  value       = module.network.vpc_id
}

output "private_subnet_ids" {
  description = "Private subnets used by Fargate tasks."
  value       = module.network.private_subnet_ids
}

output "secret_name" {
  description = "Secrets Manager name of the API token. The value is set out-of-band."
  value       = aws_secretsmanager_secret.api_token.name
}
