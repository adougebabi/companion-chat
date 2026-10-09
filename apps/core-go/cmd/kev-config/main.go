// kev-config is an explicit operator command. It uses the same Owner settings
// service as the UI; migrations never overwrite existing settings.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/config"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
)

func main() {
	endpoint := flag.String("endpoint", "", "System One URL reachable by Core and Worker (required)")
	disable := flag.Bool("disable", false, "Disable Kev; preserve original workflows")
	flag.Parse()
	if *endpoint == "" {
		log.Fatal("--endpoint is required; do not use another process's loopback address")
	}
	c := decision.DefaultConfig()
	c.Enabled = !*disable
	c.Endpoint = *endpoint
	if err := c.Validate(); err != nil {
		log.Fatal(err)
	}
	settings, err := config.FromEnv(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repository, err := core.NewPostgresRepository(ctx, settings.DatabaseURL)
	if err != nil {
		log.Fatal("database is unavailable; check schema and connection configuration")
	}
	defer repository.Close()
	application, err := core.NewApp(repository, settings.SettingsKey, settings.ServiceKey, settings.S3Endpoint, settings.S3Region, settings.S3AccessKey, settings.S3SecretKey, settings.S3Bucket, settings.S3UseSSL)
	if err != nil {
		log.Fatal("application configuration invalid")
	}
	var owner string
	if err := repository.Pool().QueryRow(ctx, `SELECT human_actor_id FROM public.owner_accounts LIMIT 1`).Scan(&owner); err != nil {
		log.Fatal("Owner account is required")
	}
	if _, err := application.UpdateSettings(ctx, owner, map[string]any{"values": map[string]any{"kev": c}}); err != nil {
		log.Fatal("Kev settings update failed")
	}
	fmt.Printf("Kev enabled=%t; seven points configured; strategy=%s\n", c.Enabled, c.Strategy)
}
