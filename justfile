export DATABASE_URL := "postgres://db:db@localhost:5432/db?sslmode=disable"
export JWT_SECRET := "dev-secret"
export SMTP_HOST := "localhost"
export SMTP_PORT := "1025"
export SMTP_USER := ""
export SMTP_PASS := ""
export SMTP_FROM := "noreply@ledger.example.com"
export APP_URL := "http://localhost:3000"

db:
    docker compose up postgres -d --wait

mailpit:
    docker compose up mailpit -d --wait

stop_db:
    docker compose down postgres

delete_db: stop_db
    docker volume rm ledger_pgdata || true

serve: db mailpit
    go run .

psql: db
    docker compose exec -it postgres psql -U db

create_user: db
    go run cmd/create_user/main.go

init_roles: db
    go run cmd/init_roles/main.go

test filter="":
    docker compose down postgres_test
    docker volume rm ledger_pgdata_test || true
    docker compose up postgres_test -d --wait

    DATABASE_URL="postgres://db:db@localhost:5433/db?sslmode=disable" SMTP_HOST="" go test ./handlers/... -v {{ if filter != "" { "-run " + filter } else { "" } }}

    docker compose down postgres_test || true
    docker volume rm ledger_pgdata_test || true

build:
    go build -ldflags "-s -w" .

audit_go:
    go run golang.org/x/vuln/cmd/govulncheck@latest ./...

[working-directory: 'front']
audit_npm:
    npm audit

[working-directory: 'front']
front:
    npm install
    npm run dev

[parallel]
audit: audit_go audit_npm