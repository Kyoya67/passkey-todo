AWS_PROFILE ?= ca-techlounge-todo-terraform
AWS_REGION ?= ap-northeast-1
ECS_CLUSTER ?= kyoya67-todo
ECS_SERVICE ?= kyoya67-todo

.PHONY: db-migrate-up db-migrate-down db-migrate-force db-migrate-ecs ecs-start ecs-stop ecs-status

db-migrate-up:
	docker compose --profile migration run --rm migration up

db-migrate-down:
	docker compose --profile migration run --rm migration down 1

db-migrate-force:
	@test -n "$(VERSION)" || (echo "Usage: make db-migrate-force VERSION=<migration_version>" && exit 1)
	docker compose --profile migration run --rm migration force $(VERSION)

db-migrate-ecs:
	@set -eu; \
	cluster=$$(terraform -chdir=infra output -raw ecs_cluster_name); \
	task_definition=$$(terraform -chdir=infra output -raw migration_task_definition_arn); \
	subnets=$$(terraform -chdir=infra output -json migration_subnet_ids | jq -r 'join(",")'); \
	security_group=$$(terraform -chdir=infra output -raw ecs_security_group_id); \
	AWS_PROFILE=$(AWS_PROFILE) AWS_REGION=$(AWS_REGION) aws ecs run-task \
		--cluster "$$cluster" \
		--task-definition "$$task_definition" \
		--launch-type FARGATE \
		--network-configuration "awsvpcConfiguration={subnets=[$$subnets],securityGroups=[$$security_group],assignPublicIp=ENABLED}" \
		--query 'tasks[0].taskArn' \
		--output text > /tmp/kyoya67-todo-migration-task-arn; \
	task_arn=$$(cat /tmp/kyoya67-todo-migration-task-arn); \
	AWS_PROFILE=$(AWS_PROFILE) AWS_REGION=$(AWS_REGION) aws ecs wait tasks-stopped \
		--cluster "$$cluster" \
		--tasks "$$task_arn"; \
	exit_code=$$(AWS_PROFILE=$(AWS_PROFILE) AWS_REGION=$(AWS_REGION) aws ecs describe-tasks \
		--cluster "$$cluster" \
		--tasks "$$task_arn" \
		--query 'tasks[0].containers[0].exitCode' \
		--output text); \
	echo "Migration task exit code: $$exit_code"; \
	test "$$exit_code" = 0

ecs-start:
	AWS_PROFILE=$(AWS_PROFILE) AWS_REGION=$(AWS_REGION) aws ecs update-service \
		--cluster $(ECS_CLUSTER) \
		--service $(ECS_SERVICE) \
		--desired-count 1

ecs-stop:
	AWS_PROFILE=$(AWS_PROFILE) AWS_REGION=$(AWS_REGION) aws ecs update-service \
		--cluster $(ECS_CLUSTER) \
		--service $(ECS_SERVICE) \
		--desired-count 0

ecs-status:
	AWS_PROFILE=$(AWS_PROFILE) AWS_REGION=$(AWS_REGION) aws ecs describe-services \
		--cluster $(ECS_CLUSTER) \
		--services $(ECS_SERVICE) \
		--query 'services[0].{desired:desiredCount,running:runningCount,pending:pendingCount,status:status}' \
		--output table
