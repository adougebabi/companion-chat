package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/config"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"log"
	"os"
	"time"
)

func main() {
	owner := flag.String("owner", "", "authorized Owner ID")
	instance := flag.String("fluctlight", "", "explicit instance ID")
	after := flag.String("after", "", "batch cursor")
	limit := flag.Int("limit", 20, "batch size 1..100")
	apply := flag.Bool("apply", false, "apply source links; default is read-only preview")
	flag.Parse()
	if *owner == "" || *instance == "" {
		log.Fatal("--owner and --fluctlight required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	repo, err := core.NewPostgresRepository(ctx, config.DatabaseURLFromEnv(os.LookupEnv))
	if err != nil {
		log.Fatal(err)
	}
	defer repo.Close()
	report, err := (&core.App{DB: repo}).MigrateGoalPlannerSources(ctx, *owner, *instance, *after, *limit, *apply)
	if err != nil {
		log.Fatal(err)
	}
	data, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(data))
}
