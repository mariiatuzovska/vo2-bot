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
| Strava      | `net/http` — OAuth2 + activity sync (`internal/strava`) |
| Telegram    | `go-telegram-bot-api/telegram-bot-api/v5` — long-poll bot, allowlisted chats |
| Claude      | planned (`anthropic-sdk-go`, model `claude-opus-4-7`)  |

## Repo layout

```
vo2-bot/
  cmd/bot/main.go            entrypoint: config → migrate → open pool → wait
  embed.go                   embeds db/migrations/*.sql into the binary
  db/                        ALL SQL lives here:
    migrations/
      0001_apple.up.sql      Apple Health tables (workouts, HR bins, route, metrics, imports)
      0002_strava.up.sql     Strava tables (athletes, tokens, links, activities, sync, rate limit)
    queries.sql              hand-written sqlc queries (Apple + Strava)
    sqlc.yaml                sqlc codegen config
  internal/
    config/                  viper-based env loader
    store/                   DB wrapper package:
      store.go               pgx pool (Open, Close)
      migrate.go             golang-migrate runner
      queries/               GENERATED sqlc code (package: queries)
    apple/                   Apple Health ingest: parse HAE archive,
                             orchestrate store calls, HTTP handler.
    strava/                  Strava OAuth2 + activity collector:
      client.go              Client struct, do() with lazy token refresh + rate-limit tracking
      oauth.go               AuthURL(), HandleCallback() — OAuth2 flow
      athlete.go             GetAthlete(), fetchAthleteWithToken(), athleteParams()
      activities.go          apiActivity struct, listPage() — paginated activity fetch
      sync.go                Sync() — advisory lock, pagination loop, cursor update
    httpx/                   shared HTTP helpers (Handle wrapper, WriteJSON)
    errs/                    typed HTTP errors (NewBadRequest, NewNotFound, …)
    telegram/                long-poll bot, allowlist, command dispatch:
      bot.go                 New(), Run() (long poll), dispatch(), allowlist
      handlers.go            /start, /help, /strava, /apple handlers
    claude/                  package stub
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
- **Strava schema.** `migrations/0002_strava.up.sql` creates 6 tables —
  see [Strava: DB schema](#strava-db-schema) below.
- **Strava collector.** `internal/strava` implements the full OAuth2 flow
  and on-demand activity sync — see [Strava: internals](#strava-internals) below.
- **Telegram bot.** `internal/telegram` runs a long-poll bot gated by an
  allowlist of chat IDs. Commands: `/start` and `/help` print usage,
  `/strava` triggers Strava sync for the calling chat, `/apple` triggers a
  local Apple Health import. See [Telegram bot](#telegram-bot) below.
- **Claude:** package stub.

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

## Strava: DB schema

`db/migrations/0002_strava.up.sql` adds six tables:

| Table | Purpose |
| ----- | ------- |
| `strava_athletes` | Athlete profile snapshot (name, city, weight, FTP, avatar URL). Upserted on every OAuth callback and on demand. |
| `strava_tokens` | OAuth2 access + refresh tokens per athlete, with `expires_at`. One row per athlete; the access token is short-lived, the refresh token is long-lived. |
| `telegram_strava_links` | Maps a Telegram `chat_id` to a Strava `athlete_id`. One-to-one for v1. Re-logging from a different chat upserts the mapping cleanly. |
| `strava_activities` | Activity summary fields from `GET /athlete/activities`: sport type, timing, distance, elevation, speed, HR, power, cadence, suffer score. The full raw JSON is kept in a `payload JSONB` column for debugging and future backfill. |
| `strava_sync_state` | One row per athlete holding the `last_activity_at` cursor for incremental sync and `last_synced_at` for observability. |
| `strava_rate_limit` | Single row (enforced by `CHECK (id = 1)`) that mirrors the `X-RateLimit-Limit` / `X-RateLimit-Usage` response headers. Updated after every Strava API call; pre-seeded with Strava defaults (100 / 1 000). |

All writes use `INSERT … ON CONFLICT … DO UPDATE` (upserts on natural keys),
so every operation is safe to retry.

## Strava: internals

### OAuth flow

1. On bot startup, `cmd/bot/main.go` calls `client.AuthURL(chatID)` for the first
   ID in `TELEGRAM_ALLOWED_CHAT_IDS` and logs the URL. The operator opens it once
   from localhost to link their Strava account. The URL embeds an HMAC-SHA256-signed
   state parameter encoding `chatID` and a 10-minute expiry:
   ```
   state = "{chatID}:{expiry_unix}" + "." + HMAC-SHA256(payload, clientSecret)
   ```
   Using `STRAVA_CLIENT_SECRET` as the HMAC key is standard practice and
   avoids an extra env var.

2. After the athlete approves on Strava, `client.HandleCallback(ctx, state, code)`
   is called by the HTTP callback handler:
   - Verifies HMAC and TTL; rejects tampered or expired states.
   - POSTs to `https://www.strava.com/oauth/token` to exchange the code for
     tokens.
   - Fetches the full athlete profile via `GET /api/v3/athlete` using the new
     access token (the exchange response omits weight, FTP, and other fields).
   - In **one transaction**: upserts `strava_athletes`, upserts `strava_tokens`,
     upserts `telegram_strava_links`, seeds `strava_sync_state` with an empty
     cursor.
   - Returns `chatID` so the caller can reply "Linked ✓" to the right chat.

