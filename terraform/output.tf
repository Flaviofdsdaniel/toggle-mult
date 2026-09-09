#VPC

output "vpc_id" {
  value = module.vpc.vpc_id
}

output "private_subnets" {
  value = module.vpc.private_subnets
}

output "public_subnets" {
  value = module.vpc.public_subnets
}

#Security Group

output "security_group_id" {
  value = module.security_group.id
}

#RDS

output "rds_endpoint" {
  value = module.rds.db_instance_endpoint
}

output "rds_port" {
  value = module.rds.db_instance_port
}

output "rds_username" {
  value     = module.rds.db_instance_username
  sensitive = true
}


output "rds_db_name" {
  value = module.rds.db_instance_name
}

output "rds_master_password_secret_arn" {
  value = module.rds.db_instance_master_user_secret_arn
}

#Redis

output "redis_endpoint" {
  value = module.redis.replication_group_primary_endpoint_address
}

output "redis_port" {
  value = module.redis.replication_group_port
}

output "redis_replication_group_id" {
  value = module.redis.replication_group_id
}

#DynamoDB

output "dynamodb_table_id" {
  value = module.dynamodb_table.dynamodb_table_id
}

output "dynamodb_table_arn" {
  value = module.dynamodb_table.dynamodb_table_arn
}

#SQS Fila

output "sqs_queue_name" {
  value = module.sqs.queue_name
}

output "sqs_queue_arn" {

  value = module.sqs.queue_arn
}

output "sqs_queue_url" {
  value = module.sqs.queue_url
}

#EKS Cluster

output "eks_cluster_status" {
  value = module.eks.cluster_status
}

output "eks_cluster_endpoint" {
  value = module.eks.cluster_endpoint
}

#ECR

output "ecr_repository_urls" {
  description = "URLs dos repositorios ECR"

  value = {
    for name, repository in module.ecr :
    name => repository.repository_url
  }
}

output "ecr_repository_arns" {
  description = "ARNs dos repositorios ECR"

  value = {
    for name, repository in module.ecr :
    name => repository.repository_arn
  }
}

output "ecr_repository_names" {
  description = "Nomes dos repositorios ECR"

  value = {
    for name, repository in module.ecr :
    name => repository.repository_name
  }
}