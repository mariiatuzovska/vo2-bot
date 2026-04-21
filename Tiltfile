# vo2-bot Tiltfile
#
# Dev loop:
#   1. `make tf-up`     - Postgres container up via Terraform
#   2. `go run ./cmd/bot` - bot runs on host, auto-restarts on source change
#
# Migrations (`migrate -path migrations ... up`) will be added as a resource
# once the first schema lands.
#
# build/Dockerfile is for production deployment, not the dev loop.

local_resource(
    'infra-up',
    cmd='make tf-up',
    deps=[
        'infra/main.tf',
        'infra/variables.tf',
        'infra/versions.tf',
        'infra/outputs.tf',
    ],
    labels=['infra'],
)

local_resource(
    'bot',
    serve_cmd='go run ./cmd/bot',
    deps=['cmd', 'internal', 'go.mod', 'go.sum'],
    resource_deps=['infra-up'],
    labels=['app'],
)
