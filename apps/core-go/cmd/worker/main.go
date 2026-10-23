package main

import (
	"context"
	"encoding/json"
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
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/instant"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/platform"
	coreworkflow "github.com/fluctlight/local-ai-companion/apps/core-go/internal/workflow"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"go.temporal.io/sdk/client"
)

const workerTickOperationTimeout = 15 * time.Second

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo, ReplaceAttr: instant.LogAttribute})))
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
	if err := application.VerifyWorkingPersonasReady(ctx); err != nil {
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
	if ensured, err := application.EnsureConversationSegmentIntents(ctx, 100); err != nil {
		log.Printf("ensure conversation segment intents: %v", err)
	} else if ensured > 0 {
		slog.Default().Info("Go Worker ensured conversation segment intents", "count", ensured)
	}
	if ensured, err := application.EnsureDiaryIntents(ctx); err != nil {
		log.Printf("ensure accepted local-day diary intents: %v", err)
	} else if ensured > 0 {
		slog.Default().Info("Go Worker ensured diary intents", "count", ensured)
	}
	if reconciled, err := application.ReconcileLegacyConversationSummaries(ctx, 100); err != nil {
		log.Printf("reconcile legacy conversation summaries: %v", err)
	} else if reconciled > 0 {
		slog.Default().Info("Go Worker reconciled legacy conversation summaries", "count", reconciled)
	}
	if released, err := application.ReleaseDueWakeUpIntents(ctx, 50); err != nil {
		log.Printf("release due WakeUp intents: %v", err)
	} else if released > 0 {
		slog.Default().Info("Go Worker released due WakeUp intents", "count", released)
	}
	temporalClient, err := client.Dial(client.Options{HostPort: settings.TemporalAddr, Namespace: settings.TemporalNS})
	if err != nil {
		log.Fatal(err)
	}
	defer temporalClient.Close()
	application.SetWorkflowRuntime(coreworkflow.NewTemporalRuntime(temporalClient, settings.TemporalNS, "fluctlight-core-worker"))
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
	application.SetRedisClient(redisClient, consumerID)
	if reconciled, err := application.ReconcileWakeUpIntents(ctx); err != nil {
		log.Printf("schedule Redis wake-up hints: %v", err)
	} else if reconciled > 0 {
		slog.Default().Info("Go Worker startup reconciled WakeUp intents", "count", reconciled)
	}
	publisher := platform.NewOutboxPublisher(application.DB.Pool(), redisClient, consumerID)
	consumers := make([]*platform.EventConsumer, 0, len(platform.DurableConsumerGroups))
	for _, group := range platform.DurableConsumerGroups {
		consumer := platform.NewEventConsumer(application.DB.Pool(), redisClient, group, consumerID)
		if group == "api-notifications" {
			consumer.Apply = func(ctx context.Context, tx pgx.Tx, event platform.EventEnvelope) error {
				var payload map[string]any
				if err := json.Unmarshal(event.Payload, &payload); err != nil {
					return err
				}
				return application.ApplyNotificationSourceTx(ctx, tx, core.NotificationSource{EventID: event.EventID, Kind: event.EventType, FluctlightID: event.FluctlightID, SourceID: event.AggregateID, OccurredAt: event.OccurredAt, Payload: payload})
			}
		}
		if err := consumer.EnsureGroup(ctx); err != nil {
			log.Fatal(err)
		}
		consumers = append(consumers, consumer)
	}
	logger := slog.Default()
	triggerListener := core.NewRedisTriggerListener(application, redisClient)
	go func() {
		if err := triggerListener.Run(ctx); err != nil && ctx.Err() == nil {
			logger.Warn("Redis trigger listener stopped", "error", err)
		}
	}()
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
	stopDispatch := startWorkerLoop(ctx, time.Second, func(ctx context.Context) {
		if _, err := application.ExpireDueConversationTurns(ctx, 50); err != nil {
			logger.Warn("Go Worker reply deadline recovery retry", "error", err)
		}
		if err := runWorkerTickOperation(ctx, func(operationCtx context.Context) error {
			_, err := dispatcher.DispatchOnce(operationCtx, 20)
			return err
		}); err != nil {
			logger.Warn("Go Worker dispatcher retry", "error", err)
		}
		if err := runWorkerTickOperation(ctx, func(operationCtx context.Context) error {
			_, err := dispatcher.DispatchReplyContinuations(operationCtx, 20)
			return err
		}); err != nil {
			logger.Warn("Go Worker reply continuation retry", "error", err)
		}
		if err := runWorkerTickOperation(ctx, func(operationCtx context.Context) error {
			_, err := dispatcher.ReconcileOnce(operationCtx, 50)
			return err
		}); err != nil {
			logger.Warn("Go Worker intent reconciliation retry", "error", err)
		}
	})
	defer stopDispatch()
	stopDelivery := startWorkerLoop(ctx, time.Second, func(ctx context.Context) {
		if err := runWorkerTickOperation(ctx, func(operationCtx context.Context) error {
			_, err := application.ReleaseDueWakeUpIntents(operationCtx, 50)
			return err
		}); err != nil {
			logger.Warn("Go Worker due WakeUp release retry", "error", err)
		}
		if err := runWorkerTickOperation(ctx, func(operationCtx context.Context) error {
			_, err := application.Provider.ReconcileRedisQueue(operationCtx, "generic_llm")
			return err
		}); err != nil {
			logger.Warn("Go Worker Redis LLM queue reconciliation retry", "binding", "generic_llm", "error", err)
		}
		if err := runWorkerTickOperation(ctx, func(operationCtx context.Context) error {
			_, err := application.Provider.ReconcileRedisQueue(operationCtx, "embedding")
			return err
		}); err != nil {
			logger.Warn("Go Worker Redis LLM queue reconciliation retry", "binding", "embedding", "error", err)
		}
		if err := runWorkerTickOperation(ctx, func(operationCtx context.Context) error {
			_, err := publisher.PublishOnce(operationCtx, 50)
			return err
		}); err != nil {
			logger.Warn("Go Worker outbox publisher retry", "error", err)
		}
		for _, consumer := range consumers {
			if err := runWorkerTickOperation(ctx, func(operationCtx context.Context) error {
				_, err := consumer.ConsumeOnce(operationCtx, 25)
				return err
			}); err != nil {
				logger.Warn("Go Worker event consumer retry", "group", consumer.Group, "error", err)
			}
		}

	})
	defer stopDelivery()
	stopRetention := startWorkerLoop(ctx, 15*time.Minute, func(ctx context.Context) {
		if err := application.MaintainKevDiagnostics(ctx); err != nil {
			logger.Warn("Kev diagnostics maintenance failed")
		}
		if recovered, err := application.RecoverStaleModelRuns(ctx, 15*time.Minute); err != nil {
			logger.Warn("Go Worker stale model-run recovery retry", "error", err)
		} else if recovered > 0 {
			logger.Info("Go Worker recovered stale model runs", "count", recovered)
		}
		if _, err := application.PruneDiagnostics(ctx, 30*24*time.Hour, 10000); err != nil {
			logger.Warn("Go Worker diagnostics retention retry", "error", err)
		}
		if audited, err := application.AuditWakeUpClocks(ctx, 50); err != nil {
			logger.Warn("Go Worker WakeUp clock audit retry", "error", err)
		} else if audited > 0 {
			logger.Warn("Go Worker found unhealthy WakeUp clocks", "count", audited)
		}
	})
	defer stopRetention()
	stopSupervision := startWorkerLoop(ctx, 30*time.Minute, func(ctx context.Context) {
		if reconciled, err := application.ReconcileWakeUpIntents(ctx); err != nil {
			logger.Warn("Go Worker periodic 30m WakeUp supervision retry", "error", err)
		} else if reconciled > 0 {
			logger.Info("Go Worker periodic 30m WakeUp supervision reconciled intents", "count", reconciled)
		}
	})
	defer stopSupervision()
	stopRepair := startWorkerLoop(ctx, time.Minute, func(ctx context.Context) {
		if _, err := application.RepairWardrobePreparations(ctx, 20); err != nil {
			logger.Warn("Go Worker wardrobe recovery retry", "error", err)
		}
		if reconciled, err := application.ReconcileGoalAttempts(ctx, 50); err != nil {
			logger.Warn("Go Worker Goal attempt reconciliation retry", "error", err)
		} else if reconciled > 0 {
			logger.Info("Go Worker reconciled Goal attempts", "count", reconciled)
		}
		if reconciled, err := application.ReconcileLegacyConversationSummaries(ctx, 100); err != nil {
			logger.Warn("Go Worker legacy conversation summary reconciliation retry", "error", err)
		} else if reconciled > 0 {
			logger.Info("Go Worker reconciled legacy conversation summaries", "count", reconciled)
		}
		if ensured, err := application.EnsureConversationSegmentIntents(ctx, 100); err != nil {
			logger.Warn("Go Worker conversation segment repair retry", "error", err)
		} else if ensured > 0 {
			logger.Info("Go Worker repaired conversation segment intents", "count", ensured)
		}
		if _, err := application.RepairWorldEvents(ctx, 20); err != nil {
			logger.Error("World event recovery failed", "error", err)
		}
		if _, err := application.RepairWorldNPCPresence(ctx, 20); err != nil {
			logger.Error("NPC presence recovery failed", "error", err)
		}
		if _, err := application.RepairNPCOpportunities(ctx, 50); err != nil {
			logger.Error("NPC opportunity recovery failed", "error", err)
		}
		if _, err := application.RepairOwnerPromiseDeadlines(ctx, 20); err != nil {
			logger.Error("promise deadline recovery failed", "error", err)
		}
		if _, err := application.RepairGoalEvaluationCadence(ctx, 20); err != nil {
			logger.Error("goal evaluation cadence recovery failed", "error", err)
		}
		if _, err := application.RepairGoalPlanning(ctx, 20); err != nil {
			logger.Error("goal planner recovery failed", "error", err)
		}
		if repaired, err := application.RepairWakeUpClocks(ctx); err != nil {
			logger.Warn("Go Worker WakeUp clock repair retry", "error", err)
		} else if repaired > 0 {
			logger.Info("Go Worker repaired WakeUp clocks", "count", repaired)
		}
	})
	defer stopRepair()
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
		}
	}
}

// Independent, internally serialized infrastructure lanes. A slow Redis
// consumer or maintenance pass must not block conversation dispatch/readiness.
func startWorkerLoop(parent context.Context, interval time.Duration, work func(context.Context)) func() {
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if ctx.Err() != nil {
					return
				}
				work(ctx)
			}
		}
	}()
	return func() { cancel(); <-done }
}

func runWorkerTickOperation(parent context.Context, operation func(context.Context) error) error {
	operationCtx, cancel := context.WithTimeout(parent, workerTickOperationTimeout)
	defer cancel()
	return operation(operationCtx)
}

func writeWorkerHealth(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(time.Now().UTC().Format(time.RFC3339Nano)+"\n"), 0o644)
}
