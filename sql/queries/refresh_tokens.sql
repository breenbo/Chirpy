-- name: CreateRefreshToken :one
INSERT INTO refresh_token (token, created_at, updated_at, user_id, expires_at)
VALUES ($1, NOW(), NOW(), $2, $3)
RETURNING *;

-- name: RevokeRefreshToken :one
UPDATE refresh_token SET revoked_at = NOW(),
updated_at = NOW()
WHERE token = $1
RETURNING *;

-- name: GetUserFromRefreshToken :one
SELECT users.* FROM users
JOIN refresh_token ON users.id = refresh_token.user_id
WHERE refresh_token.token = $1
AND revoked_at IS NULL
AND expires_at > NOW();
