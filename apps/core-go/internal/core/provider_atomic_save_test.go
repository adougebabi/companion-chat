package core

import (
	"bytes"
	"strings"
	"testing"
)

func providerAtomicVersions(t *testing.T, f independentToolE2EFixture) map[string]any {
	t.Helper()
	settings, err := f.app.ReadSettings(f.ctx, f.ownerID)
	if err != nil {
		t.Fatal(err)
	}
	return mapValue(settings["versions"])
}

func TestProviderAtomicSaveAndCredentialOnlyRotation(t *testing.T) {
	f := v3OwnerFixture(t, "provider-atomic")
	key := bytes.Repeat([]byte{7}, 32)
	f.app.SettingsKey = key
	plain := strings.Repeat("fixture-a", 3)
	body := map[string]any{"endpoint_id": "atomic-endpoint", "kind": "openai-compatible", "base_url": "http://controlled.invalid/v1", "secret_purpose": "atomic-purpose", "secrets": map[string]any{"atomic-purpose": plain}, "expected_versions": map[string]any{}, "idempotency_key": "atomic-create"}
	if err := f.app.ConfigureProviderEndpoint(f.ctx, f.ownerID, body); err != nil {
		t.Fatal(err)
	}
	var ciphertext, nonce []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT ciphertext,nonce FROM public.setting_secrets WHERE purpose='atomic-purpose'`).Scan(&ciphertext, &nonce); err != nil {
		t.Fatalf("endpoint committed without credential: %v", err)
	}
	if bytes.Contains(ciphertext, []byte(plain)) {
		t.Fatal("plaintext stored")
	}
	if actual, err := decryptSecret(key, "atomic-purpose", nonce, ciphertext); err != nil || actual != plain {
		t.Fatal("saved credential cannot be consumed", err)
	}
	before := providerAtomicVersions(t, f)
	if err := f.app.ConfigureProviderEndpoint(f.ctx, f.ownerID, body); err != nil {
		t.Fatal("exact replay", err)
	}
	if jsonString(before) != jsonString(providerAtomicVersions(t, f)) {
		t.Fatal("replay changed revisions")
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.provider_endpoints SET capability_status='ready' WHERE id='atomic-endpoint'; INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,token_budget,timeout_seconds) VALUES('conversation_reply','atomic-endpoint','fixture-model',100,30)`); err != nil {
		t.Fatal(err)
	}
	rotation := cloneMap(body)
	rotation["idempotency_key"] = "atomic-rotate"
	rotation["expected_versions"] = providerAtomicVersions(t, f)
	rotation["secrets"] = map[string]any{"atomic-purpose": strings.Repeat("fixture-b", 3)}
	endpointVersion := mapValue(rotation["expected_versions"])["endpoint:atomic-endpoint"]
	if err := f.app.ConfigureProviderEndpoint(f.ctx, f.ownerID, rotation); err != nil {
		t.Fatal(err)
	}
	var count int
	var status string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.model_roles WHERE provider_endpoint_id='atomic-endpoint'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("credential rotation removed binding", count, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT capability_status FROM public.provider_endpoints WHERE id='atomic-endpoint'`).Scan(&status); err != nil || status != "ready" {
		t.Fatal("rotation reset preflight", status, err)
	}
	if providerAtomicVersions(t, f)["endpoint:atomic-endpoint"] != endpointVersion {
		t.Fatal("rotation changed endpoint version")
	}
	secretVersion := providerAtomicVersions(t, f)["secret:atomic-purpose"]
	for index, value := range []any{"", "configured", nil} {
		noop := cloneMap(rotation)
		noop["secrets"] = map[string]any{"atomic-purpose": value}
		noop["idempotency_key"] = []string{"empty-value", "masked-value", "null-value"}[index]
		noop["expected_versions"] = providerAtomicVersions(t, f)
		if err := f.app.ConfigureProviderEndpoint(f.ctx, f.ownerID, noop); err != nil {
			t.Fatal(err)
		}
		if providerAtomicVersions(t, f)["secret:atomic-purpose"] != secretVersion {
			t.Fatal("blank or masked value overwrote credential")
		}
	}
	clear := cloneMap(rotation)
	delete(clear, "secrets")
	clear["clear_secrets"] = []any{"atomic-purpose"}
	clear["idempotency_key"] = "atomic-clear"
	clear["expected_versions"] = providerAtomicVersions(t, f)
	if err := f.app.ConfigureProviderEndpoint(f.ctx, f.ownerID, clear); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.setting_secrets WHERE purpose='atomic-purpose'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("explicit clear did not remove ciphertext", err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.model_roles WHERE provider_endpoint_id='atomic-endpoint'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("clear removed model binding", err)
	}
	changed := cloneMap(rotation)
	changed["base_url"] = "http://new-controlled.invalid/v1"
	changed["idempotency_key"] = "atomic-change"
	changed["expected_versions"] = providerAtomicVersions(t, f)
	if err := f.app.ConfigureProviderEndpoint(f.ctx, f.ownerID, changed); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.model_roles WHERE provider_endpoint_id='atomic-endpoint'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("changed endpoint kept stale binding", count, err)
	}
	changedAgain := cloneMap(changed)
	changedAgain["base_url"] = "http://different.invalid/v1"
	requireV3SurfaceError(t, f.app.ConfigureProviderEndpoint(f.ctx, f.ownerID, changedAgain), "idempotency_conflict")
	var records string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT COALESCE(string_agg(response::text,''),'') FROM public.owner_surface_idempotency WHERE owner_actor_id=$1`, f.ownerID).Scan(&records); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(records, plain) || strings.Contains(records, strings.Repeat("fixture-b", 3)) {
		t.Fatal("credential leaked into receipt")
	}
}

