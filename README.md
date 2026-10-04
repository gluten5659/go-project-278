### Hexlet tests and linter status

[![Actions Status](https://github.com/gluten5659/go-project-278/actions/workflows/hexlet-check.yml/badge.svg)](https://github.com/gluten5659/go-project-278/actions)
[![CI](https://github.com/gluten5659/go-project-278/actions/workflows/ci.yml/badge.svg)](https://github.com/gluten5659/go-project-278/actions/workflows/ci.yml)

## Description

Link shortener with a REST API and an admin UI. It turns a long URL into a short
name, redirects visitors to the original address and records every visit.

## Install

Needs Go 1.26, Node.js 24 and PostgreSQL 17.

```
make install
export DATABASE_DSN="postgres://user:password@localhost:5432/database?sslmode=disable"
export BASE_URL="http://localhost:8080"
make migrate
```

`make install` pulls both the Node packages of the admin UI and the Go modules.
`make migrate` applies the migrations to the database from `DATABASE_DSN`.
`BASE_URL` is what the short links are built from, so the API refuses to start
without it.

## Usage

```
make start
```

The API listens on `http://localhost:8080` and the admin UI on
`http://localhost:5173`. `make run` starts the API alone, with live reload, when
the admin UI is not needed.

## Configuration

Everything is read from the environment. The database connection and the public
base address are both required, and without either of them the process refuses to
start. `DATABASE_URL` is there because some hosts and test harnesses pass the
connection string under that name.

`BASE_URL` has to describe the public entry point of the service, because every
short link handed out is built from it. It takes an http or https scheme and a
host with an optional port, and nothing else. A path, a query, a fragment or a
user is refused, a trailing slash is dropped.

| Variable               | Default                 | Description                                        |
|------------------------|-------------------------|----------------------------------------------------|
| `DATABASE_DSN`         | none, required          | PostgreSQL connection string                       |
| `DATABASE_URL`         | none                    | Read only when `DATABASE_DSN` is unset             |
| `BASE_URL`             | none, required          | Public address the short links are built from      |
| `HTTP_ADDR`            | `:8080`                 | Address the API listens on, a bare port is allowed |
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

`original_url` is required and has to be an http or https URL. `short_name` is
optional, three to thirty two characters, and unique across all links. It may
only hold latin letters, digits, dashes and underscores, because it has to fit
into a single segment of the short URL. A slash, a question mark, a hash, a
percent sign, a dot or a space would turn `/r/<short_name>` into an address that
does not reach the redirect.

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
with `404 Not Found` and a failure on our side answers with `500`. A path that
matches no route answers with the same `404` body, so a client parses every error
the same way.

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

Anything that answers `500` is also sent to Sentry with the method and the path of
the request, as long as `SENTRY_DSN` is set. Nothing else about the caller travels
with it, so the address, the user agent and the headers stay out of the report. A
panic is reported the same way and still answers `500`. Client errors never reach
Sentry, so a flood of bad requests cannot drown out the real failures.

The report leaves on its own and the handler does not wait for it, so a slow
Sentry never holds up an answer or a redirect. Whatever is still queued is sent
while the process shuts down.

### Visits

A redirect does not wait for its visit to be written, so a visit the database
does not take within five seconds, or during a shutdown, is lost.

### Pagination

Both list endpoints take `?range=[first,last]` and answer with a `Content-Range`
header of the form `links 0-9/42`. Both bounds are inclusive, so `[0,9]` asks for
ten records. That is what the admin UI reads to build its pager. Without the
parameter the whole collection comes back. A range asking for more than a thousand
records is cut down to a thousand.

The header always describes the records that came back, not the ones that were
asked for. A range starting after the last record has no records to describe, so
the header answers `links */42` and the client still reads the total from it.

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

The service runs at https://go-project-278-ym54.onrender.com. The admin UI opens
at the root, and the API answers under the same host over HTTPS, so `/ping`,
`/api/links` and every short link live at that address too. `BASE_URL` there is
set to the same address, because that is what the short links are built from, and
it has to follow the service whenever the deploy moves.

The `Dockerfile` builds the admin UI and the API into one image. Caddy serves the
UI and proxies everything else to the API, and `bin/run.sh` applies the
migrations before starting both.

Caddy is the only listener the platform sees, so the port the platform assigns
belongs to Caddy and never reaches the API. Caddy forwards to `localhost:8080`,
which means `HTTP_ADDR` inside the image has to keep that port.
