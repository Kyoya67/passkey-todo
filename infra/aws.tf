module "vpc" {
  source = "./modules/vpc"

  name       = local.project
  cidr_block = "10.0.0.0/16"
  tags       = local.common_tags
}

# module "subnet" {
#   source = "./modules/subnet"

#   name               = local.project
#   vpc_id             = module.vpc.id
#   availability_zones = ["ap-northeast-1a", "ap-northeast-1c"]
#   public_cidrs       = ["10.0.0.0/20", "10.0.16.0/20"]
#   private_cidrs      = ["10.0.128.0/20", "10.0.144.0/20"]
#   tags               = local.common_tags
# }

# module "security_groups" {
#   source = "./modules/security_groups"

#   name   = local.project
#   vpc_id = module.vpc.id
#   tags   = local.common_tags
# }

# module "ecr" {
#   source = "./modules/ecr"

#   name = "${local.project}/backend"
#   tags = local.common_tags
# }

# module "logs" {
#   source = "./modules/logs"

#   name              = "/ecs/${local.project}/backend"
#   retention_in_days = 30
#   tags              = local.common_tags
# }

# module "secrets" {
#   source = "./modules/secrets"

#   name             = "${local.project}/database"
#   username         = var.db_username
#   password         = var.db_password
#   password_version = var.db_password_version
#   tags             = local.common_tags
# }

# module "rds" {
#   source = "./modules/rds"

#   identifier        = local.project
#   subnet_ids        = module.subnet.private_subnet_ids
#   security_group_id = module.security_groups.rds_id
#   username          = var.db_username
#   password          = var.db_password
#   password_version  = var.db_password_version
#   tags              = local.common_tags
# }

# module "iam" {
#   source = "./modules/iam"

#   name               = local.project
#   secret_arn         = module.secrets.arn
#   log_group_arn      = module.logs.arn
#   ecr_repository_arn = module.ecr.arn
#   tags               = local.common_tags
# }

# module "ecs" {
#   source = "./modules/ecs"

#   name               = local.project
#   image              = var.backend_image != "" ? var.backend_image : module.ecr.repository_url
#   port               = var.backend_port
#   cluster_name       = local.project
#   subnet_ids         = module.subnet.public_subnet_ids
#   security_group_id  = module.security_groups.ecs_id
#   execution_role_arn = module.iam.execution_role_arn
#   task_role_arn      = module.iam.task_role_arn
#   secret_arn         = module.secrets.arn
#   log_group_name     = module.logs.name
#   database_host      = module.rds.address
#   desired_count      = 0
#   region             = local.region
#   tags               = local.common_tags
# }

# module "alb" {
#   source = "./modules/alb"

#   name              = local.project
#   vpc_id            = module.vpc.id
#   subnet_ids        = module.subnet.public_subnet_ids
#   security_group_id = module.security_groups.alb_id
#   target_port       = var.backend_port
#   tags              = local.common_tags
# }

# resource "aws_ecs_service" "backend" {
#   name            = local.project
#   cluster         = module.ecs.cluster_id
#   task_definition = module.ecs.task_definition_arn
#   desired_count   = 0
#   launch_type     = "FARGATE"

#   network_configuration {
#     subnets          = module.subnet.public_subnet_ids
#     security_groups  = [module.security_groups.ecs_id]
#     assign_public_ip = true
#   }

#   load_balancer {
#     target_group_arn = module.alb.target_group_arn
#     container_name   = "backend"
#     container_port   = var.backend_port
#   }

#   tags = local.common_tags
# }
