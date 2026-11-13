resource "aws_appautoscaling_target" "this" {
  service_namespace  = "ecs"
  resource_id        = "service/${var.cluster_name}/${aws_ecs_service.this.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  min_capacity       = var.min_capacity
  max_capacity       = var.max_capacity
}

resource "aws_appautoscaling_policy" "cpu" {
  name               = "${var.name}-cpu-target"
  policy_type        = "TargetTrackingScaling"
  service_namespace  = aws_appautoscaling_target.this.service_namespace
  resource_id        = aws_appautoscaling_target.this.resource_id
  scalable_dimension = aws_appautoscaling_target.this.scalable_dimension

  target_tracking_scaling_policy_configuration {
    target_value       = var.cpu_target_percent
    scale_in_cooldown  = 300
    scale_out_cooldown = 60

    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageCPUUtilization"
    }
  }
}

# The predefined ALBRequestCountPerTarget metric is bound to one target group.
# With blue/green, the active group changes on every release, so the policy
# would silently track the wrong group. This customised metric sums the ALB
# request count and divides by the healthy hosts in both groups instead.
resource "aws_appautoscaling_policy" "requests" {
  name               = "${var.name}-requests-per-target"
  policy_type        = "TargetTrackingScaling"
  service_namespace  = aws_appautoscaling_target.this.service_namespace
  resource_id        = aws_appautoscaling_target.this.resource_id
  scalable_dimension = aws_appautoscaling_target.this.scalable_dimension

  target_tracking_scaling_policy_configuration {
    target_value       = var.requests_per_target
    scale_in_cooldown  = 300
    scale_out_cooldown = 60

    customized_metric_specification {
      metrics {
        id          = "requests"
        label       = "ALB request count"
        return_data = false

        metric_stat {
          stat = "Sum"

          metric {
            namespace   = "AWS/ApplicationELB"
            metric_name = "RequestCount"

            dimensions {
              name  = "LoadBalancer"
              value = var.alb_arn_suffix
            }
          }
        }
      }

      metrics {
        id          = "healthyblue"
        label       = "Healthy hosts (blue)"
        return_data = false

        metric_stat {
          stat = "Average"

          metric {
            namespace   = "AWS/ApplicationELB"
            metric_name = "HealthyHostCount"

            dimensions {
              name  = "LoadBalancer"
              value = var.alb_arn_suffix
            }
            dimensions {
              name  = "TargetGroup"
              value = aws_lb_target_group.blue.arn_suffix
            }
          }
        }
      }

      metrics {
        id          = "healthygreen"
        label       = "Healthy hosts (green)"
        return_data = false

        metric_stat {
          stat = "Average"

          metric {
            namespace   = "AWS/ApplicationELB"
            metric_name = "HealthyHostCount"

            dimensions {
              name  = "LoadBalancer"
              value = var.alb_arn_suffix
            }
            dimensions {
              name  = "TargetGroup"
              value = aws_lb_target_group.green.arn_suffix
            }
          }
        }
      }

      metrics {
        id          = "perhost"
        label       = "Requests per healthy host"
        expression  = "IF(healthyblue + healthygreen > 0, requests / (healthyblue + healthygreen), 0)"
        return_data = true
      }
    }
  }
}