### Token lifecycle

Before every Strava API call, `client.do()` checks token expiry with a 30-second
buffer. If the token has expired, `refresh()` POSTs to `/oauth/token` with
`grant_type=refresh_token`, persists the new token pair, and returns the fresh
access token transparently to the caller. On `HTTP 400 invalid_grant`, the
tokens row is deleted and `ErrTokenRevoked` is returned so the operator can
restart the bot and re-link via the printed Strava auth URL.

### Activity sync (`Sync`)

`client.Sync(ctx, chatID)` is the implementation behind the `/strava`
Telegram command:

1. Resolves `strava_athlete_id` from `telegram_strava_links` via `chatID`.
   Returns an error if not linked.
2. Acquires a **dedicated pgx connection** from the pool and runs
   `pg_try_advisory_lock(athleteID)`. If another goroutine (on any pod) already
   holds the lock for this athlete, returns "already syncing" immediately. The
   lock is released via `pg_advisory_unlock` in a deferred call before the
   connection is returned to the pool.
3. Reads `strava_sync_state.last_activity_at` for the incremental cursor
   (`after` parameter). First-ever sync uses `after=0` → full history walk.
4. Pages through `GET /api/v3/athlete/activities?per_page=200&after=<cursor>`
   until an empty page is returned, upsertng each activity.
5. Updates `strava_sync_state`: sets `last_synced_at = now()`, advances
   `last_activity_at` to the latest `start_date` seen. If no new activities
   were found, the cursor is preserved unchanged.
6. Queries `COUNT(*)` for the athlete's total stored activities.
7. Returns `SyncResult{Added, Total, Latest}`.

### Rate limiting

`trackRateLimit()` is called after every `do()` response. It parses the
`X-RateLimit-Limit` and `X-RateLimit-Usage` headers (format: `"15min,daily"`)
and writes updated counters to `strava_rate_limit`. The handler for
`/strava` can read this row before calling `Sync` and surface a user-friendly
message ("Strava rate limited — try again at HH:MM") instead of hitting a 429.
`ErrRateLimited` is also returned by `do()` on `HTTP 429` for synchronous
handling.

### Horizontal-scale safety

| Concern | How it's handled |
| ------- | ---------------- |
| Concurrent `/strava` for the same athlete | `pg_try_advisory_lock` — at most one winner per `athlete_id` across all pods |
| Pod restart mid-sync | Advisory lock is session-level; it's automatically released when the connection closes, so the next `/strava` after a crash will succeed |
| Multiple pods, different athletes | Each athlete has an independent lock key; no global bottleneck |
| Token storage | Tokens live in DB — any pod can refresh and serve any athlete |
| All writes idempotent | `INSERT … ON CONFLICT … DO UPDATE` throughout — safe to retry |

## Telegram bot

`internal/telegram` runs a long-poll bot using
`go-telegram-bot-api/telegram-bot-api/v5`. `cmd/bot/main.go` constructs the
bot in `registerTelegram` and starts the poll loop in a goroutine; the
process exits if `TELEGRAM_BOT_TOKEN` is missing.

