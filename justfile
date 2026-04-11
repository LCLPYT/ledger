export DATABASE_URL := "postgres://db:db@localhost:5432/db?sslmode=disable"
export JWT_SECRET := "dev-secret"

db:
    docker compose up postgres -d --wait

stop_db:
    docker compose down postgres

delete_db: stop_db
    docker volume rm ledger_pgdata || true

serve: db
    go run .

psql: db
    docker compose exec -it postgres psql -U db

create_user: db
    go run cmd/create_user/main.go

init_roles: db
    go run cmd/init_roles/main.go

[env("DATABASE_URL", "postgres://db:db@localhost:5433/db?sslmode=disable")]
test filter="":
    docker compose down postgres_test
    docker volume rm ledger_pgdata_test || true
    docker compose up postgres_test -d --wait

    go test ./handlers/... -v {{ if filter != "" { "-run " + filter } else { "" } }}

    docker compose down postgres_test || true
    docker volume rm ledger_pgdata_test || true

build:
    go build -ldflags "-s -w" .

[working-directory: 'front']
front:
    npm install
    npm run dev