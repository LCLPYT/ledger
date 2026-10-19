# ledger
A lightweight backend service for Minecraft game and player data, written in Go.

## Hosting the Go backend

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
| `modules.asset_repo.download_timeout`  | `10m`    | Maximum time to send a file, slower downloads are aborted                          |

The asset repository is disabled when no repos are configured.
Repo ids have to match `^[a-z0-9_-]+$` and become part of the URL `/v1/assets/<id>/`.
A repo directory is created on startup if it does not exist. 
Files and directories starting with `.` are never served.
Since they are map keys, repos can be added or overridden via environment variables, e.g. `LEDGER__MODULES__ASSET_REPO__REPOS__PRIVATE__PATH`.

### Health check

`ledger healthcheck` can be used for container health checks.

## Using the clients

Ledger uses [protobufs](https://protobuf.dev/) located in `proto/`.
The repository provides the following first-party bindings:

- JVM: For use with Java, Kotlin, Scala etc. (`jvm/core`; for use with JRE 8 or later)
- Kotlin: Bindings for Kotlin Coroutines (`jvm/kotlin`; for use with JRE 17 or later)

### Kotlin, Java, Scala and Groovy (JVM)

JVM bindings for the protobuf client and server code are available via Maven.
The JVM bindings are written in Kotlin but are usable with any JVM language, such as Java, Scala or Groovy.

#### Using Gradle
Add this to your `build.gradle.kts`:
```kotlin
repositories {
    maven {
        url = uri("https://repo.lclpnet.work/repository/internal")
    }
}

dependencies {
    implementation("work.lclpnet.ledger:ledger-jvm-core:0.1.0")
    
    // if you are using kotlin, add this as well
    implementation("work.lclpnet.ledger:ledger-jvm-kotlin:0.1.0")
}
```

>[!TIP]
> For cleaner dependency management, use the [Gradle version catalog](https://docs.gradle.org/current/userguide/version_catalogs.html).
> This makes it way easier to manage dependencies and versions in one central place, especially for multi-project Gradle projects.

#### Updating protobufs
Project releases are managed using Google's [release-please](https://github.com/googleapis/release-please).
This project uses a monorepo-style-layout for all ledger-related things.

The `release-please-config.json` partitions the codebase into separate packages.
Changes to files within the specific package sources cause new releases to only the changed parts.
That means that a change in `proto/` currently doesn't automatically cause a new release in the clients (at least until [PR #2534](https://github.com/googleapis/release-please/pull/2534) is merged into release-please).

To bypass this, this project introduces a simple solution: each client pins a hash of the `proto/` directory in a file within its release-please file tree.
This way, changing the protos needs an update to the hash files using:
```bash
just proto-hash
```

There is a [CI action](./.github/workflows/proto-hash.yml) that verifies that these proto hash files are kept up-to-date.