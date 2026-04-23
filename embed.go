package vo2bot

import "embed"

//go:embed db/migrations/*.sql
var MigrationsFS embed.FS
