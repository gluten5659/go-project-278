-- name: GetLinkVisits :many
SELECT
    id,
    link_id,
    ip,
    user_agent,
    referer,
    status,
    created_at
FROM link_visits
ORDER BY id
LIMIT sqlc.arg(page_size)::bigint OFFSET sqlc.arg(page_offset)::bigint;

-- name: CountLinkVisits :one
SELECT count(*)
FROM link_visits;
