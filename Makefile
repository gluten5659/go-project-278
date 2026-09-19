.PHONY: install start run build migrate test test-coverage lint fmt lint-fix

install:
	npm ci
	go mod download
start:
	npm start
run:
	go tool air
build:
	go build -o bin/app .
migrate:
	go tool goose -dir db/migrations postgres "$${DATABASE_DSN:-$$DATABASE_URL}" up
test:
	go test -race ./...
test-coverage:
	go test -race -coverprofile=coverage.out ./...
lint:
	go tool golangci-lint run
fmt:
	go tool golangci-lint fmt
lint-fix:
	go tool golangci-lint run --fix
