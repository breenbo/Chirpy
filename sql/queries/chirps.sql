-- name: CreateChirp :one
INSERT INTO chirps (id, body, user_id)
VALUES (gen_random_uuid(), $1, $2)
RETURNING *;

-- name: GetAllChirps :many
SELECT * FROM chirps;

-- name: GetOneChirp :one
SELECT * FROM chirps WHERE id = $1;
