-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (created_at, updated_at, revoked_at, expires_at, token, user_id)
VALUES (
    NOW(),
    NOW(),
    NULL,
    $1,
    $2,
    $3
)
RETURNING *;

-- name: GetRefreshToken :one
SELECT * 
FROM refresh_tokens
WHERE token = $1;

-- name: GetUserFromRefreshToken :one
SELECT user_id
FROM refresh_tokens
WHERE token = $1;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET revoked_at = NOW(), updated_at = NOW()
WHERE token = $1;