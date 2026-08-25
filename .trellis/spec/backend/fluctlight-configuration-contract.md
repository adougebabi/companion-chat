# Fluctlight Configuration Contract

## Scenario: Two-Layer Local Configuration With One Settings Key

### 1. Scope / Trigger

- Trigger: a process starts, infrastructure connection is configured, system settings are read/updated, or a Provider credential is stored/used.
- Configuration has exactly two authorities: startup `.env` and PostgreSQL system settings.
- The design intentionally avoids Vault/KMS, a secrets service, per-record data keys, key rings, and special encrypted backup archives.

### 2. Signatures

Startup env includes:

```text
FLUCTLIGHT_ENV
DATABASE_URL
REDIS_URL
S3_ENDPOINT / S3_REGION / S3_BUCKET
S3_ACCESS_KEY / S3_SECRET_KEY / S3_USE_SSL
FLUCTLIGHT_CORE_SERVICE_KEY
FLUCTLIGHT_SETTINGS_KEY
runtime host/port and optional OTLP endpoint
```

System settings commands:

```python
read_settings(actor: HumanActor) -> SafeSettingsView
update_settings(actor: HumanActor, patch: SettingsPatch) -> SafeSettingsView
resolve_provider_secret(purpose: str) -> SecretValue
```

Sensitive setting storage contains ciphertext, nonce, purpose/AAD scope, and update timestamp using the single `FLUCTLIGHT_SETTINGS_KEY`.

### 3. Contracts

- Startup/infrastructure values required before PostgreSQL settings can be read live only in `.env`/process environment and are not editable through the browser.
- Runtime-editable Provider URLs/keys/models, workflows, media/cognitive policies, and product options live in PostgreSQL system settings.
- Sensitive fields use a vetted AEAD implementation with `FLUCTLIGHT_SETTINGS_KEY`. The project does not implement custom cryptography.
- Missing/invalid settings key blocks sensitive setting save/resolve. It never falls back to plaintext or another old source.
- Secret fields are write-only in browser/API views. Responses expose only configured state and bounded timestamps/safe summaries.
- Empty, omitted, or masked sentinel values do not overwrite an existing secret. Clearing requires an explicit clear operation.
- Decrypted values exist only in the Python Provider adapter call scope and never enter Node, prompts, traces, logs, exceptions, debug views, OpenAPI examples, or ordinary DTOs.
- Setting changes are authorized to the Owner and audited by purpose/field/time/result without secret content.
- NAS backup documentation requires both application data and `.env`. Loss of the settings key requires re-entering Provider secrets but does not corrupt other domain data.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Required startup env missing/invalid | Fail the owning process readiness/startup with bounded field names and no secret values. |
| `FLUCTLIGHT_SETTINGS_KEY` missing and sensitive value is saved/read | Reject sensitive operation; never store/read plaintext fallback. |
| Settings key cannot decrypt existing ciphertext | Return explicit configuration error and require key correction or secret replacement. |
| Browser sends omitted/empty/masked secret | Keep existing value unless explicit clear is requested. |
| Unauthorized Actor updates settings | Reject before encryption/write. |
| Provider URL/model/policy fails schema | Reject patch atomically; preserve prior settings. |
| Secret clear requested | Delete ciphertext explicitly and return `configured: false`. |
| Log/debug serialization sees a secret type | Redact by type and fail tests if raw value appears. |

### 5. Good / Base / Bad Cases

- Good: Owner updates an API key, PostgreSQL stores ciphertext, UI returns configured state, and only the Python Provider adapter resolves it.
- Base: a non-secret model selection changes without touching the existing encrypted key.
- Bad: persist `.env` startup credentials into settings, store an API key plaintext, return a masked value that later overwrites the real key, or silently use an old env Provider key after decryption failure.

### 6. Tests Required

- Startup config tests for each required/optional env, role-specific readiness, bounded errors, and no value leakage.
- Sensitive-setting tests for AEAD round trip, wrong/missing key, nonce uniqueness, AAD/purpose mismatch, explicit clear, empty/masked no-op, and atomic patch rollback.
- API/BFF tests proving secret fields are write-only and never enter generated browser DTOs.
- Logging/trace/debug snapshot tests scanning for plaintext test secrets.
- Authorization/audit tests for Owner-only changes and no secret content in audit rows.
- Database dump fixture asserts Provider test secrets appear only as ciphertext.
- Backup/restore documentation test/checklist requires `.env` plus application data and verifies re-entry behavior after key loss.

### 7. Wrong vs Correct

#### Wrong

```python
settings.provider_api_key = patch.provider_api_key or os.getenv("OLD_API_KEY")
database.save(settings.model_dump())
```

#### Correct

```python
if patch.provider_api_key.is_explicit_value:
    encrypted = secret_codec.encrypt(
        purpose="provider_api_key",
        plaintext=patch.provider_api_key.value,
    )
    settings.update_secret(encrypted, tx=tx)
```
