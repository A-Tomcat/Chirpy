-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email)
VALUES (
    gen_random_uuid(),
    NOW(),
    NOW(),
    $1
)
RETURNING *;

-- name: GetUser :one
SELECT * FROM users
WHERE id =  $1;

-- name: Reset :exec
DELETE FROM users;

-- name: GetUsers :many
SELECT * FROM users;

-- name: GetUserFromEmail :one
SELECT *
FROM users
WHERE email = $1;

-- name: AddPassword :exec
UPDATE users
SET hashed_password = $2
WHERE id = $1;