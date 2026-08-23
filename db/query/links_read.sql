-- name: GetLinkByshortName :one
SELECT
    id,
    original_url,
    short_name,
    created_at
FROM links
WHERE short_name = $1;

-- name: GetLinks :many
SELECT
    id,
    original_url,
    short_name,
    created_at
FROM links;

-- name: GetLinkById :one
SELECT DISTINCT
    id,
    original_url,
    short_name,
    created_at
FROM links
WHERE id = $1;
