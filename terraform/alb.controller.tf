resource "helm_release" "aws_load_balancer_controller" {
  name = "aws-load-balancer-controller"

  repository = "https://aws.github.io/eks-charts"
  chart      = "aws-load-balancer-controller"
  version    = "1.14.0"

  namespace = "kube-system"

  set = [
    {
      name  = "clusterName"
      value = module.eks.cluster_name
    },
    {
      name  = "serviceAccount.create"
      value = "true"
    },
    {
      name  = "serviceAccount.name"
      value = "aws-load-balancer-controller"
    },
    {
      name  = "region"
      value = "us-east-1"
    },
    {
      name  = "vpcId"
      value = module.vpc.vpc_id
    }
  ]

  wait    = true
  timeout = 600

  depends_on = [
    aws_eks_pod_identity_association.load_balancer_controller
  ]
}