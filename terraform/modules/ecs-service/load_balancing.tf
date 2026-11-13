# Blue and green target groups. CodeDeploy registers the new tasks in the
# idle group, shifts the listeners, then drains the old group.
resource "aws_lb_target_group" "blue" {
  name = local.target_group_names.blue
  port = var.container_port
  # checkov:skip=CKV_AWS_378: TLS terminates at the ALB. ALB to task traffic stays inside private subnets behind the task security group.
  protocol             = "HTTP"
  target_type          = "ip"
  vpc_id               = var.vpc_id
  deregistration_delay = var.deregistration_delay_seconds

  health_check {
    enabled             = true
    path                = var.health_check_path
    matcher             = "200"
    interval            = 15
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 3
  }
}

resource "aws_lb_target_group" "green" {
  name = local.target_group_names.green
  port = var.container_port
  # checkov:skip=CKV_AWS_378: TLS terminates at the ALB. ALB to task traffic stays inside private subnets behind the task security group.
  protocol             = "HTTP"
  target_type          = "ip"
  vpc_id               = var.vpc_id
  deregistration_delay = var.deregistration_delay_seconds

  health_check {
    enabled             = true
    path                = var.health_check_path
    matcher             = "200"
    interval            = 15
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 3
  }
}

# Production listener. CodeDeploy rewrites its default action during a
# deployment, so Terraform must not revert it (see lifecycle below).
resource "aws_lb_listener" "https" {
  load_balancer_arn = var.alb_arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = var.ssl_policy
  certificate_arn   = var.certificate_arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.blue.arn
  }

  lifecycle {
    ignore_changes = [default_action]
  }
}

# Test listener. CodeDeploy points it at the green group while a deployment
# is in progress, so release checks can run before traffic moves.
resource "aws_lb_listener" "test" {
  load_balancer_arn = var.alb_arn
  port              = var.test_listener_port
  protocol          = "HTTPS"
  ssl_policy        = var.ssl_policy
  certificate_arn   = var.certificate_arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.blue.arn
  }

  lifecycle {
    ignore_changes = [default_action]
  }
}
