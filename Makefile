.PHONY: db-migrate-up db-migrate-down db-migrate-force

db-migrate-up:
	docker compose --profile migration run --rm migration up

db-migrate-down:
	docker compose --profile migration run --rm migration down 1

db-migrate-force:
	@test -n "$(VERSION)" || (echo "Usage: make db-migrate-force VERSION=<migration_version>" && exit 1)
	docker compose --profile migration run --rm migration force $(VERSION)
