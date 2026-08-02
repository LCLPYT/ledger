db:
    docker compose up postgres -d --wait

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