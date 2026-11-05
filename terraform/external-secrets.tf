resource "helm_release" "external_secrets" {
  name       = "external-secrets"
  repository = "https://charts.external-secrets.io"
  chart      = "external-secrets"
  version    = "2.14.0"

  namespace        = "external-secrets"
  create_namespace = true

  wait            = true
  timeout         = 600
  atomic          = true
  cleanup_on_fail = true

  values = [
    yamlencode({
      serviceAccount = {
        create = true
        name   = "external-secrets"
      }
    })
  ]

  depends_on = [
    module.eks,
    helm_release.aws_load_balancer_controller
  ]
}

locals {
  application_secret_names = toset([
    "auth-service",
    "flag-service",
    "targeting-service",
    "evaluation-service"
  ])
}

resource "aws_secretsmanager_secret" "application" {
  for_each = local.application_secret_names

  name = "toggle-mult/${each.value}"
}