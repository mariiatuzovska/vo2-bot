# vo2-bot

Telegram bot that pulls training data from Strava and Apple Health, stores it
in Postgres, and asks Claude for coaching recommendations on demand.

This repo is a work in progress.

## Stack

| Concern     | Choice                                                 |
| ----------- | ------------------------------------------------------ |
| Language    | Go 1.25                                                |
| DB          | Postgres 16 (Docker, provisioned by Terraform)         |
| Config      | `spf13/viper` (`.env` + env vars)                      |
| Migrations  | `golang-migrate`, embedded into the binary             |
| DB access   | `sqlc` (generated) for CRUD, hand-written `pgx.CopyFrom` for bulk inserts |
| Local infra | Terraform with `kreuzwerker/docker` provider           |
| Dev loop    | Tilt                                                   |
| Strava      | planned (`golang.org/x/oauth2` + `net/http`)           |
| Telegram    | planned (`go-telegram-bot-api/telegram-bot-api/v5`)    |
| Claude      | planned (`anthropic-sdk-go`, model `claude-opus-4-7`)  |

## Repo layout

```
vo2-bot/
  cmd/bot/main.go            entrypoint: config → migrate → open pool → wait
  embed.go                   embeds db/migrations/*.sql into the binary
  db/                        ALL SQL lives here:
    migrations/              golang-migrate up/down files
    queries.sql              hand-written sqlc queries
    sqlc.yaml                sqlc codegen config
  internal/
    config/                  viper-based env loader
    store/                   DB wrapper package:
      store.go               pgx pool (Open, Close)
      migrate.go             golang-migrate runner
      queries/               GENERATED sqlc code (package: queries)
    apple/                   Apple Health ingest: parse HAE archive,
                             orchestrate store calls, HTTP handler.
                             Imports internal/store/queries directly.
    httpx/                   shared HTTP helpers (Handle wrapper, WriteJSON)
    errs/                    typed HTTP errors (NewBadRequest, NewNotFound, …)
    strava/  telegram/  claude/   package stubs
  build/Dockerfile           production image (not used in dev loop)
  infra/                     Terraform: Postgres container, network, volume
  Tiltfile                   orchestrates init → tf-up → postgres → bot
  Makefile                   init / tf-up / tf-down / dev / clear / sqlc
  local/                     gitignored; holds Apple Health archives in dev
  .env.example
```

## What works today

- **Local Postgres via Terraform.** `make tf-up` brings up `vo2-postgres`
  (Postgres 16 in Docker, named volume, `pg_isready` healthcheck).
- **Embedded migrations.** `migrations/*.sql` are compiled into the binary
  via `embed.go` and applied on startup by `internal/store.Migrate`. No
  separate migrate step in the dev loop.
- **Tilt dev loop.** `tilt up` runs `make tf-up`, waits for Postgres to
  report healthy, then runs `go run ./cmd/bot` and restarts on source
  changes.
- **Config.** `internal/config` reads `.env` and environment variables,
  requires `DATABASE_URL`, and defaults `CLAUDE_MODEL`, `HTTP_ADDR`,
  `APPLE_ARCHIVE_DIR`.
- **Apple Health schema.** `migrations/0001_apple.up.sql` creates
  `apple_workouts`, `apple_workout_heart_rate`, `apple_workout_route`,
  `apple_daily_metrics`, and `apple_imports` (dedupe audit).
- **Apple Health import.** `internal/apple` parses a Health Auto Export
  zip, sha256-dedupes against `apple_imports`, and persists workouts +
  per-workout HR bins + GPS route + daily metrics in one transaction.
- **HTTP server.** The bot listens on `HTTP_ADDR` (default `:8080`) with
  graceful shutdown. `POST /apple/import` is live. Handlers return
  `error`; `internal/httpx.Handle` adapts them and `internal/errs`
  (`NewBadRequest`, `NewNotFound`, `NewConflict`, …) renders typed
  responses.
- **Strava / Telegram / Claude:** package stubs only.

## Apple Health: the local-first decision

