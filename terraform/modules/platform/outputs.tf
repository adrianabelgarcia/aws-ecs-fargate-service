output "cluster_name" {
  description = "ECS cluster name."
  value       = aws_ecs_cluster.this.name
}

output "cluster_arn" {
  description = "ECS cluster ARN."
  value       = aws_ecs_cluster.this.arn
}

output "repository_url" {
  description = "ECR repository URL, without a tag."
  value       = aws_ecr_repository.app.repository_url
}

output "repository_arn" {
  description = "ECR repository ARN."
  value       = aws_ecr_repository.app.arn
}

output "kms_key_arn" {
  description = "ARN of the platform CMK."
  value       = aws_kms_key.platform.arn
}
