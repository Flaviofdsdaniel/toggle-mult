terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.61"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "3.2.0"
    }
  }
}

provider "aws" {
  region = "us-east-1"

}