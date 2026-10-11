// kev-config is an explicit operator command. It uses the same Owner settings
// service as the UI; migrations never overwrite existing settings.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/config"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
)

func main() {
	endpoint := flag.String("endpoint", "", "System One URL reachable by Core and Worker (required)")
	mediaOnly := flag.Bool("media-only", false, "Change media points while preserving effective seven-point behavior")
	backup := flag.String("backup", "", "Private settings backup path (required with --media-only)")
	disable := flag.Bool("disable", false, "Disable Kev; preserve original workflows")
	flag.Parse()
	if *endpoint == "" && !*mediaOnly {
		log.Fatal("--endpoint is required; do not use another process's loopback address")
	}
	c := decision.DefaultConfig()
	c.Enabled = !*disable
	if *endpoint != "" {
		c.Endpoint = *endpoint
	}
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
	if *mediaOnly {
		if *backup == "" {
			log.Fatal("--backup is required before changing local media settings")
		}
		saved, err := application.ReadSettings(ctx, owner)
		if err != nil {
			log.Fatal("settings read failed")
		}
		raw, err := json.MarshalIndent(saved, "", "  ")
		if err != nil {
			log.Fatal("settings backup serialization failed")
		}
		if err := os.MkdirAll(filepath.Dir(*backup), 0700); err != nil {
			log.Fatal("backup directory unavailable")
		}
		f, err := os.OpenFile(*backup, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			log.Fatal("backup must be a new private file")
		}
		if _, err = f.Write(raw); err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			log.Fatal("settings backup failed; no update applied")
		}
		values, _ := saved["values"].(map[string]any)
		if current, ok := values["kev"]; ok {
			raw, _ := json.Marshal(current)
			if err := json.Unmarshal(raw, &c); err != nil {
				log.Fatal("existing Kev settings invalid")
			}
		}
		if _, exists := values["kev"]; !exists && *endpoint == "" {
			log.Fatal("--endpoint is required when no prior Kev settings exist")
		}
		if *endpoint != "" {
			c.Endpoint = *endpoint
		}
		if !*disable {
			if !c.Enabled {
				for _, point := range decision.Points[:7] {
					c.Points[point] = false
				}
			}
			c.Enabled = true
		}
		c.MediaEnabled = !*disable
		for _, point := range []string{"media.subject_route", "media.capture_plan", "media.view_plan"} {
			c.Points[point] = !*disable
		}
		if err := c.Validate(); err != nil {
			log.Fatal("merged media configuration invalid; no update applied")
		}
	}
	if _, err := application.UpdateSettings(ctx, owner, map[string]any{"values": map[string]any{"kev": c}}); err != nil {
		log.Fatal("Kev settings update failed")
	}
	fmt.Printf("Kev enabled=%t; ten points configured; strategy=%s\n", c.Enabled, c.Strategy)
}
