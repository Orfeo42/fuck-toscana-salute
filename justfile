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

key_file := env_var_or_default("FTS_KEY_FILE", env_var("HOME") / ".config/fuck-toscana-salute/secret.key")

container-key:
    #!/usr/bin/env bash
    set -euo pipefail
    key_file="{{ key_file }}"
    if [[ -f "${key_file}" ]]; then
        echo "key present: ${key_file}"
        exit 0
    fi
    mkdir -p "$(dirname "${key_file}")"
    (umask 077 && openssl rand -base64 32 > "${key_file}")
    echo "key created: ${key_file}"

db_password_file := env_var_or_default("FTS_DB_PASSWORD_FILE", env_var("HOME") / ".config/fuck-toscana-salute/db-password")

db-password:
    #!/usr/bin/env bash
    set -euo pipefail
    password_file="{{ db_password_file }}"
    if [[ -f "${password_file}" ]]; then
        echo "db password present: ${password_file}"
        exit 0
    fi
    mkdir -p "$(dirname "${password_file}")"
    (umask 077 && openssl rand -hex 32 > "${password_file}")
    echo "db password created: ${password_file}"

up: container-key db-password
    docker compose up -d --build

down:
    docker compose down

logs:
    docker compose logs -f
