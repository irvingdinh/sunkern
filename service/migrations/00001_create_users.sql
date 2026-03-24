-- +sunkern Up
CREATE TABLE users (
    id         TEXT PRIMARY KEY,
    email      TEXT NOT NULL,
    name       TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    deleted_at TEXT
);

CREATE UNIQUE INDEX idx_users_email_active ON users(email) WHERE deleted_at IS NULL;

-- +sunkern Down
DROP TABLE IF EXISTS users;
