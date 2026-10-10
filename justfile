default:
    just -l

DB_DSN := "postgres://db:db@localhost:5432/db?sslmode=disable"

check: test fmt-check lint

alias db-gen := bobgen

build:
    go build .

migrate: db
    go run github.com/pressly/goose/v3/cmd/goose@$(go list -m -f '{{{{.Version}}' github.com/pressly/goose/v3) \
        -dir internal/db/migrations postgres "{{DB_DSN}}" up

bobgen: migrate
    PSQL_DSN="{{DB_DSN}}" go run github.com/stephenafamo/bob/gen/bobgen-psql@$(go list -m -f '{{{{.Version}}' github.com/stephenafamo/bob) -c bobgen.yaml

buf-update:
	cd proto && buf dep update

buf-gen:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -z "$(find proto/apis -name '*.proto' 2>/dev/null)" ]; then
        echo "no protos to generate"
        exit 0
    fi
    buf generate --template buf.gen.go.yaml

db:
    docker compose up -d --wait postgres

test-db:
    docker compose up -d --wait postgres_test

test: test-db
    go test ./...

stop-db:
    docker compose down postgres

stop-test-db:
    docker compose down postgres_test

reset-db: stop-db
    docker volume rm ledger_pgdata

fmt: fmt-go fmt-proto

fmt-go:
    golangci-lint fmt

fmt-proto:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -z "$(find proto/apis -name '*.proto' 2>/dev/null)" ]; then
        echo "no protos to format"
        exit 0
    fi
    cd proto && buf format -w

# Prints the diff and fails when anything is unformatted.
fmt-check:
    #!/usr/bin/env bash
    set -euo pipefail
    golangci-lint fmt --diff
    if [ -z "$(find proto/apis -name '*.proto' 2>/dev/null)" ]; then
        echo "no protos to format"
        exit 0
    fi
    cd proto && buf format --diff --exit-code

lint: lint-go lint-proto vet

vet:
    go vet ./...

# golangci-lint runs govet
lint-go:
    golangci-lint run

lint-proto:
    #!/usr/bin/env bash
    set -euo pipefail
    protos="$(cd proto && find apis -name '*.proto' 2>/dev/null | sort || true)"
    if [ -z "$protos" ]; then
        echo "no protos to lint"
        exit 0
    fi
    cd proto
    buf lint
    api-linter --descriptor-set-in <(buf build . -o -#format=binpb) \
        -I . --config api-linter.yaml --set-exit-status $protos