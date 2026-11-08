module "vpc" {
  count  = local.is_ecs ? 1 : 0
  source = "./modules/vpc"

  cidr_block = "10.0.0.0/16"
  tags       = local.common_tags
}

module "subnet" {
  count  = local.is_ecs ? 1 : 0
  source = "./modules/subnet"

  vpc_id             = try(module.vpc[0].id, null)
  availability_zones = ["ap-northeast-1a", "ap-northeast-1c"]
  public_cidrs       = ["10.0.0.0/18", "10.0.64.0/18"]
  private_cidrs      = ["10.0.128.0/18", "10.0.192.0/18"]
  tags               = local.common_tags
}

module "internet_gateway" {
  count  = local.is_ecs ? 1 : 0
  source = "./modules/internet_gateway"

  vpc_id = try(module.vpc[0].id, null)
  tags   = local.common_tags
}

module "route_table" {
  count  = local.is_ecs ? 1 : 0
  source = "./modules/route_table"

  vpc_id              = try(module.vpc[0].id, null)
  internet_gateway_id = try(module.internet_gateway[0].id, null)
  public_subnet_ids   = try(module.subnet[0].public_subnet_ids, [])
  private_subnet_ids  = try(module.subnet[0].private_subnet_ids, [])
  tags                = local.common_tags
}

module "security_groups" {
  count  = local.is_ecs ? 1 : 0
  source = "./modules/security_groups"

  name   = local.project
  vpc_id = try(module.vpc[0].id, null)
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

module "rds" {
  count  = local.is_ecs ? 1 : 0
  source = "./modules/rds"

  identifier        = local.project
  subnet_ids        = try(module.subnet[0].private_subnet_ids, [])
  security_group_id = try(module.security_groups[0].rds_id, null)
  username          = var.db_username
  password          = var.db_password
  password_version  = var.db_password_version
  tags              = local.common_tags
}

module "elasticache" {
  count  = local.is_ecs ? 1 : 0
  source = "./modules/elasticache"

  name              = "${local.project}-auth"
  subnet_ids        = try(module.subnet[0].private_subnet_ids, [])
  security_group_id = try(module.security_groups[0].redis_id, null)
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
    DB_HOST              = local.is_ecs ? try(module.rds[0].address, "") : "db"
    DB_NAME              = "todo"
    DB_PORT              = "5432"
    DB_SSLMODE           = "verify-full"
    DB_SSLROOTCERT       = "/etc/ssl/certs/rds-global-bundle.pem"
    REDIS_ADDR           = local.is_ecs ? "${try(module.elasticache[0].primary_endpoint_address, "")}:6379" : "redis:6379"
    REDIS_TLS            = local.is_ecs ? "true" : "false"
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
  count  = local.is_ecs ? 1 : 0
  source = "./modules/ecs"

  name               = local.project
  image              = local.backend_image != "" ? local.backend_image : module.ecr.repository_url
  migration_image    = local.migration_image != "" ? local.migration_image : module.ecr_migration.repository_url
  port               = local.backend_port
  cluster_name       = local.project
  subnet_ids         = try(module.subnet[0].public_subnet_ids, [])
  security_group_id  = try(module.security_groups[0].ecs_id, null)
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
  count  = local.is_ecs ? 1 : 0
  source = "./modules/alb"

  name              = local.project
  vpc_id            = try(module.vpc[0].id, null)
  subnet_ids        = try(module.subnet[0].public_subnet_ids, [])
  security_group_id = try(module.security_groups[0].alb_id, null)
  target_port       = local.backend_port
  certificate_arn   = module.acm.certificate_arn
  tags              = local.common_tags
}

module "waf" {
  count   = local.is_ecs ? 1 : 0
  source  = "./modules/waf"
  name    = local.project
  alb_arn = try(module.alb[0].arn, null)
  tags    = local.common_tags
}

module "route53" {
  source = "./modules/route53"

  zone_name    = "ca-techlounge-todo.kyo8.dev"
  record_name  = "api.ca-techlounge-todo.kyo8.dev"
  alb_dns_name = try(module.alb[0].dns_name, "")
  alb_zone_id  = try(module.alb[0].zone_id, "")
  target       = local.is_ecs ? "alb" : "lightsail"
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
  count           = local.is_ecs ? 1 : 0
  name            = local.project
  cluster         = try(module.ecs[0].cluster_id, null)
  task_definition = try(module.ecs[0].task_definition_arn, null)
  desired_count   = 1
  launch_type     = "FARGATE"

  lifecycle {
    ignore_changes = [
      task_definition,
    ]
  }

  network_configuration {
    subnets          = try(module.subnet[0].public_subnet_ids, [])
    security_groups  = [try(module.security_groups[0].ecs_id, null)]
    assign_public_ip = true
  }

  load_balancer {
    target_group_arn = try(module.alb[0].target_group_arn, null)
    container_name   = "backend"
    container_port   = local.backend_port
  }

  tags = local.common_tags
}
