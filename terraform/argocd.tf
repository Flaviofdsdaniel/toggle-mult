resource "helm_release" "argocd" {

  name = "argocd"

  repository = "https://argoproj.github.io/argo-helm"
  chart      = "argo-cd"
  version    = "10.8.4"

  namespace        = "argocd"
  create_namespace = true

  wait    = true
  timeout = 900

  atomic          = true
  cleanup_on_fail = true

  values = [
    yamlencode({

      server = {
        service = {
          type = "ClusterIP"
        }
      }

      crds = {
        install = true
        keep    = true
      }

    })
  ]

  depends_on = [
    module.eks
  ]
}