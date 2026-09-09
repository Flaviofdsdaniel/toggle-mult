module "security_group" {
  source = "terraform-aws-modules/security-group/aws"

  name        = "sgtogglemaster"
  description = "Security group for ToggleMaster"
  vpc_id      = module.vpc.vpc_id

  ingress_rules = {
    https = {
      from_port   = 443
      ip_protocol = "tcp"
      cidr_ipv4   = "10.0.0.0/16"
      description = "HTTPS from internal"
    }

    #Realiza a liberação de todo o tráfego entre os membros do mesmo SG
    self-all = {
      ip_protocol                  = "-1"
      referenced_security_group_id = "self"
      description                  = "All traffic from members of this SG"
    }
  }

  egress_rules = {
    all = {
      ip_protocol = "-1"
      cidr_ipv4   = "0.0.0.0/0"
    }
  }

  tags = {
    Environment = "ToggleMaster"
  }
}