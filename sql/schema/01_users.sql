-- +goose Up
CREATE TABLE users(
    id         UUID PRIMARY KEY,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    email      TEXT NOT NULL
);

-- +goose Down
DROP TABLE users;
