set shell := ["bash", "-euo", "pipefail", "-c"]

web-install:
    npm --prefix web ci

web-gen:
    npm --prefix web run gen:api

web-build:
    npm --prefix web run build

build: web-build
    go build -o bin/fts ./cmd/fts

test pkg="./...":
    go test -short -race {{ pkg }}

web-test:
    npm --prefix web test

lint pkg="./...":
    golangci-lint run {{ pkg }}

web-lint:
    npm --prefix web run lint

openapi addr="127.0.0.1:8787":
    curl -fsS http://{{ addr }}/openapi.json | jq . > web/openapi.json

run: build
    ./bin/fts
