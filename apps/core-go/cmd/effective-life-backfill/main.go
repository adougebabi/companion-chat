package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/config"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
)

func main() {
	owner := flag.String("owner", "", "authorized Owner actor ID")
	ids := flag.String("fluctlights", "", "comma-separated Fluctlight IDs")
	all := flag.Bool("all", false, "inspect all instances owned by --owner")
	apply := flag.Bool("apply", false, "initialize missing effective life baselines; default is preview")
	flag.Parse()
	if strings.TrimSpace(*owner) == "" {
		log.Fatal("--owner is required")
	}
	var selected []string
	for _, raw := range strings.Split(*ids, ",") {
		if id := strings.TrimSpace(raw); id != "" {
			selected = append(selected, id)
		}
	}
	if *all == (len(selected) > 0) {
		log.Fatal("specify exactly one of --all or --fluctlights")
	}
	databaseURL := config.DatabaseURLFromEnv(os.LookupEnv)
	if databaseURL == "" {
		log.Fatal("CORE_GO_DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	repository, err := core.NewPostgresRepository(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer repository.Close()
	app := &core.App{DB: repository}
	report, err := app.BackfillEffectiveLife(ctx, *owner, selected, *all, *apply)
	if err != nil {
		log.Fatal(err)
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(encoded))
	for _, item := range report {
		if item.Status == "failed" {
			os.Exit(1)
		}
	}
}
