module "vpc" {
  source = "./modules/vpc"

  cidr_block = "10.0.0.0/16"
  tags       = local.common_tags
}

module "subnet" {
  source = "./modules/subnet"

  vpc_id             = module.vpc.id
  availability_zones = ["ap-northeast-1a", "ap-northeast-1c"]
  public_cidrs       = ["10.0.0.0/18", "10.0.64.0/18"]
  private_cidrs      = ["10.0.128.0/18", "10.0.192.0/18"]
  tags               = local.common_tags
}

module "internet_gateway" {
  source = "./modules/internet_gateway"

  vpc_id = module.vpc.id
  tags   = local.common_tags
}

module "route_table" {
  source = "./modules/route_table"

  vpc_id              = module.vpc.id
  internet_gateway_id = module.internet_gateway.id
  public_subnet_ids   = module.subnet.public_subnet_ids
  private_subnet_ids  = module.subnet.private_subnet_ids
  tags                = local.common_tags
}

module "security_groups" {
  source = "./modules/security_groups"

  name   = local.project
  vpc_id = module.vpc.id
  tags   = local.common_tags
}

module "ecr" {
  source = "./modules/ecr"

  name = "${local.project}/backend"
  tags = local.common_tags
}

module "ecr_migration" {
  source = "./modules/ecr"

  name = "${local.project}/migration"
  tags = local.common_tags
}

module "logs" {
  source = "./modules/logs"

  name              = "/ecs/${local.project}/backend"
  retention_in_days = 30
  tags              = local.common_tags
}

module "secrets" {
  source = "./modules/secrets"

  name             = "${local.project}/database"
  username         = var.db_username
  password         = var.db_password
  password_version = var.db_password_version
  tags             = local.common_tags
}

module "rds" {
  source = "./modules/rds"

  identifier        = local.project
  subnet_ids        = module.subnet.private_subnet_ids
  security_group_id = module.security_groups.rds_id
  username          = var.db_username
  password          = var.db_password
  password_version  = var.db_password_version
  tags              = local.common_tags
}

module "elasticache" {
  source = "./modules/elasticache"

  name              = "${local.project}-auth"
  subnet_ids        = module.subnet.private_subnet_ids
  security_group_id = module.security_groups.redis_id
  tags              = local.common_tags
}

module "iam" {
  source = "./modules/iam"

  name                         = local.project
  parameter_arns               = module.ssm_parameter.parameter_arns
  parameter_path_arn           = "arn:aws:ssm:${local.region}:${local.account_id}:parameter/${local.project}/*"
  log_group_arn                = module.logs.arn
  ecr_repository_arn           = module.ecr.arn
  migration_ecr_repository_arn = module.ecr_migration.arn
  github_repository            = "Kyoya67@86813838/passkey-todo@1425009180"
  github_branch                = "main"
  ecs_cluster_arn              = "arn:aws:ecs:${local.region}:${local.account_id}:cluster/${local.project}"
  amplify_app_arn              = module.amplify.app_arn
  tags                         = local.common_tags
}

module "ssm_parameter" {
  source = "./modules/ssm_parameter"

  name = local.project
  config = {
    APP_PORT             = tostring(local.backend_port)
    CORS_ALLOWED_ORIGINS = "https://ca-techlounge-todo.kyo8.dev,http://localhost:8081"
    CSRF_ALLOWED_ORIGINS = "https://ca-techlounge-todo.kyo8.dev"
    DB_HOST              = module.rds.address
    DB_NAME              = "todo"
    DB_PORT              = "5432"
    DB_SSLMODE           = "verify-full"
    DB_SSLROOTCERT       = "/etc/ssl/certs/rds-global-bundle.pem"
    REDIS_ADDR           = "${module.elasticache.primary_endpoint_address}:6379"
    REDIS_TLS            = "true"
    WEBAUTHN_ORIGIN      = "https://ca-techlounge-todo.kyo8.dev"
    WEBAUTHN_RP_ID       = "ca-techlounge-todo.kyo8.dev"
  }
  secure_config = {
    DB_USER     = var.db_username
    DB_PASSWORD = var.db_password
  }
  secure_config_version = var.db_password_version
  tags                  = local.common_tags
}

module "ecs" {
  source = "./modules/ecs"

  name               = local.project
  image              = local.backend_image != "" ? local.backend_image : module.ecr.repository_url
  migration_image    = local.migration_image != "" ? local.migration_image : module.ecr_migration.repository_url
  port               = local.backend_port
  cluster_name       = local.project
  subnet_ids         = module.subnet.public_subnet_ids
  security_group_id  = module.security_groups.ecs_id
  execution_role_arn = module.iam.execution_role_arn
  task_role_arn      = module.iam.task_role_arn
  parameter_arns     = module.ssm_parameter.parameter_arns
  log_group_name     = module.logs.name
  region             = local.region
  tags               = local.common_tags
}

module "acm" {
  source = "./modules/acm"

  domain_name = "api.ca-techlounge-todo.kyo8.dev"
  zone_name   = "ca-techlounge-todo.kyo8.dev"
  tags        = local.common_tags
}

module "alb" {
  source = "./modules/alb"

  name              = local.project
  vpc_id            = module.vpc.id
  subnet_ids        = module.subnet.public_subnet_ids
  security_group_id = module.security_groups.alb_id
  target_port       = local.backend_port
  certificate_arn   = module.acm.certificate_arn
  tags              = local.common_tags
}

module "waf" {
  source  = "./modules/waf"
  name    = local.project
  alb_arn = module.alb.arn
  tags    = local.common_tags
}

module "route53" {
  source = "./modules/route53"

  zone_name    = "ca-techlounge-todo.kyo8.dev"
  record_name  = "api.ca-techlounge-todo.kyo8.dev"
  alb_dns_name = module.alb.dns_name
  alb_zone_id  = module.alb.zone_id
  target       = local.api_target
  lightsail_ip = local.lightsail_static_ip
  tags         = local.common_tags
}

module "amplify" {
  source = "./modules/amplify"

  name        = "${local.project}-frontend"
  branch_name = "main"
  domain_name = "ca-techlounge-todo.kyo8.dev"
  tags        = local.common_tags
}

resource "aws_ecs_service" "backend" {
  name            = local.project
  cluster         = module.ecs.cluster_id
  task_definition = module.ecs.task_definition_arn
  desired_count   = 1
  launch_type     = "FARGATE"

  lifecycle {
    ignore_changes = [
      task_definition,
    ]
  }

  network_configuration {
    subnets          = module.subnet.public_subnet_ids
    security_groups  = [module.security_groups.ecs_id]
    assign_public_ip = true
  }

  load_balancer {
    target_group_arn = module.alb.target_group_arn
    container_name   = "backend"
    container_port   = local.backend_port
  }

  tags = local.common_tags
}
