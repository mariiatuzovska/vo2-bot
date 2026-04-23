# vo2-bot Tiltfile
#
# Resource order at `tilt up`:
#   init      - one-time: `make init` (env files + terraform init + sqlc)
#   infra-up  - `make tf-up` (Postgres container via Terraform)
#   postgres  - streams container logs, green when pg_isready
#   bot       - `go run ./cmd/bot`, auto-restarts on source change
#
# Migrations are embedded in the bot binary (see ./embed.go, ./migrations/)
# and run on startup via internal/store.Migrate.
#
# build/Dockerfile is for production deployment, not the dev loop.

# One-time project setup: seeds .env + terraform.tfvars from templates
# (no-op if they already exist), runs terraform init, installs sqlc.
# Runs at `tilt up` and is idempotent — steps are gated by file-target
# rules in the Makefile.
local_resource(
    'init',
    cmd='make init',
    labels=['infra'],
    trigger_mode=TRIGGER_MODE_MANUAL,
    auto_init=True,
)

local_resource(
    'infra-up',
    cmd='make tf-up',
    deps=[
        'infra/main.tf',
        'infra/variables.tf',
        'infra/versions.tf',
        'infra/outputs.tf',
    ],
    resource_deps=['init'],
    labels=['infra'],
)

# Streams Postgres container logs into Tilt; readiness is the container's own
# healthcheck (pg_isready) so Tilt marks it green only when Postgres accepts
# connections.
local_resource(
    'postgres',
    serve_cmd='docker logs -f vo2-postgres',
    readiness_probe=probe(
        exec=exec_action(['sh', '-c',
            'test "$(docker inspect --format \'{{.State.Health.Status}}\' vo2-postgres)" = "healthy"']),
        period_secs=2,
    ),
    resource_deps=['infra-up'],
    labels=['infra'],
)

local_resource(
    'bot',
    serve_cmd='go run ./cmd/bot',
    deps=['cmd', 'internal', 'embed.go', 'db/migrations', 'go.mod', 'go.sum'],
    resource_deps=['postgres'],
    labels=['app'],
)
