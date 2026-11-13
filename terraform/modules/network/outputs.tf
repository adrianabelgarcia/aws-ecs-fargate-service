output "vpc_id" {
  description = "ID of the VPC."
  value       = module.vpc.vpc_id
}

output "vpc_cidr" {
  description = "Primary IPv4 CIDR of the VPC."
  value       = module.vpc.vpc_cidr_block
}

output "public_subnet_ids" {
  description = "Public subnets, used by the internet-facing ALB."
  value       = module.vpc.public_subnets
}

output "private_subnet_ids" {
  description = "Private subnets, used by Fargate tasks and interface endpoints."
  value       = module.vpc.private_subnets
}

output "endpoints_security_group_id" {
  description = "Security group attached to interface endpoints. Workloads allow egress to it on 443."
  value       = aws_security_group.endpoints.id
}

output "s3_prefix_list_id" {
  description = "Managed prefix list for the S3 gateway endpoint, used for egress rules from tasks."
  value       = data.aws_prefix_list.s3.id
}
