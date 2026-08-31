package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/config"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	coreworkflow "github.com/fluctlight/local-ai-companion/apps/core-go/internal/workflow"
	"go.temporal.io/sdk/client"
)

func main() {
	settings, err := config.FromEnv(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	if settings.TemporalAddr == "" {
		settings.TemporalAddr = "temporal:7233"
	}
	if settings.TemporalNS == "" {
		settings.TemporalNS = "default"
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	repository, err := core.NewPostgresRepository(ctx, settings.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer repository.Close()
	application, err := core.NewApp(repository, settings.SettingsKey, settings.ServiceKey, settings.S3Endpoint, settings.S3Region, settings.S3AccessKey, settings.S3SecretKey, settings.S3Bucket, settings.S3UseSSL)
	if err != nil {
		log.Fatal(err)
	}
	coreworkflow.Configure(application)
	temporalClient, err := client.Dial(client.Options{HostPort: settings.TemporalAddr, Namespace: settings.TemporalNS})
	if err != nil {
		log.Fatal(err)
	}
	defer temporalClient.Close()
	logger := slog.Default()
	_, fatalErrors, err := coreworkflow.StartWorkers(ctx, temporalClient, logger)
	if err != nil {
		log.Fatal(err)
	}
	healthFile := os.Getenv("WORKER_HEALTH_FILE")
	if healthFile == "" {
		healthFile = "/tmp/fluctlight-worker.ready"
	}
	if err := writeWorkerHealth(healthFile); err != nil {
		log.Fatal(err)
	}
	defer os.Remove(healthFile)
	dispatcher := &coreworkflow.Dispatcher{App: application, Client: temporalClient, Started: map[string]struct{}{}}
	dispatchTicker := time.NewTicker(time.Second)
	defer dispatchTicker.Stop()
	healthTicker := time.NewTicker(5 * time.Second)
	defer healthTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case workerErr := <-fatalErrors:
			if workerErr != nil {
				logger.Error("Go Worker stopped due to fatal Temporal error", "error", workerErr)
			}
			return
		case <-healthTicker.C:
			if err := writeWorkerHealth(healthFile); err != nil {
				logger.Error("Go Worker health signal failed", "error", err)
				return
			}
		case <-dispatchTicker.C:
			if _, err := dispatcher.DispatchOnce(ctx, 20); err != nil {
				logger.Warn("Go Worker dispatcher retry", "error", err)
			}
		}
	}
}

func writeWorkerHealth(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(time.Now().UTC().Format(time.RFC3339Nano)+"\n"), 0o644)
}
