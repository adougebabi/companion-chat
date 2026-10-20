package core

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

const cancelledConfigurationDigest = "configuration-command-cancelled"

func cancelledConfigurationError() error {
	return &SurfaceError{Status: 409, Code: "configuration_command_cancelled", Message: "原配置提交已停止，请使用当前草稿重新保存"}
}

func (a *App) configurationReader(ctx context.Context) lifeContextQuerier {
	if tx, ok := SurfaceDBTx(ctx); ok {
		return tx
	}
	return a.DB.Pool()
}
func (a *App) configurationSnapshot(ctx context.Context, read func(context.Context) (map[string]any, error)) (map[string]any, error) {
	if _, ok := SurfaceDBTx(ctx); ok {
		return read(ctx)
	}
	tx, err := a.DB.Pool().BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	result, err := read(context.WithValue(ctx, surfaceDBTxKey{}, tx))
	if err != nil {
		return nil, err
	}
	return result, tx.Commit(ctx)
}

// Existing domain tables remain the writers. These counters also survive a
// secret clear / endpoint rebinding so deletion cannot reset a CAS identity.
func configurationVersionTx(ctx context.Context, q lifeContextQuerier, key string) (int, error) {
	var revision int
	sql := `SELECT revision FROM public.runtime_settings WHERE key=$1`
	if strings.HasPrefix(key, "secret:") || strings.HasPrefix(key, "endpoint:") || strings.HasPrefix(key, "binding:") {
		sql = `SELECT revision FROM public.configuration_revisions WHERE resource_key=$1`
	}
	err := q.QueryRow(ctx, sql, key).Scan(&revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return revision, err
}

func beginConfigurationCommandTx(ctx context.Context, tx pgx.Tx, owner, operation string, payload map[string]any, resources []string) (map[string]any, bool, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('owner-configuration',0))`); err != nil {
		return nil, false, err
	}
	key := stringValue(payload["idempotency_key"])
	if _, provided := payload["idempotency_key"]; !provided {
		return nil, false, nil
	} // Trusted internal setup callers keep their existing interface.
	if key == "" || strings.TrimSpace(key) != key || len(key) > 256 {
		return nil, false, invalid("Invalid configuration command identity")
	}
	if _, ok := payload["expected_versions"].(map[string]any); !ok {
		return nil, false, invalid("Expected configuration versions required")
	}
	digest := stableDigest(operation + ":" + jsonString(payload))
	var prior string
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT digest,response FROM public.owner_surface_idempotency WHERE owner_actor_id=$1 AND idempotency_key=$2`, owner, key).Scan(&prior, &raw)
	if err == nil {
		if prior == cancelledConfigurationDigest {
			return nil, false, cancelledConfigurationError()
		}
		if prior != digest {
			return nil, false, &SurfaceError{Status: 409, Code: "idempotency_conflict", Message: "这个提交身份属于另一份配置内容"}
		}
		return decodeObject(raw), true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}
	expected := mapValue(payload["expected_versions"])
	for _, resource := range resources {
		want, ok := parseRevision(expected[resource])
		if !ok {
			if _, present := expected[resource]; present {
				return nil, false, invalid("Expected configuration resource versions required")
			}
			want = 0
		}
		actual, err := configurationVersionTx(ctx, tx, resource)
		if err != nil {
			return nil, false, err
		}
		if want != actual {
			return nil, false, conflict("configuration", want, actual)
		}
	}
	return map[string]any{"command_id": "configuration_" + stableDigest(owner+":"+key), "digest": digest, "resource_keys": resources}, false, nil
}

