.PHONY: db-migrate

MIGRATION_UP_DIR := backend/db/migration/up
MIGRATION_DOWN_DIR := backend/db/migration/down

db-migrate-up:
	@set -eu; \
	found=0; \
	for migration in $(MIGRATION_UP_DIR)/*.sql; do \
		[ -f "$$migration" ] || continue; \
		found=1; \
		echo "Applying $$migration"; \
		docker compose exec -T db sh -c 'psql -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" -f -' < "$$migration"; \
	done; \
	if [ "$$found" -eq 0 ]; then \
		echo "No migration files found in $(MIGRATION_UP_DIR)"; \
		exit 1; \
	fi

db-migrate-down:
	@set -eu; \
	found=0; \
	for migration in $$(find $(MIGRATION_DOWN_DIR) -type f -name '*.sql' | sort -r); do \
		found=1; \
		echo "Reverting $$migration"; \
		docker compose exec -T db sh -c 'psql -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" -f -' < "$$migration"; \
	done; \
	if [ "$$found" -eq 0 ]; then \
		echo "No migration files found in $(MIGRATION_DOWN_DIR)"; \
		exit 1; \
	fi
