resource "aws_ecs_service" "this" {
  name            = var.name
  cluster         = var.cluster_name
  task_definition = aws_ecs_task_definition.this.arn
  desired_count   = var.desired_count

  # CODE_DEPLOY hands release control to CodeDeploy. Terraform creates the
  # service once; every later release is a CodeDeploy deployment.
  deployment_controller {
    type = "CODE_DEPLOY"
  }

  dynamic "capacity_provider_strategy" {
    for_each = var.capacity_provider_strategy
    content {
      capacity_provider = capacity_provider_strategy.value.capacity_provider
      base              = capacity_provider_strategy.value.base
      weight            = capacity_provider_strategy.value.weight
    }
  }

  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = [aws_security_group.tasks.id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.blue.arn
    container_name   = local.container_name
    container_port   = var.container_port
  }

  health_check_grace_period_seconds = var.health_check_grace_period_seconds
  enable_ecs_managed_tags           = true
  propagate_tags                    = "SERVICE"
  enable_execute_command            = false
  wait_for_steady_state             = false

  depends_on = [
    aws_lb_listener.https,
    aws_lb_listener.test,
  ]

  # Three attributes change outside Terraform on purpose. Ignoring them keeps
  # plans quiet and prevents Terraform from undoing a release.
  #   task_definition: CodeDeploy registers a new revision for each release.
  #   load_balancer:   CodeDeploy flips the service between blue and green target
  #                    groups. Reverting it would shift traffic without a deployment.
  #   desired_count:   Application Auto Scaling owns the running count.
  lifecycle {
    ignore_changes = [
      task_definition,
      load_balancer,
      desired_count,
    ]
  }
}
