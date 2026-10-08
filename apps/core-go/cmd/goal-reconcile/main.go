package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/config"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
)

func main() {
	owner := flag.String("owner", "", "authorized Owner actor ID")
	instance := flag.String("fluctlight", "", "one explicit Fluctlight ID")
	cursor := flag.String("after", "", "previous next_cursor; restartable ID waterline")
	limit := flag.Int("limit", 20, "batch size 1..100")
	apply := flag.Bool("apply", false, "apply reviewed batch; default is read-only preview")
	digest := flag.String("digest", "", "digest from reviewed preview")
	flag.Parse()
	if *owner == "" || *instance == "" {
		log.Fatal("--owner and --fluctlight are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	repo, err := core.NewPostgresRepository(ctx, config.DatabaseURLFromEnv(os.LookupEnv))
	if err != nil {
		log.Fatal(err)
	}
	defer repo.Close()
	report, err := (&core.App{DB: repo}).ReconcileGoalStock(ctx, *owner, *instance, *cursor, *limit, *apply, *digest)
	if err != nil {
		log.Fatal(err)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(data))
}