func finishConfigurationCommandTx(ctx context.Context, tx pgx.Tx, owner string, payload, receipt map[string]any, operation string) error {
	versions := map[string]any{}
	for _, raw := range arrayValue(receipt["resource_keys"]) {
		key := stringValue(raw)
		version, err := configurationVersionTx(ctx, tx, key)
		if err != nil {
			return err
		}
		versions[key] = version
	}
	// resource_keys is a []string before JSON roundtrip.
	if resources, ok := receipt["resource_keys"].([]string); ok {
		for _, key := range resources {
			version, err := configurationVersionTx(ctx, tx, key)
			if err != nil {
				return err
			}
			versions[key] = version
		}
	}
	receipt["applied_versions"] = versions
	digest := stringValue(receipt["digest"])
	delete(receipt, "digest")
	id := stringValue(receipt["command_id"])
	if _, err := tx.Exec(ctx, `INSERT INTO public.owner_surface_audits(id,owner_actor_id,fluctlight_id,identity,actor,action,reason,record_id) VALUES($1,$2,'','owner_edit','owner',$3,$4,$5)`, randomID("audit_configuration_"), owner, operation, "修改配置资源："+jsonString(receipt["resource_keys"]), id); err != nil {
		return err
	}
	if key := stringValue(payload["idempotency_key"]); key != "" {
		_, err := tx.Exec(ctx, `INSERT INTO public.owner_surface_idempotency(owner_actor_id,idempotency_key,digest,response) VALUES($1,$2,$3,$4)`, owner, key, digest, jsonBytes(receipt))
		return err
	}
	return nil
}

func (a *App) configurationCommandReplayed(ctx context.Context, owner, operation string, payload map[string]any) (bool, error) {
	key := stringValue(payload["idempotency_key"])
	if _, provided := payload["idempotency_key"]; !provided {
		return false, nil
	}
	if key == "" || strings.TrimSpace(key) != key || len(key) > 256 {
		return false, invalid("Invalid configuration command identity")
	}
	var digest string
	err := a.DB.Pool().QueryRow(ctx, `SELECT digest FROM public.owner_surface_idempotency WHERE owner_actor_id=$1 AND idempotency_key=$2`, owner, key).Scan(&digest)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if digest != stableDigest(operation+":"+jsonString(payload)) {
		if digest == cancelledConfigurationDigest {
			return false, cancelledConfigurationError()
		}
		return false, &SurfaceError{Status: 409, Code: "idempotency_conflict", Message: "这个提交身份属于另一份配置内容"}
	}
	return true, nil
}

// Share admission's lock and ledger: a request still in external preflight may
// not commit after the Owner has explicitly stopped its unaccepted identity.
func (a *App) CancelUnacceptedConfigurationCommand(ctx context.Context, owner, key string) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	if key == "" || strings.TrimSpace(key) != key || len(key) > 256 {
		return nil, invalid("Original configuration command key required")
	}
	var result map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('owner-configuration',0))`); err != nil {
			return err
		}
		var raw []byte
		err := tx.QueryRow(ctx, `SELECT response FROM public.owner_surface_idempotency WHERE owner_actor_id=$1 AND idempotency_key=$2`, owner, key).Scan(&raw)
		if err == nil {
			prior := decodeObject(raw)
			if !strings.HasPrefix(stringValue(prior["command_id"]), "configuration_") {
				return &SurfaceError{Status: 409, Code: "idempotency_conflict", Message: "这个提交身份不属于配置操作"}
			}
			result = compactStateMap(prior, []string{"command_id", "resource_keys", "applied_versions", "wake_rearm_required", "status"})
			if result["status"] != "cancelled" {
				result["status"] = "committed"
			}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		id := "configuration_" + stableDigest(owner+":"+key)
		result = map[string]any{"command_id": id, "status": "cancelled"}
		if _, err := tx.Exec(ctx, `INSERT INTO public.owner_surface_idempotency(owner_actor_id,idempotency_key,digest,response) VALUES($1,$2,$3,$4)`, owner, key, cancelledConfigurationDigest, jsonBytes(result)); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO public.owner_surface_audits(id,owner_actor_id,fluctlight_id,identity,actor,action,reason,record_id) VALUES($1,$2,'','owner_edit','owner','configuration_cancel_unaccepted','停止尚未受理的原配置提交',$3)`, randomID("audit_configuration_"), owner, id)
		return err
	})
	return result, err
}
