CREATE TABLE users (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE passkey_credentials (
    credential_id BYTEA PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    credential_data JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,
    CONSTRAINT passkey_credentials_credential_id_nonempty CHECK (octet_length(credential_id) > 0)
);

CREATE INDEX idx_passkey_credentials_user_id
    ON passkey_credentials (user_id);
