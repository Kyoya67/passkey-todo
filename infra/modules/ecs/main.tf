resource "aws_ecs_cluster" "this" {
  name = var.cluster_name
  tags = var.tags
}

resource "aws_ecs_task_definition" "this" {
  family                   = var.name
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
    secrets = [
      for name, arn in var.parameter_arns : { name = name, valueFrom = arn }
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
  family                   = "${var.name}-migration"
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
    secrets = [
      for name, arn in var.parameter_arns : { name = name, valueFrom = arn }
      if contains(["DB_HOST", "DB_NAME", "DB_PASSWORD", "DB_PORT", "DB_SSLMODE", "DB_SSLROOTCERT", "DB_USER"], name)
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
