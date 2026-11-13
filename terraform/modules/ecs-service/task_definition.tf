# The task definition is the contract between the image and the runtime:
#   - read-only root filesystem, nonroot UID (matches the distroless image)
#   - secrets resolved by ECS at start, never written into the definition
#   - health check runs the binary's own `healthcheck` subcommand, because the
#     distroless image has no shell or curl
#   - logs go to the KMS-encrypted log group through awslogs
resource "aws_ecs_task_definition" "this" {
  family                   = var.name
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = tostring(var.task_cpu)
  memory                   = tostring(var.task_memory)
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task.arn

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "X86_64"
  }

  container_definitions = jsonencode([
    {
      name                   = local.container_name
      image                  = var.container_image
      essential              = true
      readonlyRootFilesystem = true
      user                   = "65532:65532"
      privileged             = false
      stopTimeout            = var.stop_timeout_seconds

      portMappings = [
        {
          containerPort = var.container_port
          hostPort      = var.container_port
          protocol      = "tcp"
          name          = "http"
          appProtocol   = "http"
        },
      ]

      environment = [
        for k, v in var.environment_variables : { name = k, value = v }
      ]

      secrets = [
        for k, arn in var.secrets : { name = k, valueFrom = arn }
      ]

      healthCheck = {
        command     = ["CMD", "/usr/local/bin/api", "healthcheck"]
        interval    = 15
        timeout     = 5
        retries     = 3
        startPeriod = 10
      }

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.this.name
          "awslogs-region"        = data.aws_region.current.region
          "awslogs-stream-prefix" = "api"
          "mode"                  = "non-blocking"
          "max-buffer-size"       = "25m"
        }
      }
    },
  ])
}
