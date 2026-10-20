package core

import (
	"github.com/jackc/pgx/v5"
	"testing"
)

func TestConfigurationCancellationFencesLateAdmissionAndPreservesCommittedReceipt(t *testing.T) {
	f := v3OwnerFixture(t, "config-stop")
	key := "old-unknown-provider-request"
	stopped, err := f.app.CancelUnacceptedConfigurationCommand(f.ctx, f.ownerID, key)
	if err != nil || stopped["status"] != "cancelled" {
		t.Fatal(stopped, err)
	}
	again, err := f.app.CancelUnacceptedConfigurationCommand(f.ctx, f.ownerID, key)
	if err != nil || again["command_id"] != stopped["command_id"] {
		t.Fatal(again, err)
	}
	payload := map[string]any{"idempotency_key": key, "expected_versions": map[string]any{}}
	_, err = f.app.configurationCommandReplayed(f.ctx, f.ownerID, "provider_binding_update", payload)
	requireV3SurfaceError(t, err, "configuration_command_cancelled")
	err = withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		_, _, err := beginConfigurationCommandTx(f.ctx, tx, f.ownerID, "provider_binding_update", payload, nil)
		return err
	})
	requireV3SurfaceError(t, err, "configuration_command_cancelled")
	committedKey := "actually-saved-config"
	err = withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		body := map[string]any{"idempotency_key": committedKey, "expected_versions": map[string]any{}}
		receipt, _, err := beginConfigurationCommandTx(f.ctx, tx, f.ownerID, "provider_binding_update", body, nil)
		if err != nil {
			return err
		}
		return finishConfigurationCommandTx(f.ctx, tx, f.ownerID, body, receipt, "provider_binding_update")
	})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := f.app.CancelUnacceptedConfigurationCommand(f.ctx, f.ownerID, committedKey)
	if err != nil || saved["status"] != "committed" {
		t.Fatal("must not cancel committed config", saved, err)
	}
	if _, err := f.app.CancelUnacceptedConfigurationCommand(f.ctx, f.foreignOwnerID, key); err == nil {
		t.Fatal("foreign Owner cancelled configuration")
	}
}