func TestProviderAtomicSavePreservesOldConnectionOnLateFailure(t *testing.T) {
	f := v3OwnerFixture(t, "atomic-existing")
	f.app.SettingsKey = bytes.Repeat([]byte{11}, 32)
	old := strings.Repeat("fixture-old", 2)
	encrypted, err := encryptSecret(f.app.SettingsKey, "existing-purpose", old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status) VALUES('existing','openai-compatible','http://old.invalid/v1','existing-purpose','ready');INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,token_budget,timeout_seconds) VALUES('conversation_reply','existing','fixture',100,30)`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.setting_secrets(purpose,ciphertext,nonce,updated_at) VALUES('existing-purpose',$1,$2,now())`, encrypted.ciphertext, encrypted.nonce); err != nil {
		t.Fatal(err)
	}
	before := providerAtomicVersions(t, f)
	if _, err := f.repository.Pool().Exec(f.ctx, `CREATE FUNCTION public.reject_existing_secret() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'controlled secret failure'; END $$;CREATE TRIGGER reject_existing_secret BEFORE INSERT OR UPDATE ON public.setting_secrets FOR EACH ROW EXECUTE FUNCTION public.reject_existing_secret()`); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"endpoint_id": "existing", "kind": "openai-compatible", "base_url": "http://new.invalid/v1", "secret_purpose": "existing-purpose", "secrets": map[string]any{"existing-purpose": strings.Repeat("fixture-new", 2)}, "expected_versions": before, "idempotency_key": "existing-failed-change"}
	if err := f.app.ConfigureProviderEndpoint(f.ctx, f.ownerID, body); err == nil {
		t.Fatal("late failure accepted")
	}
	var base, status string
	var bindings int
	var ciphertext, nonce []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT e.base_url,e.capability_status,s.ciphertext,s.nonce,(SELECT count(*) FROM public.model_roles WHERE provider_endpoint_id=e.id) FROM public.provider_endpoints e JOIN public.setting_secrets s ON s.purpose=e.secret_purpose WHERE e.id='existing'`).Scan(&base, &status, &ciphertext, &nonce, &bindings); err != nil {
		t.Fatal(err)
	}
	if base != "http://old.invalid/v1" || status != "ready" || bindings != 1 || jsonString(before) != jsonString(providerAtomicVersions(t, f)) {
		t.Fatal("existing connection partially mutated")
	}
	if actual, err := decryptSecret(f.app.SettingsKey, "existing-purpose", nonce, ciphertext); err != nil || actual != old {
		t.Fatal("existing credential changed", err)
	}
}

func TestProviderAtomicSaveRollsBackEveryFailure(t *testing.T) {
	for _, mode := range []string{"encryption", "secret-version", "late-sql"} {
		t.Run(mode, func(t *testing.T) {
			f := v3OwnerFixture(t, "atomic-rollback")
			f.app.SettingsKey = bytes.Repeat([]byte{8}, 32)
			body := map[string]any{"endpoint_id": "rollback-endpoint", "kind": "openai-compatible", "base_url": "http://controlled.invalid/v1", "secret_purpose": "rollback-purpose", "secrets": map[string]any{"rollback-purpose": strings.Repeat("fixture", 3)}, "expected_versions": map[string]any{}, "idempotency_key": "rollback-command"}
			switch mode {
			case "encryption":
				f.app.SettingsKey = nil
			case "secret-version":
				body["expected_versions"] = map[string]any{"secret:rollback-purpose": 99}
			case "late-sql":
				if _, err := f.repository.Pool().Exec(f.ctx, `CREATE FUNCTION public.reject_atomic_secret() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'controlled secret failure'; END $$; CREATE TRIGGER reject_atomic_secret BEFORE INSERT ON public.setting_secrets FOR EACH ROW EXECUTE FUNCTION public.reject_atomic_secret()`); err != nil {
					t.Fatal(err)
				}
			}
			if err := f.app.ConfigureProviderEndpoint(f.ctx, f.ownerID, body); err == nil {
				t.Fatal("partial save accepted")
			}
			var endpoints, credentials, receipts int
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.provider_endpoints WHERE id='rollback-endpoint'),(SELECT count(*) FROM public.setting_secrets WHERE purpose='rollback-purpose'),(SELECT count(*) FROM public.owner_surface_idempotency WHERE idempotency_key='rollback-command')`).Scan(&endpoints, &credentials, &receipts); err != nil || endpoints != 0 || credentials != 0 || receipts != 0 {
				t.Fatal("partial state", endpoints, credentials, receipts, err)
			}
		})
	}
}

func TestProviderAtomicSaveRejectsForeignCredentialPurpose(t *testing.T) {
	f := v3OwnerFixture(t, "atomic-invalid")
	f.app.SettingsKey = bytes.Repeat([]byte{9}, 32)
	for _, extra := range []map[string]any{{"secrets": map[string]any{"other-purpose": "invalid"}}, {"secrets": map[string]any{"target": true}}, {"secrets": map[string]any{"target": "new"}, "clear_secrets": []any{"target"}}, {"clear_secrets": []any{"other-purpose"}}} {
		body := map[string]any{"endpoint_id": "invalid-endpoint", "kind": "openai-compatible", "base_url": "http://controlled.invalid/v1", "secret_purpose": "target"}
		for k, v := range extra {
			body[k] = v
		}
		if err := f.app.ConfigureProviderEndpoint(f.ctx, f.ownerID, body); err == nil {
			t.Fatal("accepted invalid credential mutation")
		}
	}
}
