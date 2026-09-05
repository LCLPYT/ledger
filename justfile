health_fmt := '{{.Health}}'

db:
    [ "$(docker compose ps postgres --format '{{ health_fmt }}')" = "healthy" ] || docker compose up postgres -d --wait

psql: db
    docker compose exec -it postgres psql -U db

stop_db:
    docker compose down postgres

delete_db: stop_db
    docker volume rm ledger_pgdata || true

serve: db
    go run . serve

migrate: db
    go run . db migrate

rollback: db
    go run . db rollback

status: db
    go run . db status

test_db:
    [ "$(docker compose ps postgres_test --format '{{ health_fmt }}')" = "healthy" ] || docker compose up postgres_test -d --wait

test: test_db
    DATABASE_PORT=5433 go test ./...

generate: migrate
    PSQL_DSN="$(go run . db dsn)" go run github.com/stephenafamo/bob/gen/bobgen-psql@v0.50.0 -c bobgen.yaml
    go build ./...