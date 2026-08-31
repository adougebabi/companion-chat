package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/config"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	databaseURL := config.DatabaseURLFromEnv(os.LookupEnv)
	if databaseURL == "" {
		log.Fatal("CORE_GO_DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatal(err)
	}
	if err := migrations.New(pool).Apply(ctx); err != nil {
		log.Fatal(err)
	}
	log.Printf("Go Core schema migration reached %s", migrations.Head)
}
