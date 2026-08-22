-- +goose Up
CREATE TABLE links (
    id int NOT NULL,
    original_url text NOT NULL,
    short_name varchar(191) NOT NULL,
    created_at timestamp NOT NULL DEFAULT now(),
    PRIMARY KEY (id)
);
CREATE UNIQUE INDEX idx_short_name ON links (short_name);

-- +goose Down
DROP TABLE links;
