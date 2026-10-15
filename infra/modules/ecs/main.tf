resource "aws_ecs_cluster" "this" {
  name = var.cluster_name
  tags = var.tags
}

resource "aws_ecs_task_definition" "this" {
  family                   = "${var.name}-${var.env}"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = var.execution_role_arn
  task_role_arn            = var.task_role_arn

  container_definitions = jsonencode([{
    name      = "backend"
    image     = var.image
    essential = true
    portMappings = [{
      containerPort = var.port
      hostPort      = var.port
      protocol      = "tcp"
    }]
    healthCheck = {
      command     = ["CMD-SHELL", "curl --fail --silent http://localhost:${var.port}/health || exit 1"]
      interval    = 30
      timeout     = 5
      retries     = 3
      startPeriod = 10
    }
    environment = [
      { name = "APP_PORT", value = tostring(var.port) },
      { name = "DB_HOST", value = var.database_host },
      { name = "DB_PORT", value = "5432" },
      { name = "DB_NAME", value = "todo" },
      { name = "DB_SSLMODE", value = "verify-full" },
      { name = "DB_SSLROOTCERT", value = "/etc/ssl/certs/rds-global-bundle.pem" },
      { name = "CORS_ALLOWED_ORIGINS", value = var.cors_allowed_origins },
      { name = "WEBAUTHN_RP_ID", value = "ca-techlounge-todo.kyo8.dev" },
      { name = "WEBAUTHN_ORIGIN", value = "https://ca-techlounge-todo.kyo8.dev" },
      { name = "REDIS_ADDR", value = "${var.redis_host}:6379" },
      { name = "REDIS_TLS", value = "true" }
    ]
    secrets = [
      { name = "DB_USER", valueFrom = "${var.secret_arn}:username::" },
      { name = "DB_PASSWORD", valueFrom = "${var.secret_arn}:password::" }
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = var.log_group_name
        awslogs-region        = var.region
        awslogs-stream-prefix = "backend"
      }
    }
  }])

  tags = var.tags
}

resource "aws_ecs_task_definition" "migration" {
  family                   = "${var.name}-${var.env}-migration"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = var.execution_role_arn
  task_role_arn            = var.task_role_arn

  container_definitions = jsonencode([{
    name      = "migration"
    image     = var.migration_image
    essential = true
    command   = ["up"]
    environment = [
      { name = "DB_HOST", value = var.database_host },
      { name = "DB_PORT", value = "5432" },
      { name = "DB_NAME", value = "todo" },
      { name = "DB_SSLMODE", value = "verify-full" },
      { name = "DB_SSLROOTCERT", value = "/etc/ssl/certs/rds-global-bundle.pem" }
    ]
    secrets = [
      { name = "DB_USER", valueFrom = "${var.secret_arn}:username::" },
      { name = "DB_PASSWORD", valueFrom = "${var.secret_arn}:password::" }
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = var.log_group_name
        awslogs-region        = var.region
        awslogs-stream-prefix = "migration"
      }
    }
  }])

  tags = var.tags
}
