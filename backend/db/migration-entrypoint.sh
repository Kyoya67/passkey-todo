#!/bin/sh
set -eu

exec migrate \
  -path /app/migration \
  -database "postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE}&sslrootcert=${DB_SSLROOTCERT}" \
  "$@"
