# ECS cluster with enhanced Container Insights for per-task CPU and memory
# metrics, which the autoscaling and dashboards rely on.
resource "aws_ecs_cluster" "this" {
  name = var.name

  setting {
    name  = "containerInsights"
    value = "enhanced"
  }

  configuration {
    execute_command_configuration {
      # Exec sessions are audited through CloudWatch Logs and encrypted with the platform key.
      kms_key_id = aws_kms_key.platform.arn
      logging    = "OVERRIDE"

      log_configuration {
        cloud_watch_encryption_enabled = true
        cloud_watch_log_group_name     = "/aws/ecs/exec/${var.name}"
      }
    }
  }
}

resource "aws_cloudwatch_log_group" "exec" {
  name              = "/aws/ecs/exec/${var.name}"
  retention_in_days = 365
  kms_key_id        = aws_kms_key.platform.arn
}

resource "aws_ecs_cluster_capacity_providers" "this" {
  cluster_name       = aws_ecs_cluster.this.name
  capacity_providers = ["FARGATE", "FARGATE_SPOT"]

  default_capacity_provider_strategy {
    capacity_provider = "FARGATE"
    base              = var.fargate_base
    weight            = var.fargate_weight
  }

  default_capacity_provider_strategy {
    capacity_provider = "FARGATE_SPOT"
    weight            = var.fargate_spot_weight
  }
}
