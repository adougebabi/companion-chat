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
	owner := flag.String("owner", "", "authorized Owner Actor ID")
	manifest := flag.String("manifest", "", "JSON array of exact IDs, Fluctlight scope, expected revisions and source qualifications")
	reason := flag.String("reason", "", "explicit audit reason")
	expectedDigest := flag.String("expected-plan-digest", "", "digest printed by the reviewed dry-run; required for first apply")
	apply := flag.Bool("apply", false, "apply this reviewed plan; default is dry-run")
	rollback := flag.String("rollback-batch", "", "rollback an unchanged audited batch")
	flag.Parse()
	if *owner == "" || *reason == "" || (*manifest == "") == (*rollback == "") {
		log.Fatal("--owner and --reason plus exactly one --manifest or --rollback-batch are required")
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
	var result any
	if *rollback != "" {
		result, err = app.RollbackHistoryPollutionRepair(ctx, *owner, *rollback, *reason)
	} else {
		data, readErr := os.ReadFile(*manifest)
		if readErr != nil {
			log.Fatal(readErr)
		}
		var entries []core.HistoryRepairEntry
		if decodeErr := json.Unmarshal(data, &entries); decodeErr != nil {
			log.Fatal(decodeErr)
		}
		if *apply {
			var replay map[string]any
			replay, err = app.ReplayHistoryPollutionRepair(ctx, *owner, entries, *reason)
			if err != nil {
				log.Fatal(err)
			}
			if replay != nil {
				result = replay
			}
		}
		if result == nil {
			plan, planErr := app.PlanHistoryPollutionRepair(ctx, *owner, entries, *reason)
			if planErr != nil {
				log.Fatal(planErr)
			}
			result = plan
			if *apply {
				if *expectedDigest == "" || *expectedDigest != plan.Digest {
					log.Fatal("first apply requires --expected-plan-digest matching the current dry-run")
				}
				result, err = app.ApplyHistoryPollutionRepair(ctx, plan)
			}
		}
	}
	if err != nil {
		log.Fatal(err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(data))
}
