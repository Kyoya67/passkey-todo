.PHONY: db-migrate-up db-migrate-down db-migrate-force db-migrate-ecs ecs-start ecs-stop ecs-status check-aws-env

db-migrate-up:
	docker compose --profile migration run --rm migration up

db-migrate-down:
	docker compose --profile migration run --rm migration down 1

db-migrate-force:
	@test -n "$(VERSION)" || (echo "Usage: make db-migrate-force VERSION=<migration_version>" && exit 1)
	docker compose --profile migration run --rm migration force $(VERSION)

check-aws-env:
	@test -n "$(AWS_PROFILE)" && test -n "$(AWS_REGION)" && test -n "$(ECS_CLUSTER)" && test -n "$(ECS_SERVICE)" || \
		(echo "AWS_PROFILE, AWS_REGION, ECS_CLUSTER, and ECS_SERVICE are required; load .envrc with direnv" >&2; exit 1)

ecs-start: check-aws-env
	AWS_PROFILE=$(AWS_PROFILE) AWS_REGION=$(AWS_REGION) aws ecs update-service \
		--cluster $(ECS_CLUSTER) \
		--service $(ECS_SERVICE) \
		--desired-count 1

ecs-stop: check-aws-env
	AWS_PROFILE=$(AWS_PROFILE) AWS_REGION=$(AWS_REGION) aws ecs update-service \
		--cluster $(ECS_CLUSTER) \
		--service $(ECS_SERVICE) \
		--desired-count 0

ecs-status: check-aws-env
	AWS_PROFILE=$(AWS_PROFILE) AWS_REGION=$(AWS_REGION) aws ecs describe-services \
		--cluster $(ECS_CLUSTER) \
		--services $(ECS_SERVICE) \
		--query 'services[0].{desired:desiredCount,running:runningCount,pending:pendingCount,status:status}' \
		--output table
