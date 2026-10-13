# ledger
A lightweight backend service for Minecraft game and player data, written in Go.

## Hosting

### Configuration

Ledger reads its settings from a YAML file and from environment variables. 
Sources are layered, with later ones overriding earlier ones:

1. `config.yaml` in the working directory (can also be overridden using `LEDGER__CONFIG_FILE`)
2. A `.local` sibling of that file, e.g. `config.local.yaml` (or `prod.local.yaml` for `prod.yaml`), if it exists
3. Environment variables prefixed with `LEDGER__`

`config.yaml` may be missing, in which case only defaults and environment variables apply. 
A file named via `LEDGER__CONFIG_FILE` has to exist. 

Durations are written like `30s` or `5m`, byte sizes like `4MiB`.

#### YAML file

All settings live under `modules.<module>`. 
A minimal production `config.yaml`, for example, can look like this:

```yaml
modules:
  grpcerr:
    domain: ledger.example.com

  grpcserver:
    addr: "0.0.0.0:9000"
    reflection: false

  postgres:
    url: postgres://ledger:secret@db.example.com:5432/ledger?sslmode=require

  logging:
    format: json

  asset_repo:
    repos:
      maps:
        path: /srv/ledger/maps
        public: true
```

#### Environment variables

Every yaml key can also be set via environment variable.
Take the dotted key path, uppercase it, replace each `.` with a double underscore `__` and prefix it with `LEDGER__`. 
Single underscores stay part of the key name.

| YAML key                             | Environment variable                             |
|--------------------------------------|--------------------------------------------------|
| `modules.grpcserver.reflection`      | `LEDGER__MODULES__GRPCSERVER__REFLECTION`        |
| `modules.postgres.url`               | `LEDGER__MODULES__POSTGRES__URL`                 |
| `modules.postgres.max_open_conns`    | `LEDGER__MODULES__POSTGRES__MAX_OPEN_CONNS`      |
| `modules.asset_repo.repos.maps.path` | `LEDGER__MODULES__ASSET_REPO__REPOS__MAPS__PATH` |

#### Reference

| Key                                    | Default  | Description                                                                        |
|----------------------------------------|----------|------------------------------------------------------------------------------------|
| `modules.grpcerr.domain`               | required | Domain reported in the `ErrorInfo` of every gRPC error                             |
| `modules.grpcserver.addr`              | `:50051` | gRPC listen address                                                                |
| `modules.grpcserver.grace_period`      | `5s`     | Time in-flight calls get to finish on shutdown                                     |
| `modules.grpcserver.reflection`        | `true`   | Enables gRPC server reflection; it is not covered by auth                          |
| `modules.grpcserver.health`            | `true`   | Registers the gRPC health service                                                  |
| `modules.grpcserver.max_receive_bytes` | `4MiB`   | Maximum size of a received message                                                 |
| `modules.postgres.url`                 | required | PostgreSQL connection URL                                                          |
| `modules.postgres.max_open_conns`      | `25`     | Maximum open connections                                                           |
| `modules.postgres.max_idle_conns`      | `5`      | Maximum idle connections                                                           |
| `modules.postgres.conn_max_lifetime`   | `30m`    | Maximum lifetime of a connection                                                   |
| `modules.postgres.conn_max_idle_time`  | `5m`     | Maximum idle time of a connection                                                  |
| `modules.postgres.connect_timeout`     | `10s`    | Timeout for the initial connection                                                 |
| `modules.migrate.lock_retry_interval`  | `1s`     | Wait between attempts to acquire the migration lock                                |
| `modules.migrate.lock_retries`         | `300`    | Attempts to acquire the migration lock                                             |
| `modules.logging.level`                | `info`   | One of `debug`, `info`, `warn`, `error`                                            |
| `modules.logging.format`               | `text`   | `text` or `json`                                                                   |
| `modules.auth.cache_ttl`               | `30s`    | How long authenticated credentials are cached                                      |
| `modules.asset_repo.addr`              | `:8080`  | HTTP listen address of the asset repository                                        |
| `modules.asset_repo.repos.<id>.path`   | required | Directory served by the repo `<id>`, absolute or relative to the working directory |
| `modules.asset_repo.repos.<id>.public` | `false`  | Serves the repo without auth, otherwise `assets.read:<id>` is required             |
| `modules.asset_repo.grace_period`      | `5s`     | Time in-flight requests get to finish on shutdown                                  |

The asset repository is disabled when no repos are configured.
Repo ids have to match `^[a-z0-9_-]+$` and become part of the URL `/v1/assets/<id>/`.
A repo directory is created on startup if it does not exist. 
Files and directories starting with `.` are never served.
Since they are map keys, repos can be added or overridden via environment variables, e.g. `LEDGER__MODULES__ASSET_REPO__REPOS__PRIVATE__PATH`.

### Health check

`ledger healthcheck` can be used for container health checks.
