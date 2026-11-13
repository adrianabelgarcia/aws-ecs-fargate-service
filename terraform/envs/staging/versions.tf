terraform {
  required_version = ">= 1.10"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.73"
    }
  }

  # Partial configuration: bucket, key and region come from backend.hcl.
  #   terraform init -backend-config=backend.hcl
  backend "s3" {}
}
