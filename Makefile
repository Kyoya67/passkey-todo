AWS_PROFILE ?= ca-techlounge-todo-terraform
AWS_REGION ?= ap-northeast-1
ECS_CLUSTER ?= kyoya67-todo
ECS_SERVICE ?= kyoya67-todo

.PHONY: db-migrate-up db-migrate-down db-migrate-force ecs-start ecs-stop ecs-status

db-migrate-up:
	docker compose --profile migration run --rm migration up

db-migrate-down:
	docker compose --profile migration run --rm migration down 1

db-migrate-force:
	@test -n "$(VERSION)" || (echo "Usage: make db-migrate-force VERSION=<migration_version>" && exit 1)
	docker compose --profile migration run --rm migration force $(VERSION)

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
