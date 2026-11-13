resource "aws_security_group" "alb" {
  name        = "${var.name}-alb"
  description = "Public ALB: HTTPS in, redirect in, egress to tasks only"
  vpc_id      = var.vpc_id
}

resource "aws_vpc_security_group_ingress_rule" "https" {
  security_group_id = aws_security_group.alb.id
  description       = "HTTPS from the internet"
  cidr_ipv4         = "0.0.0.0/0"
  from_port         = 443
  to_port           = 443
  ip_protocol       = "tcp"
}

resource "aws_vpc_security_group_ingress_rule" "http_redirect" {
  # checkov:skip=CKV_AWS_260: Port 80 exists only to answer with a 301 redirect to HTTPS. The listener never forwards plaintext traffic.
  security_group_id = aws_security_group.alb.id
  description       = "HTTP from the internet, answered only with a redirect to HTTPS"
  cidr_ipv4         = "0.0.0.0/0"
  from_port         = 80
  to_port           = 80
  ip_protocol       = "tcp"
}

resource "aws_vpc_security_group_ingress_rule" "test_listener" {
  for_each = toset(var.test_listener_allowed_cidrs)

  security_group_id = aws_security_group.alb.id
  description       = "CodeDeploy test listener, restricted to ${each.value}"
  cidr_ipv4         = each.value
  from_port         = var.test_listener_port
  to_port           = var.test_listener_port
  ip_protocol       = "tcp"
}

resource "aws_vpc_security_group_egress_rule" "tasks" {
  security_group_id = aws_security_group.alb.id
  description       = "Forward to Fargate tasks on the container port inside the VPC"
  cidr_ipv4         = var.vpc_cidr
  from_port         = var.target_port
  to_port           = var.target_port
  ip_protocol       = "tcp"
}

resource "aws_lb" "this" {
  name               = var.name
  load_balancer_type = "application"
  internal           = false
  subnets            = var.public_subnet_ids
  security_groups    = [aws_security_group.alb.id]

  enable_deletion_protection = var.enable_deletion_protection
  drop_invalid_header_fields = true
  desync_mitigation_mode     = "strictest"
  idle_timeout               = 60
  enable_http2               = true

  access_logs {
    bucket  = aws_s3_bucket.access_logs.id
    prefix  = var.name
    enabled = true
  }

  depends_on = [aws_s3_bucket_policy.access_logs]
}

# Everything on port 80 is redirected. No plaintext traffic is forwarded.
resource "aws_lb_listener" "http_redirect" {
  load_balancer_arn = aws_lb.this.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type = "redirect"

    redirect {
      port        = "443"
      protocol    = "HTTPS"
      status_code = "HTTP_301"
    }
  }
}
