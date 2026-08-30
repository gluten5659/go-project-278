-- name: CreateLinkVisit :one
INSERT INTO link_visits (
    link_id,
    ip,
    user_agent,
    referer,
    status
) VALUES ($1, $2, $3, $4, $5)
RETURNING id, link_id, ip, user_agent, referer, status, created_at;
