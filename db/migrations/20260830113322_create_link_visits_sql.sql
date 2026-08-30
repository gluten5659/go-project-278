-- +goose Up
CREATE TABLE link_visits (
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
    link_id bigint NOT NULL REFERENCES links (id) ON DELETE CASCADE,
    ip varchar NOT NULL,
    user_agent varchar NOT NULL,
    referer varchar NOT NULL,
    status int NOT NULL,
    created_at timestamp NOT NULL DEFAULT now(),
    PRIMARY KEY (id)
);

-- +goose Down
DROP TABLE link_visits;
