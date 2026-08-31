package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// setup-token is a one-shot, operator-only bootstrap command. It prints the
// plaintext token exactly once; the database stores only its SHA-256 digest.
func main() {
	minutes := flag.Int("expires-minutes", 30, "token lifetime")
	flag.Parse()
	if *minutes < 1 || *minutes > 1440 {
		log.Fatal("expires-minutes must be between 1 and 1440")
	}
	databaseURL := config.DatabaseURLFromEnv(os.LookupEnv)
	if databaseURL == "" {
		log.Fatal("CORE_GO_DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatal(err)
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		log.Fatal(err)
	}
	token := "setup_" + hex.EncodeToString(raw)
	digest := sha256.Sum256([]byte(token))
	id := "setup_" + hex.EncodeToString(digest[:])[:32]
	if _, err := pool.Exec(ctx, `INSERT INTO public.owner_setup_tokens(id,token_hash,expires_at) VALUES($1,$2,$3)`, id, hex.EncodeToString(digest[:]), time.Now().UTC().Add(time.Duration(*minutes)*time.Minute)); err != nil {
		log.Fatal(err)
	}
	fmt.Println(token)
}
