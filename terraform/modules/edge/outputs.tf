output "alb_arn" {
  description = "ALB ARN."
  value       = aws_lb.this.arn
}

output "alb_arn_suffix" {
  description = "ALB ARN suffix, used as the LoadBalancer dimension in CloudWatch."
  value       = aws_lb.this.arn_suffix
}

output "alb_dns_name" {
  description = "Public DNS name of the ALB. Point a CNAME or alias record here."
  value       = aws_lb.this.dns_name
}

output "alb_zone_id" {
  description = "Hosted zone ID of the ALB, needed for alias records."
  value       = aws_lb.this.zone_id
}

output "alb_security_group_id" {
  description = "Security group of the ALB. Task security groups allow ingress from it."
  value       = aws_security_group.alb.id
}

output "waf_web_acl_arn" {
  description = "ARN of the regional WAF web ACL."
  value       = aws_wafv2_web_acl.this.arn
}
