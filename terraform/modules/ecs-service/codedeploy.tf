resource "aws_codedeploy_app" "this" {
  name             = var.name
  compute_platform = "ECS"
}

# Blue/green release with automatic rollback. A deployment stops and rolls back
# when any alarm below enters ALARM. The alarms are the same ones that page
# on-call, so a release that burns error budget is reverted automatically.
resource "aws_codedeploy_deployment_group" "this" {
  app_name               = aws_codedeploy_app.this.name
  deployment_group_name  = "${var.name}-${var.environment}"
  service_role_arn       = aws_iam_role.codedeploy.arn
  deployment_config_name = var.deployment_config_name

  deployment_style {
    deployment_option = "WITH_TRAFFIC_CONTROL"
    deployment_type   = "BLUE_GREEN"
  }

  blue_green_deployment_config {
    deployment_ready_option {
      action_on_timeout = "CONTINUE_DEPLOYMENT"
    }

    terminate_blue_instances_on_deployment_success {
      action                           = "TERMINATE"
      termination_wait_time_in_minutes = var.termination_wait_minutes
    }
  }

  ecs_service {
    cluster_name = var.cluster_name
    service_name = aws_ecs_service.this.name
  }

  load_balancer_info {
    target_group_pair_info {
      prod_traffic_route {
        listener_arns = [aws_lb_listener.https.arn]
      }

      test_traffic_route {
        listener_arns = [aws_lb_listener.test.arn]
      }

      target_group {
        name = aws_lb_target_group.blue.name
      }

      target_group {
        name = aws_lb_target_group.green.name
      }
    }
  }

  auto_rollback_configuration {
    enabled = true
    events  = ["DEPLOYMENT_FAILURE", "DEPLOYMENT_STOP_ON_ALARM"]
  }

  alarm_configuration {
    enabled                   = true
    ignore_poll_alarm_failure = false
    alarms = [
      aws_cloudwatch_metric_alarm.error_ratio_fast.alarm_name,
      aws_cloudwatch_metric_alarm.latency_p99.alarm_name,
      aws_cloudwatch_metric_alarm.unhealthy_hosts.alarm_name,
    ]
  }
}
