package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/config"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/platform"
	coreworkflow "github.com/fluctlight/local-ai-companion/apps/core-go/internal/workflow"
	"github.com/redis/go-redis/v9"
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
	if _, err := application.RecoverStaleModelRuns(ctx, 15*time.Minute); err != nil {
		log.Printf("recover stale model runs: %v", err)
	}
	coreworkflow.Configure(application)
	if ensured, err := application.EnsureWakeUpIntents(ctx); err != nil {
		log.Fatalf("ensure wake-up intents: %v", err)
	} else if ensured > 0 {
		slog.Default().Info("Go Worker ensured wake-up intents", "count", ensured)
	}
	temporalClient, err := client.Dial(client.Options{HostPort: settings.TemporalAddr, Namespace: settings.TemporalNS})
	if err != nil {
		log.Fatal(err)
	}
	defer temporalClient.Close()
	redisOptions, err := redis.ParseURL(settings.RedisURL)
	if err != nil {
		log.Fatal(err)
	}
	redisClient := redis.NewClient(redisOptions)
	defer redisClient.Close()
	redisCtx, redisCancel := context.WithTimeout(ctx, 5*time.Second)
	if err := redisClient.Ping(redisCtx).Err(); err != nil {
		redisCancel()
		log.Fatal(err)
	}
	redisCancel()
	hostname, _ := os.Hostname()
	consumerID := fmt.Sprintf("go-worker-%s-%d", hostname, os.Getpid())
	publisher := platform.NewOutboxPublisher(application.DB.Pool(), redisClient, consumerID)
	consumers := make([]*platform.EventConsumer, 0, len(platform.DurableConsumerGroups))
	for _, group := range platform.DurableConsumerGroups {
		consumer := platform.NewEventConsumer(application.DB.Pool(), redisClient, group, consumerID)
		if err := consumer.EnsureGroup(ctx); err != nil {
			log.Fatal(err)
		}
		consumers = append(consumers, consumer)
	}
	logger := slog.Default()
	_, fatalErrors, err := coreworkflow.StartWorkers(ctx, temporalClient, logger)
	if err != nil {
		log.Fatal(err)
	}
	deploymentHandle := temporalClient.WorkerDeploymentClient().GetHandle(coreworkflow.WorkerDeploymentName)
	buildID := coreworkflow.WorkerDeploymentBuildID()
	if err := coreworkflow.EnsureWorkerDeploymentCurrentVersion(ctx, deploymentHandle, buildID); err != nil {
		log.Fatalf("configure Temporal Worker Deployment: %v", err)
	}
	logger.Info("Go Worker Deployment current version configured", "deployment", coreworkflow.WorkerDeploymentName, "build_id", buildID)
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
	retentionTicker := time.NewTicker(15 * time.Minute)
	defer retentionTicker.Stop()
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
		case <-retentionTicker.C:
			if _, err := application.PruneDiagnostics(ctx, 30*24*time.Hour, 10000); err != nil {
				logger.Warn("Go Worker diagnostics retention retry", "error", err)
			}
		case <-dispatchTicker.C:
			if _, err := publisher.PublishOnce(ctx, 50); err != nil {
				logger.Warn("Go Worker outbox publisher retry", "error", err)
			}
			for _, consumer := range consumers {
				if _, err := consumer.ConsumeOnce(ctx, 25); err != nil {
					logger.Warn("Go Worker event consumer retry", "group", consumer.Group, "error", err)
				}
			}
			if _, err := dispatcher.DispatchOnce(ctx, 20); err != nil {
				logger.Warn("Go Worker dispatcher retry", "error", err)
			}
			if _, err := dispatcher.ReconcileOnce(ctx, 50); err != nil {
				logger.Warn("Go Worker intent reconciliation retry", "error", err)
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
