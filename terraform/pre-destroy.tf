resource "terraform_data" "gitops_pre_destroy" {
  input = {
    cluster_name = module.eks.cluster_name
    region       = "us-east-1"
    vpc_id       = module.vpc.vpc_id
  }

  # Run the cleanup while Argo CD, the AWS Load Balancer Controller,
  # External Secrets, KEDA, and EKS still exist.
  depends_on = [
    module.dynamodb_table,
    module.ecr,
    module.eks,
    module.rds,
    module.redis,
    module.sqs,
    module.vpc,
    module.security_group,
    helm_release.argocd,
    helm_release.aws_load_balancer_controller,
    helm_release.external_secrets,
    helm_release.keda,
    aws_eks_pod_identity_association.analytics,
    aws_eks_pod_identity_association.evaluation,
    aws_eks_pod_identity_association.external_secrets,
    aws_eks_pod_identity_association.keda,
    aws_eks_pod_identity_association.load_balancer_controller,
    aws_iam_role_policy_attachment.external_secrets,
    aws_iam_role_policy_attachment.keda,
    aws_iam_role_policy_attachment.load_balancer_controller,
    aws_secretsmanager_secret.application
  ]

  provisioner "local-exec" {
    when        = destroy
    interpreter = ["PowerShell", "-NoProfile", "-NonInteractive", "-Command"]
    command     = "& '${path.module}/pre-destroy.ps1' -ClusterName '${self.input.cluster_name}' -VpcId '${self.input.vpc_id}' -Region '${self.input.region}'"
  }
}