### Commands

| Command       | Behaviour |
| ------------- | --------- |
| `/start`, `/help` | Prints usage. |
| `/strava`     | Calls `strava.Client.Sync(chatID)` and replies with `Pulled N new activities (total: M)` plus the latest activity's sport and start date. |
| `/apple`      | Calls `apple.Service.Import({Source: "local"})` (newest `HealthAutoExport_*.zip` in `APPLE_ARCHIVE_DIR`) and replies with workout/metric counts and the imported date range. |
| anything else | Replies "Unknown command. Use /help to see available commands." |

### Access control

There is **no DB-backed user table**. Access is gated by the
comma-separated `TELEGRAM_ALLOWED_CHAT_IDS` env var, parsed once at
startup into an in-memory `map[int64]bool`. To check the current size,
read the env var (e.g. `printenv TELEGRAM_ALLOWED_CHAT_IDS`) or look for
the startup log line `telegram: allowed chat IDs: [...]`. If the var is
empty, the bot logs `accepting all chats (dev mode)` and lets every chat
through — only use this locally.

### Strava linking

`registerTelegram` takes the first ID from `TELEGRAM_ALLOWED_CHAT_IDS`
and logs `strava: connect at <AuthURL>` on startup. The operator opens
that URL once from localhost to link the chat to a Strava account; the
HTTP callback at `STRAVA_REDIRECT_URL` finishes the OAuth handshake and
seeds `telegram_strava_links`. After that, `/strava` works for the linked
chat.

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
`queries`. Domain packages (e.g. `internal/apple`, `internal/strava`) import
`internal/store/queries` directly.

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
4. Call from a domain package:
   `rows, err := q.GetWorkoutsInRange(ctx, queries.GetWorkoutsInRangeParams{...})`.

### What sqlc can't do, and what to do instead

- **`COPY FROM`** — use `tx.CopyFrom(ctx, pgx.Identifier{…}, cols, rows)`
  directly on the pgx transaction. See `Store.Persist` for the pattern
  on `apple_workout_heart_rate` and `apple_workout_route`.
- **Dynamic `WHERE` clauses** (optional filters, variable IN-lists) —
  write multiple `:many` variants, or fall back to a hand-written
  `pgx.Query` when the combinatorics get absurd.
- **Advisory locks** — execute raw SQL on a dedicated connection acquired via
  `pool.Acquire(ctx)`. See `internal/strava/sync.go` for the pattern.

### Type overrides

`db/sqlc.yaml` overrides `uuid` → `string` (we store HAE UUIDs as strings)
and `timestamptz` → `time.Time` (so callers don't juggle the
`pgtype.Timestamptz.Valid` flag on NOT NULL columns). Nullable
`timestamptz` still surfaces as `pgtype.Timestamptz` in generated
structs; `internal/apple/store.go` has `toTimestamptz` /
`fromAppleImport` adapters for that edge. The same pattern applies in
`internal/strava/sync.go` for `StravaSyncState.LastActivityAt`.

## Environment variables

See `.env.example`. Currently-used keys:

- `DATABASE_URL` — required.
- `APPLE_ARCHIVE_DIR` — base directory for `source=local` imports, default `local/apple`.
- `HTTP_ADDR` — default `:8080`.
- `CLAUDE_MODEL` — default `claude-opus-4-7`.
- `STRAVA_CLIENT_ID`, `STRAVA_CLIENT_SECRET` — required for Strava OAuth2.
  `STRAVA_CLIENT_SECRET` also serves as the HMAC key for signing OAuth state.
- `STRAVA_REDIRECT_URL` — the callback URL registered in your Strava app
  (e.g. `http://localhost:8080/strava/callback`). Must match exactly.
- `TELEGRAM_BOT_TOKEN` — required; the bot exits if unset.
- `TELEGRAM_ALLOWED_CHAT_IDS` — comma-separated chat IDs allowed to
  invoke commands. Empty = accept all chats (dev mode). The first ID also
  receives the startup Strava OAuth URL.
- `ANTHROPIC_API_KEY` — loaded but not yet consumed (Claude package is a stub).
