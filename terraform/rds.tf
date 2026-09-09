module "rds" {
  source = "terraform-aws-modules/rds/aws"

  identifier = "togglemaster-postgres"

  # PostgreSQL
  engine         = "postgres"
  engine_version = "16"
  family         = "postgres16"

  # Instância
  instance_class    = "db.t4g.micro"
  allocated_storage = 20
  storage_type      = "gp3"
  storage_encrypted = true

  # Banco
  db_name  = "postgres"
  username = "postgres"
  port     = 5432

  # Senha gerenciada automaticamente pelo Secrets Manager
  manage_master_user_password = true

  # Rede
  create_db_subnet_group = true

  subnet_ids = module.vpc.private_subnets

  vpc_security_group_ids = [
    module.security_group.id
  ]

  # Banco privado
  publicly_accessible = false

  # Laboratório
  multi_az            = false
  deletion_protection = false
  skip_final_snapshot = true

  # Backup
  backup_retention_period = 1

  # PostgreSQL não usa DB Option Group
  create_db_option_group = false

  tags = {
    Name        = "togglemaster-postgres"
    Environment = "ToggleMaster"
  }
}