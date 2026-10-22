default:
    just -l

DB_DSN := "postgres://db:db@localhost:5432/db?sslmode=disable"

# Committed next to each client package so release-please attributes proto changes to it.
PROTO_HASH_FILES := "jvm/core/proto.sha256 jvm/kotlin/proto.sha256"

check: test fmt-check lint proto-hash-check check-jvm

gradlew *args:
    cd jvm && ./gradlew {{ args }}

check-jvm:
    just gradlew check

alias db-gen := bobgen

build:
    go build .

run *args: db
    go run . {{args}}

serve:
    just run serve

tui: db
    go run . tui

migrate: db
    go run github.com/pressly/goose/v3/cmd/goose@$(go list -m -f '{{{{.Version}}' github.com/pressly/goose/v3) \
        -dir internal/db/migrations postgres "{{DB_DSN}}" up

bobgen: migrate
    PSQL_DSN="{{DB_DSN}}" go run github.com/stephenafamo/bob/gen/bobgen-psql@$(go list -m -f '{{{{.Version}}' github.com/stephenafamo/bob) -c bobgen.yaml

    # bob doesn't support custom destination dir for generated queries through config right now...
    # https://github.com/stephenafamo/bob/issues/566
    rm -rf gen/go/bob/queries
    mkdir -p gen/go/bob/queries
    mv internal/db/queries/*.bob.go internal/db/queries/*.bob.sql gen/go/bob/queries/

buf-update:
	cd proto && buf dep update

buf-gen: && proto-hash
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -z "$(find proto/apis -name '*.proto' 2>/dev/null)" ]; then
        echo "no protos to generate"
        exit 0
    fi
    buf generate --template buf.gen.go.yaml

proto-hash:
    #!/usr/bin/env bash
    set -euo pipefail
    hash="$({{just_executable()}} _proto-hash)"
    for f in {{PROTO_HASH_FILES}}; do
        echo "$hash" > "$f"
    done

proto-hash-check:
    #!/usr/bin/env bash
    set -euo pipefail
    hash="$({{just_executable()}} _proto-hash)"
    status=0
    for f in {{PROTO_HASH_FILES}}; do
        if [ "$(cat "$f" 2>/dev/null)" != "$hash" ]; then
            echo "$f is out of date, run 'just proto-hash'" >&2
            status=1
        fi
    done
    exit $status

[private]
_proto-hash:
    @find proto -type f -name '*.proto' -print0 | LC_ALL=C sort -z | xargs -0 -r sha256sum | sha256sum | cut -d' ' -f1

db:
    docker compose up -d --wait postgres

test-db:
    docker compose up -d --wait postgres_test

[parallel]
test: test-go test-jvm

test-go: test-db
    go test ./...

test-jvm:
    just gradlew test

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