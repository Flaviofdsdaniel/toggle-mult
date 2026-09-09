module "redis" {
  source = "terraform-aws-modules/elasticache/aws"

  replication_group_id = "togglemaster-redis"

  engine         = "redis"
  engine_version = "7.1"

  node_type = "cache.t4g.micro"
  port      = 6379

  # Laboratorio: somente 1 node
  num_cache_clusters = 1

  automatic_failover_enabled = false
  multi_az_enabled           = false

  # Criptografia em repouso
  at_rest_encryption_enabled = true

  # Deixando desabilitado para manter conexao Redis simples
  # sem TLS no ambiente de laboratorio
  transit_encryption_enabled = false

  # Usa as subnets privadas da VPC
  subnet_ids = module.vpc.private_subnets

  # Nao cria outro Security Group
  create_security_group = false

  # Usa o SG que voce ja criou
  security_group_ids = [
    module.security_group.id
  ]

  apply_immediately = true

  tags = {
    Name        = "togglemaster-redis"
    Environment = "ToggleMaster"
  }
}