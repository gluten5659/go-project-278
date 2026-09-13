### Hexlet tests and linter status

[![Actions Status](https://github.com/gluten5659/go-project-278/actions/workflows/hexlet-check.yml/badge.svg)](https://github.com/gluten5659/go-project-278/actions)
[![CI](https://github.com/gluten5659/go-project-278/actions/workflows/ci.yml/badge.svg)](https://github.com/gluten5659/go-project-278/actions/workflows/ci.yml)

## Description

Link shortener with a REST API and an admin UI. It turns a long URL into a short
name, redirects visitors to the original address and records every visit.

## Install

Needs Go 1.26, Node.js 24 and PostgreSQL 17.

```
npm ci
export DATABASE_DSN="postgres://user:password@localhost:5432/database?sslmode=disable"
go tool goose -dir db/migrations postgres "$DATABASE_DSN" up
```

## Usage

```
npm start
```

The API listens on `http://localhost:8080` and the admin UI on
`http://localhost:5173`. `make run` starts the API alone, with live reload.

## Configuration

Everything is read from the environment. Only the database connection is
required, and without it the process refuses to start. `DATABASE_URL` is there
because some hosts and test harnesses pass the connection string under that name.

| Variable               | Default                 | Description                                        |
|------------------------|-------------------------|----------------------------------------------------|
| `DATABASE_DSN`         | none, required          | PostgreSQL connection string                       |
| `DATABASE_URL`         | none                    | Read only when `DATABASE_DSN` is unset             |
| `BASE_URL`             | `http://localhost:8080` | Address the short links are built from             |
| `SENTRY_DSN`           | none                    | Sentry project. Error reporting is off when unset  |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:5173` | Comma separated origins allowed to call the API    |

## API

| Method   | Path               | Description                                    |
|----------|--------------------|------------------------------------------------|
| `GET`    | `/ping`            | Health check, answers 503 when the database is unreachable |
| `GET`    | `/r/:short_name`   | Redirect to the original URL, record the visit |
| `GET`    | `/api/links`       | List links                                     |
| `POST`   | `/api/links`       | Create a link                                  |
| `GET`    | `/api/links/:id`   | Show a link                                    |
| `PUT`    | `/api/links/:id`   | Update a link                                  |
| `DELETE` | `/api/links/:id`   | Delete a link                                  |
| `GET`    | `/api/link_visits` | List visits                                    |

## Examples

```
curl -X POST http://localhost:8080/api/links \
  -H 'Content-Type: application/json' \
  -d '{"original_url": "https://hexlet.io", "short_name": "hexlet"}'

{"id":1,"original_url":"https://hexlet.io","short_name":"hexlet","short_url":"http://localhost:8080/r/hexlet","created_at":"2026-09-06T09:50:50.859538Z"}
```

```
curl -i http://localhost:8080/r/hexlet

HTTP/1.1 302 Found
Location: https://hexlet.io
```

## Behavior

A few rules are worth calling out explicitly so the responses never feel
surprising.

### Short names

`original_url` is required and has to be a URL. `short_name` is optional, three
to thirty two characters, and unique across all links.

When the field is left out on create, the server generates a name of eight
letters. A generated name can collide with one that already exists, so the
server tries three times before giving up and answering `503`. Update never
generates anything, because a short name is what people share and replacing it
behind their back would break every link already handed out. Changing it through
an update does break those links, so that is a deliberate call by whoever edits
the record.

### Errors

Every failure answers with a body. A request that never made it to validation
answers with `400 Bad Request`, which covers a body that is not valid JSON, an
identifier that is not a number and a malformed `range`. A missing link answers
with `404 Not Found` and a failure on our side answers with `500`.

```json
{"error": "invalid request"}
{"error": "not found"}
{"error": "internal server error"}
```

A body that parses but breaks a rule answers with `422 Unprocessable Entity` and
carries one message per field.

```json
{"errors": {"original_url": "Key: 'createLinkRequest.original_url' Error:Field validation for 'original_url' failed on the 'http_url' tag"}}
```

Uniqueness is the one rule the validator cannot check on its own, so the database
enforces it. A short name that is already taken comes back in the same shape,
which leaves the client a single place to look for field errors.

```json
{"errors": {"short_name": "short name already in use"}}
```

Anything that answers `500` is also sent to Sentry along with the request, as
long as `SENTRY_DSN` is set. A panic is reported the same way and still answers
`500`. Client errors never reach Sentry, so a flood of bad requests cannot drown
out the real failures.

### Pagination

Both list endpoints take `?range=[first,last]` and answer with a `Content-Range`
header of the form `links 0-9/42`. Both bounds are inclusive, so `[0,9]` asks for
ten records. That is what the admin UI reads to build its pager. Without the
parameter the whole collection comes back, and a range asking for more than a
thousand records is refused.

### Deleting a link

Deleting a link deletes the visits recorded for it. The statistics belong to the
link and do not outlive it.

## Development

```
make test
make lint
make build
```

Queries are generated from `db/query/*.sql` by [sqlc](https://sqlc.dev), so
after changing SQL run `go tool sqlc generate` and commit the result in
`internal/db`.

The query tests need a database of their own and skip when `TEST_DATABASE_DSN`
is unset. They apply the migrations themselves and run every case in a
transaction that is rolled back, so the database is left as they found it.

```
TEST_DATABASE_DSN="postgres://user:password@localhost:5432/database_test?sslmode=disable" make test
```

## Deployment

The `Dockerfile` builds the admin UI and the API into one image. Caddy serves the
UI and proxies everything else to the API, and `bin/run.sh` applies the
migrations before starting both.
