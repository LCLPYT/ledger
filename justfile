DB_DSN := "postgres://db:db@localhost:5432/db?sslmode=disable"

db:
    docker compose up postgres -d --wait

stop_db:
    docker compose down

delete_db: stop_db
    docker volume rm ledger_pgdata

serve: db
    DATABASE_URL="{{DB_DSN}}" go run .

psql: db
    docker compose exec -it postgres psql -U db

create_user: db
    DATABASE_URL="{{DB_DSN}}" go run cmd/create_user/main.go

build_production:
    go build -ldflags "-s -w" .