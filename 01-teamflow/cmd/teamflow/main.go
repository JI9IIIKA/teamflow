package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"teamflow/internal/platform"
	"teamflow/migrations"
	"teamflow/projects/teamflow"
)

func main() {
	platform.Serve("teamflow", teamflow.Register, func(ctx context.Context, pool *pgxpool.Pool) error { return migrations.Apply(ctx, pool, "teamflow") })
}
