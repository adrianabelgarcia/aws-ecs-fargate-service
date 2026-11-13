locals {
  dashboard_region = data.aws_region.current.region
}

# One dashboard per service and environment. Layout follows the SLO order:
# traffic and errors first, then latency, then capacity and saturation.
resource "aws_cloudwatch_dashboard" "this" {
  dashboard_name = "${local.alarm_name_prefix}-service"

  dashboard_body = jsonencode({
    widgets = [
      {
        type = "metric", x = 0, y = 0, width = 12, height = 6
        properties = {
          title  = "Requests and 5xx (ALB)"
          region = local.dashboard_region
          stat   = "Sum"
          period = 60
          view   = "timeSeries"
          metrics = [
            ["AWS/ApplicationELB", "RequestCount", "LoadBalancer", var.alb_arn_suffix, { label = "requests" }],
            ["AWS/ApplicationELB", "HTTPCode_Target_5XX_Count", "LoadBalancer", var.alb_arn_suffix, { label = "target 5xx", color = "#d62728" }],
            ["AWS/ApplicationELB", "HTTPCode_ELB_5XX_Count", "LoadBalancer", var.alb_arn_suffix, { label = "ELB 5xx", color = "#ff7f0e" }],
          ]
        }
      },
      {
        type = "metric", x = 12, y = 0, width = 12, height = 6
        properties = {
          title  = "5xx percent (SLO ${format("%.2f", local.error_budget_percent)}% budget)"
          region = local.dashboard_region
          view   = "timeSeries"
          period = 60
          metrics = [
            ["AWS/ApplicationELB", "RequestCount", "LoadBalancer", var.alb_arn_suffix, { id = "req", stat = "Sum", visible = false }],
            ["AWS/ApplicationELB", "HTTPCode_Target_5XX_Count", "LoadBalancer", var.alb_arn_suffix, { id = "t5", stat = "Sum", visible = false }],
            ["AWS/ApplicationELB", "HTTPCode_ELB_5XX_Count", "LoadBalancer", var.alb_arn_suffix, { id = "e5", stat = "Sum", visible = false }],
            [{ expression = "IF(req < 20, 0, 100 * (t5 + e5) / req)", label = "5xx %", id = "ratio" }],
          ]
          annotations = {
            horizontal = [
              { label = "fast burn", value = local.fast_threshold, color = "#d62728" },
              { label = "slow burn", value = local.slow_threshold, color = "#ff7f0e" },
            ]
          }
        }
      },
      {
        type = "metric", x = 0, y = 6, width = 12, height = 6
        properties = {
          title  = "Target response time (p50, p99)"
          region = local.dashboard_region
          view   = "timeSeries"
          period = 60
          metrics = [
            ["AWS/ApplicationELB", "TargetResponseTime", "LoadBalancer", var.alb_arn_suffix, { stat = "p50", label = "p50" }],
            ["AWS/ApplicationELB", "TargetResponseTime", "LoadBalancer", var.alb_arn_suffix, { stat = "p99", label = "p99", color = "#d62728" }],
          ]
          annotations = {
            horizontal = [{ label = "p99 target", value = var.latency_p99_threshold_seconds, color = "#7f7f7f" }]
          }
        }
      },
      {
        type = "metric", x = 12, y = 6, width = 12, height = 6
        properties = {
          title  = "Healthy and unhealthy targets"
          region = local.dashboard_region
          view   = "timeSeries"
          stat   = "Maximum"
          period = 60
          metrics = [
            ["AWS/ApplicationELB", "HealthyHostCount", "LoadBalancer", var.alb_arn_suffix, "TargetGroup", aws_lb_target_group.blue.arn_suffix, { label = "healthy blue" }],
            ["AWS/ApplicationELB", "HealthyHostCount", "LoadBalancer", var.alb_arn_suffix, "TargetGroup", aws_lb_target_group.green.arn_suffix, { label = "healthy green" }],
            ["AWS/ApplicationELB", "UnHealthyHostCount", "LoadBalancer", var.alb_arn_suffix, "TargetGroup", aws_lb_target_group.blue.arn_suffix, { label = "unhealthy blue", color = "#d62728" }],
            ["AWS/ApplicationELB", "UnHealthyHostCount", "LoadBalancer", var.alb_arn_suffix, "TargetGroup", aws_lb_target_group.green.arn_suffix, { label = "unhealthy green", color = "#ff7f0e" }],
          ]
        }
      },
      {
        type = "metric", x = 0, y = 12, width = 12, height = 6
        properties = {
          title  = "Task CPU and memory (Container Insights)"
          region = local.dashboard_region
          view   = "timeSeries"
          stat   = "Average"
          period = 60
          metrics = [
            ["ECS/ContainerInsights", "CpuUtilized", "ClusterName", var.cluster_name, "ServiceName", var.name, { label = "CPU units used" }],
            ["ECS/ContainerInsights", "MemoryUtilized", "ClusterName", var.cluster_name, "ServiceName", var.name, { label = "memory MiB used", yAxis = "right" }],
          ]
        }
      },
      {
        type = "metric", x = 12, y = 12, width = 12, height = 6
        properties = {
          title  = "Running tasks"
          region = local.dashboard_region
          view   = "timeSeries"
          stat   = "Average"
          period = 60
          metrics = [
            ["ECS/ContainerInsights", "RunningTaskCount", "ClusterName", var.cluster_name, "ServiceName", var.name],
          ]
        }
      },
      {
        type = "alarm", x = 0, y = 18, width = 24, height = 4
        properties = {
          title = "Service alarms"
          alarms = [
            aws_cloudwatch_metric_alarm.error_ratio_fast.arn,
            aws_cloudwatch_metric_alarm.error_ratio_slow.arn,
            aws_cloudwatch_metric_alarm.latency_p99.arn,
            aws_cloudwatch_metric_alarm.unhealthy_hosts.arn,
          ]
        }
      },
    ]
  })
}
