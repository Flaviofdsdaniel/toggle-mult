module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 21.25"

  name               = "ToggleMaster"
  kubernetes_version = "1.36"

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets

  # API Kubernetes
  endpoint_private_access = true
  endpoint_public_access  = true

  # Permite que o usuario que executou o Terraform administre o cluster inicialmente
  enable_cluster_creator_admin_permissions = true

  # Add-ons fundamentais
  addons = {
    coredns = {}

    kube-proxy = {}

    vpc-cni = {
      before_compute = true
    }

    eks-pod-identity-agent = {}
  }

  # Managed Node Group
  eks_managed_node_groups = {

    main = {
      name = "togglemaster-nodes"

      kubernetes_version = "1.36"

      ami_type       = "AL2023_x86_64_STANDARD"
      instance_types = ["t3.medium"]

      capacity_type = "ON_DEMAND"

      min_size     = 1
      max_size     = 4
      desired_size = 2

      vpc_security_group_ids = [
        module.security_group.id
      ]

      tags = {
        Name = "togglemaster-nodes"
      }
    }
  }


}