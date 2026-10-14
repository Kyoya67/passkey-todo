-- name: CreateUser :one
INSERT INTO users (id)
VALUES ($1)
RETURNING id, created_at;

-- name: GetUserByID :one
SELECT id, created_at
FROM users
WHERE id = $1;

-- name: CreatePasskeyCredential :one
INSERT INTO passkey_credentials (credential_id, user_id, credential_data)
VALUES ($1, $2, $3)
RETURNING credential_id, user_id, credential_data, created_at, last_used_at;

-- name: GetPasskeyCredential :one
SELECT credential_id, user_id, credential_data, created_at, last_used_at
FROM passkey_credentials
WHERE credential_id = $1;

-- name: ListPasskeyCredentialsByUser :many
SELECT credential_id, user_id, credential_data, created_at, last_used_at
FROM passkey_credentials
WHERE user_id = $1
ORDER BY created_at, credential_id;
