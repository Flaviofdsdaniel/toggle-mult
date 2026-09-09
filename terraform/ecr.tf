locals {
  ecr_repositories = toset([
    "auth-service",
    "flag-service",
    "evaluation-service",
    "analytics-service",
    "targeting-service"
  ])
}

module "ecr" {
  for_each = local.ecr_repositories

  source  = "terraform-aws-modules/ecr/aws"
  version = "3.2.0"

  repository_name = each.value

  repository_image_tag_mutability = "MUTABLE"

  repository_image_scan_on_push = true

  # permite terraform destroy mesmo se houver imagens
  repository_force_delete = true

  create_lifecycle_policy = true

  repository_lifecycle_policy = jsonencode({
    rules = [
      {
        rulePriority = 1

        description = "Remove imagens sem tag antigas"

        selection = {
          tagStatus   = "untagged"
          countType   = "imageCountMoreThan"
          countNumber = 5
        }

        action = {
          type = "expire"
        }
      }
    ]
  })

  tags = merge(
    {
      Name      = each.value
      Project   = "ToggleMaster"
      ManagedBy = "Terraform"
    },
    {
      Service = each.value
    }
  )
}