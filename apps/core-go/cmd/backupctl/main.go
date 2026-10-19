// backupctl is an operator composition root. It never initializes Agents,
// Providers, workers or schedules, and exposes no HTTP command execution.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/config"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "backup_operator_failed: operation did not complete; inspect the private operator journal and job stage")
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) < 2 {
		return errors.New("command required")
	}
	command := os.Args[1]
	flags := flag.NewFlagSet("backupctl", flag.ContinueOnError)
	job := flags.String("job", "", "Owner-created maintenance job ID")
	if err := flags.Parse(os.Args[2:]); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()
	database := config.DatabaseURLFromEnv(os.LookupEnv)
	if database == "" {
		return errors.New("database required")
	}
	pool, err := pgxpool.New(ctx, database)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return err
	}
	switch command {
	case "claim", "record", "audit-restore":
		var body struct {
			Source  string         `json:"source_backup_id"`
			Target  string         `json:"target_name"`
			Attempt int            `json:"attempt"`
			Token   string         `json:"token"`
			Stage   string         `json:"stage"`
			Status  string         `json:"status"`
			Result  map[string]any `json:"result"`
			Private map[string]any `json:"private"`
		}
		if err := json.NewDecoder(io.LimitReader(os.Stdin, 1<<20)).Decode(&body); err != nil {
			return err
		}
		if command == "claim" {
			result, err := core.ClaimBackupOperator(ctx, pool, *job, body.Token)
			if err != nil {
				return err
			}
			return json.NewEncoder(os.Stdout).Encode(result)
		}
		if command == "audit-restore" {
			if os.Getenv("FLUCTLIGHT_ISOLATED_RESTORE") != "true" {
				return errors.New("isolated operator environment required")
			}
			if err := core.RecordIsolatedBackupRecovery(ctx, pool, *job, body.Source, body.Target, body.Result, body.Private); err != nil {
				return err
			}
			return json.NewEncoder(os.Stdout).Encode(map[string]any{"recorded": true, "models_started": false})
		}
		if err := core.RecordBackupOperator(ctx, pool, *job, body.Token, body.Stage, body.Status, body.Attempt, body.Result, body.Private); err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"recorded": true})
	case "probe":
		return probe(ctx, pool, database)
	case "upgrade":
		if err := migrations.New(pool).Apply(ctx); err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"migration_head": migrations.Head, "models_started": false})
	default:
		return errors.New("unknown command")
	}
}
func probe(ctx context.Context, pool *pgxpool.Pool, database string) error {
	if err := core.VerifyBackupSettingsKey(ctx, pool, os.Getenv("FLUCTLIGHT_SETTINGS_KEY")); err != nil {
		return err
	}
	var appDB, head string
	if err := pool.QueryRow(ctx, `SELECT current_database()`).Scan(&appDB); err != nil {
		return err
	}
	if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil {
		return err
	}
	config, err := pgxpool.ParseConfig(database)
	if err != nil {
		return err
	}
	counts := map[string]map[string]int64{}
	databases := []string{appDB, envOr("FLUCTLIGHT_BACKUP_TEMPORAL_DATABASE", "temporal"), envOr("FLUCTLIGHT_BACKUP_VISIBILITY_DATABASE", "temporal_visibility")}
	if databases[0] == databases[1] || databases[0] == databases[2] || databases[1] == databases[2] {
		return errors.New("database identities overlap")
	}
	for _, name := range databases {
		cfg := config.Copy()
		cfg.ConnConfig.Database = name
		p, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			return err
		}
		tables, err := tableCounts(ctx, p)
		p.Close()
		if err != nil {
			return err
		}
		counts[name] = tables
	}
	workflows := []map[string]any{}
	rows, err := pool.Query(ctx, `SELECT intent_id,workflow_id,intent_type,status FROM public.platform_workflow_intents WHERE status IN ('pending','retry','started','paused','cancel_requested') ORDER BY intent_id`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id, wf, kind, status string
		if err := rows.Scan(&id, &wf, &kind, &status); err != nil {
			rows.Close()
			return err
		}
		workflows = append(workflows, map[string]any{"intent_id": id, "workflow_id": wf, "kind": kind, "status": status})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	var invalidFK int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_constraint WHERE contype='f' AND NOT convalidated`).Scan(&invalidFK); err != nil {
		return err
	}
	endpoint, err := url.Parse(os.Getenv("S3_ENDPOINT"))
	if err != nil || endpoint.Host == "" {
		return errors.New("storage endpoint required")
	}
	storage, err := minio.New(endpoint.Host, &minio.Options{Creds: credentials.NewStaticV4(os.Getenv("S3_ACCESS_KEY"), os.Getenv("S3_SECRET_KEY"), ""), Secure: endpoint.Scheme == "https", Region: os.Getenv("S3_REGION")})
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(map[string]any{"type": "manifest", "format": "fluctlight.backup-probe.v1", "migration_head": head, "database_counts": counts, "active_workflows": workflows, "unvalidated_foreign_keys": invalidFK, "settings_key_verified": true}); err != nil {
		return err
	}
	rows, err = pool.Query(ctx, `SELECT id,bucket,object_key,COALESCE(object_version,''),byte_size,sha256 FROM public.media_assets WHERE status='ready' ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	checked, legacy := 0, 0
	for rows.Next() {
		var id, bucket, key, version, expected string
		var size int64
		if err := rows.Scan(&id, &bucket, &key, &version, &size, &expected); err != nil {
			return err
		}
		object, err := storage.GetObject(ctx, bucket, key, minio.GetObjectOptions{VersionID: version})
		if err != nil {
			return errors.New("media object unavailable")
		}
		hash := sha256.New()
		n, readErr := io.Copy(hash, io.LimitReader(object, size+1))
		closeErr := object.Close()
		if readErr != nil || closeErr != nil || n != size || hex.EncodeToString(hash.Sum(nil)) != expected {
			return errors.New("media object integrity mismatch")
		}
		if version == "" {
			legacy++
		}
		checked++
		if err := encoder.Encode(map[string]any{"type": "media", "id": id, "bucket": bucket, "object_key": key, "object_version": version, "byte_size": n, "sha256": expected}); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return encoder.Encode(map[string]any{"type": "end", "complete": true, "media_checked": checked, "legacy_media_versions": legacy, "models_started": false})
}
func tableCounts(ctx context.Context, pool *pgxpool.Pool) (map[string]int64, error) {
	rows, err := pool.Query(ctx, `SELECT schemaname,tablename FROM pg_tables WHERE schemaname NOT IN ('pg_catalog','information_schema') ORDER BY schemaname,tablename`)
	if err != nil {
		return nil, err
	}
	names := [][2]string{}
	for rows.Next() {
		var schema, table string
		if err := rows.Scan(&schema, &table); err != nil {
			rows.Close()
			return nil, err
		}
		names = append(names, [2]string{schema, table})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	sort.Slice(names, func(i, j int) bool { return strings.Join(names[i][:], ".") < strings.Join(names[j][:], ".") })
	counts := map[string]int64{}
	for _, name := range names {
		var count int64
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{name[0], name[1]}.Sanitize()).Scan(&count); err != nil {
			return nil, err
		}
		counts[name[0]+"."+name[1]] = count
	}
	return counts, nil
}
func envOr(key, value string) string {
	if actual := os.Getenv(key); actual != "" {
		return actual
	}
	return value
}
