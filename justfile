DB_DSN := "postgres://db:db@localhost:5432/db?sslmode=disable"

db:
    docker compose up postgres -d --wait

stop_db:
    docker compose down postgres

delete_db: stop_db
    docker volume rm ledger_pgdata || true

serve: db
    JWT_SECRET=dev DATABASE_URL="{{DB_DSN}}" go run .

psql: db
    docker compose exec -it postgres psql -U db

create_user: db
    JWT_SECRET=dev DATABASE_URL="{{DB_DSN}}" go run cmd/create_user/main.go

init_roles: db
    JWT_SECRET=dev DATABASE_URL="{{DB_DSN}}" go run cmd/init_roles/main.go

test filter="":
    docker compose down postgres_test
    docker volume rm ledger_pgdata_test || true
    docker compose up postgres_test -d --wait
    JWT_SECRET=test DATABASE_URL="postgres://db:db@localhost:5433/db?sslmode=disable" go test ./handlers/... -v {{ if filter != "" { "-run " + filter } else { "" } }}

build:
    go build -ldflags "-s -w" .