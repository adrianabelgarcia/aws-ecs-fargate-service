provider "aws" {
  region = var.aws_region

  # Refuses to run against any other account. Protects against applying prod
  # code with staging credentials, or the reverse.
  allowed_account_ids = [var.expected_account_id]

  default_tags {
    tags = {
      Project     = "aws-ecs-fargate-service"
      Environment = var.environment
      ManagedBy   = "terraform"
      Owner       = "platform"
      CostCenter  = "portfolio-demo"
    }
  }
}
