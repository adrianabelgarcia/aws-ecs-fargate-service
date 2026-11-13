resource "aws_sns_topic" "alerts" {
  name              = "${local.alarm_name_prefix}-alerts"
  kms_master_key_id = var.kms_key_arn
}

resource "aws_sns_topic_subscription" "email" {
  for_each = toset(var.alarm_email_endpoints)

  topic_arn = aws_sns_topic.alerts.arn
  protocol  = "email"
  endpoint  = each.value
}

# Metric queries shared by the alarms. ALB metrics are read at the load
# balancer level so the numbers include both target groups.
locals {
  alb_dimension = {
    name  = "LoadBalancer"
    value = var.alb_arn_suffix
  }
}

# Fast burn (page). Error ratio over 1-minute periods, threshold =
# fast_burn_rate * error budget. Two of three minutes must breach, so one bad
# datapoint does not page, but a sustained burn does. CodeDeploy uses this
# alarm to stop and roll back a release.
resource "aws_cloudwatch_metric_alarm" "error_ratio_fast" {
  alarm_name          = "${local.alarm_name_prefix}-5xx-fast-burn"
  alarm_description   = "5xx ratio above ${format("%.2f", local.fast_threshold)}% (${var.fast_burn_rate}x burn of a ${var.availability_slo} SLO). Pages on-call and stops any CodeDeploy rollout."
  comparison_operator = "GreaterThanThreshold"
  threshold           = local.fast_threshold
  evaluation_periods  = 3
  datapoints_to_alarm = 2
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alerts.arn]

  metric_query {
    id          = "requests"
    return_data = false
    metric {
      namespace   = "AWS/ApplicationELB"
      metric_name = "RequestCount"
      period      = 60
      stat        = "Sum"
      dimensions  = { (local.alb_dimension.name) = local.alb_dimension.value }
    }
  }

  metric_query {
    id          = "target5xx"
    return_data = false
    metric {
      namespace   = "AWS/ApplicationELB"
      metric_name = "HTTPCode_Target_5XX_Count"
      period      = 60
      stat        = "Sum"
      dimensions  = { (local.alb_dimension.name) = local.alb_dimension.value }
    }
  }

  metric_query {
    id          = "elb5xx"
    return_data = false
    metric {
      namespace   = "AWS/ApplicationELB"
      metric_name = "HTTPCode_ELB_5XX_Count"
      period      = 60
      stat        = "Sum"
      dimensions  = { (local.alb_dimension.name) = local.alb_dimension.value }
    }
  }

  metric_query {
    id          = "ratio"
    label       = "5xx percent"
    expression  = "IF(requests < 20, 0, 100 * (target5xx + elb5xx) / requests)"
    return_data = true
  }
}

# Slow burn (ticket). Same ratio, lower multiplier, sustained for an hour
# (12 consecutive 5-minute datapoints). It catches slow leaks that never trip
# the fast alarm but still consume the monthly budget.
resource "aws_cloudwatch_metric_alarm" "error_ratio_slow" {
  alarm_name          = "${local.alarm_name_prefix}-5xx-slow-burn"
  alarm_description   = "5xx ratio above ${format("%.2f", local.slow_threshold)}% for one hour (${var.slow_burn_rate}x burn of a ${var.availability_slo} SLO). Opens a ticket."
  comparison_operator = "GreaterThanThreshold"
  threshold           = local.slow_threshold
  evaluation_periods  = 12
  datapoints_to_alarm = 12
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alerts.arn]

  metric_query {
    id          = "requests"
    return_data = false
    metric {
      namespace   = "AWS/ApplicationELB"
      metric_name = "RequestCount"
      period      = 300
      stat        = "Sum"
      dimensions  = { (local.alb_dimension.name) = local.alb_dimension.value }
    }
  }

  metric_query {
    id          = "target5xx"
    return_data = false
    metric {
      namespace   = "AWS/ApplicationELB"
      metric_name = "HTTPCode_Target_5XX_Count"
      period      = 300
      stat        = "Sum"
      dimensions  = { (local.alb_dimension.name) = local.alb_dimension.value }
    }
  }

  metric_query {
    id          = "elb5xx"
    return_data = false
    metric {
      namespace   = "AWS/ApplicationELB"
      metric_name = "HTTPCode_ELB_5XX_Count"
      period      = 300
      stat        = "Sum"
      dimensions  = { (local.alb_dimension.name) = local.alb_dimension.value }
    }
  }

  metric_query {
    id          = "ratio"
    label       = "5xx percent (5m)"
    expression  = "IF(requests < 20, 0, 100 * (target5xx + elb5xx) / requests)"
    return_data = true
  }
}

# Latency SLO. p99 of target response time at the ALB, which is the number
# users feel. The CodeDeploy rollback also uses this alarm.
resource "aws_cloudwatch_metric_alarm" "latency_p99" {
  alarm_name          = "${local.alarm_name_prefix}-latency-p99"
  alarm_description   = "p99 target response time above ${var.latency_p99_threshold_seconds}s"
  namespace           = "AWS/ApplicationELB"
  metric_name         = "TargetResponseTime"
  extended_statistic  = "p99"
  period              = 60
  evaluation_periods  = 3
  datapoints_to_alarm = 2
  comparison_operator = "GreaterThanThreshold"
  threshold           = var.latency_p99_threshold_seconds
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alerts.arn]

  dimensions = {
    (local.alb_dimension.name) = local.alb_dimension.value
  }
}

# Unhealthy targets across both groups. A sustained non-zero count means
# tasks are failing readiness, which is the earliest sign of a bad release.
resource "aws_cloudwatch_metric_alarm" "unhealthy_hosts" {
  alarm_name          = "${local.alarm_name_prefix}-unhealthy-hosts"
  alarm_description   = "Unhealthy targets across blue and green groups"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  threshold           = var.unhealthy_host_threshold
  evaluation_periods  = 3
  datapoints_to_alarm = 3
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alerts.arn]

  metric_query {
    id          = "unhealthyblue"
    return_data = false
    metric {
      namespace   = "AWS/ApplicationELB"
      metric_name = "UnHealthyHostCount"
      period      = 60
      stat        = "Maximum"
      dimensions = {
        (local.alb_dimension.name) = local.alb_dimension.value
        TargetGroup                = aws_lb_target_group.blue.arn_suffix
      }
    }
  }

  metric_query {
    id          = "unhealthygreen"
    return_data = false
    metric {
      namespace   = "AWS/ApplicationELB"
      metric_name = "UnHealthyHostCount"
      period      = 60
      stat        = "Maximum"
      dimensions = {
        (local.alb_dimension.name) = local.alb_dimension.value
        TargetGroup                = aws_lb_target_group.green.arn_suffix
      }
    }
  }

  metric_query {
    id          = "unhealthy"
    label       = "Unhealthy targets"
    expression  = "unhealthyblue + unhealthygreen"
    return_data = true
  }
}
