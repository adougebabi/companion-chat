package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/config"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/httpapi"
)

func main() {
	settings, err := config.FromEnv(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	repository, err := core.NewPostgresRepository(ctx, settings.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer repository.Close()
	application, err := core.NewApp(
		repository,
		settings.SettingsKey,
		settings.ServiceKey,
		settings.S3Endpoint,
		settings.S3Region,
		settings.S3AccessKey,
		settings.S3SecretKey,
		settings.S3Bucket,
		settings.S3UseSSL,
	)
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{
		Addr:              settings.ListenAddress,
		Handler:           httpapi.NewApp(application, settings.ServiceKey, nil).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       11 * time.Minute,
	}
	go func() {
		log.Printf("Go Core API listening on %s", settings.ListenAddress)
		if serveErr := server.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
			log.Printf("go Core server failed: %v", serveErr)
			cancel()
		}
	}()
	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("go Core shutdown failed: %v", err)
	}
}