Apple HealthKit has no public REST API, so the Apple side of this bot
reads an archive produced by the
[Health Auto Export](https://www.healthexportapp.com/) iOS app. The app
generates a zip on demand containing one big JSON file (workouts,
per-workout HR bins, GPS routes, daily metrics like HRV and VO2 max).

**We are still deciding how this will work in production.** For now, while
we are on Health Auto Export's free tier:

- **Local only.** The zip sits on the developer's filesystem under
  `local/apple/` (configurable via `APPLE_ARCHIVE_DIR`). An import is
  triggered manually by calling `POST /apple/import` with
  `{"source": "local", "name": "HealthAutoExport_*.zip"}`. If `name` is
  omitted, the bot picks the newest `HealthAutoExport_*.zip` in the
  archive dir (HAE embeds a timestamp in the filename, so
  lexicographic max == newest). `source=gcs` is rejected as not
  implemented.
- **Manual export, manual import.** We run the export from the phone,
  drop the zip in `local/apple/`, and fire the import ourselves.

**Later, we may automate this via the cloud.** The candidates on the
table are a paid Health Auto Export tier (automatic exports to iCloud/
Dropbox/GCS) plus a small server-side pull, or a GCS drop + notification
that triggers the import. The `Source` interface in `internal/apple`
already anticipates a `GCSSource`; implementing it, deciding who uploads
the zip, and deciding the import trigger (cron ping vs. GCS
notification) are all deferred until the local flow is proven.

## Local run sequence

Prereqs: Docker, Terraform, Go 1.25, Tilt.

```sh
# 1. One-time setup: copies .env + infra/terraform.tfvars from templates
#    (if absent), runs `terraform init`, installs sqlc.
make init

# 2. Fill in .env (STRAVA_*, TELEGRAM_*, ANTHROPIC_API_KEY) and the
#    Postgres password in infra/terraform.tfvars.

# 3. Dev loop (brings up Postgres + runs the bot)
make dev        # == tilt up

# 4. Tear down
make clear      # == tilt down && make tf-down
```

`go run ./cmd/bot` works standalone too, as long as Postgres is up and
`DATABASE_URL` points at it.

## Adding tables / queries / models

DB access is codegen'd by `sqlc`. All SQL lives under `db/`:

- `db/migrations/` — golang-migrate up/down files, embedded into the bot binary.
- `db/queries.sql` — hand-written sqlc queries.
- `db/sqlc.yaml` — codegen config.

Running `make sqlc` (or `cd db && sqlc generate`) produces
`internal/store/queries/{db,models,queries.sql.go}` in package
`queries`. Domain packages (e.g. `internal/apple`) import
`internal/store/queries` directly and use `queries.Queries`,
`queries.UpsertWorkoutParams`, `queries.AppleImport`, etc.

Hand-written `pgx.CopyFrom` is kept for bulk inserts that sqlc can't
express (see `internal/apple/store.go` for the `apple_workout_heart_rate`
and `apple_workout_route` blocks).

### One-time: install sqlc

```sh
make sqlc-install   # go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
```

### Add a new table

1. Create `db/migrations/000N_<name>.up.sql` and matching `.down.sql`
   with the `CREATE TABLE` / `CREATE INDEX` statements. Numbering is
   sequential starting from `0001`.
2. Restart the bot — `store.Migrate` applies the migration on startup
   (files are embedded via `embed.go`). For ad-hoc verification:
   `make tf-up && go run ./cmd/bot`, then
   `docker exec vo2-postgres psql -U vo2 -d vo2 -c "\dt"`.
3. Never edit an already-applied migration; add a new one that alters.

### Add a new query

1. Open `db/queries.sql`. One database, one queries file.
2. Write the query with a sqlc annotation:

   ```sql
   -- name: GetWorkoutsInRange :many
   SELECT id, name, started_at, ended_at, distance_km, avg_hr_bpm
     FROM apple_workouts
    WHERE started_at >= $1 AND started_at < $2
    ORDER BY started_at DESC;
   ```

   Annotation kinds: `:one` (single row), `:many` (slice), `:exec`
   (no rows returned), `:execrows` (affected row count).
3. Run `make sqlc`. This regenerates
   `internal/store/queries/{db,models,queries.sql.go}`.
4. Call from a domain package (e.g. `internal/apple/store.go`):
   `rows, err := s.q.GetWorkoutsInRange(ctx, queries.GetWorkoutsInRangeParams{...})`.

### What sqlc can't do, and what to do instead

- **`COPY FROM`** — use `tx.CopyFrom(ctx, pgx.Identifier{…}, cols, rows)`
  directly on the pgx transaction. See `Store.Persist` for the pattern
  on `apple_workout_heart_rate` and `apple_workout_route`.
- **Dynamic `WHERE` clauses** (optional filters, variable IN-lists) —
  write multiple `:many` variants, or fall back to a hand-written
  `pgx.Query` when the combinatorics get absurd. `GET /apple/data` will
  likely need this.

### Type overrides

`db/sqlc.yaml` overrides `uuid` → `string` (we store HAE UUIDs as strings)
and `timestamptz` → `time.Time` (so callers don't juggle the
`pgtype.Timestamptz.Valid` flag on NOT NULL columns). Nullable
`timestamptz` still surfaces as `pgtype.Timestamptz` in generated
structs; `internal/apple/store.go` has `toTimestamptz` /
`fromAppleImport` adapters for that edge.

## Environment variables

See `.env.example`. The currently-used keys are:

- `DATABASE_URL` — required.
- `APPLE_ARCHIVE_DIR` — base directory for `source=local` imports,
  default `local/apple`.
- `HTTP_ADDR` — default `:8080`.
- `CLAUDE_MODEL` — default `claude-opus-4-7`.
- `STRAVA_CLIENT_ID`, `STRAVA_CLIENT_SECRET`, `STRAVA_REDIRECT_URL`,
  `TELEGRAM_BOT_TOKEN`, `TELEGRAM_ALLOWED_CHAT_IDS`, `ANTHROPIC_API_KEY`
  — loaded but not yet consumed.
