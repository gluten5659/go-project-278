-- name: GetLinkByShortName :one
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
FROM links
ORDER BY id
LIMIT sqlc.arg(page_size)::bigint OFFSET sqlc.arg(page_offset)::bigint;

-- name: CountLinks :one
SELECT count(*)
FROM links;

-- name: GetLinkById :one
SELECT DISTINCT
    id,
    original_url,
    short_name,
    created_at
FROM links
WHERE id = $1;
