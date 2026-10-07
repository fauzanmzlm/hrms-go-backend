BINARY := bin/api

# Load .env into the environment of every recipe when it exists. Make parses
# it, so values must not contain '#' (starts a comment) or '$' (expansion).
-include .env
export

.PHONY: run build test vet tidy db-up up down

run:
	go run ./cmd/api

build:
	go build -o $(BINARY) ./cmd/api

test:
	go test -race -cover ./...

vet:
	go vet ./...

tidy:
	go mod tidy

# Start only MySQL in Docker, for `make run` against it.
db-up:
	docker compose up -d mysql

# Run the full stack (MySQL + API) in Docker.
up:
	docker compose up --build

down:
	docker compose down
