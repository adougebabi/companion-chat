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
- Decrypted values exist only in the Go Provider adapter call scope and never enter the browser boundary, prompts, traces, logs, exceptions, debug views, OpenAPI examples, or ordinary DTOs.
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

- Good: Owner updates an API key, PostgreSQL stores ciphertext, UI returns configured state, and only the Go Provider adapter resolves it.
- Base: a non-secret model selection changes without touching the existing encrypted key.
- Bad: persist `.env` startup credentials into settings, store an API key plaintext, return a masked value that later overwrites the real key, or silently use an old env Provider key after decryption failure.

### 6. Tests Required

- Startup config tests for each required/optional env, role-specific readiness, bounded errors, and no value leakage.
- Sensitive-setting tests for AEAD round trip, wrong/missing key, nonce uniqueness, AAD/purpose mismatch, explicit clear, empty/masked no-op, and atomic patch rollback.
- API/browser boundary tests proving secret fields are write-only and never enter generated browser DTOs.
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

## Scenario: Versioned Owner configuration and safe submission recovery (0091)

### 1. Scope / Trigger

The current v3 configuration editor writes existing runtime settings, secrets,
Provider endpoints and role bindings. A lost response or another open editor
must not overwrite an intervening configuration or recreate a cleared secret.

### 2. Signatures

`PUT /api/settings` adds `expectedVersions: Record<string,number>` and
`idempotencyKey`. Provider endpoint/role writes carry the same fields.
`GET /api/configuration-catalog` returns stored/effective values and their source;
`GET /api/configuration-commands/by-key?key=…` returns only a safe committed receipt.
`POST /api/configuration-commands/recover {key}` resumes accepted post-save wake-up
rearming, with no credential replay. `configuration_revisions(resource_key,revision)`
retains `secret:purpose`, `endpoint:id`, and `binding:role` versions across deletion.

`POST /api/configuration-commands/cancel-unaccepted {key}` resolves an old pending
configuration identity under the same admission lock. An existing committed
receipt returns `status:committed` and is never undone. Otherwise the existing
Owner-scoped idempotency ledger receives a `configuration-command-cancelled`
digest and `status:cancelled` receipt atomically with audit; late admission and
early replay both reject that key with `configuration_command_cancelled`.
Repeated cancellation returns the same identity. Unknown cancellation responses
retain the client record; a by-key 404 alone is never proof of non-admission.
This reuses the existing ledger and requires no schema migration.

Model binding with an explicit real context window and omitted max input uses
`window - output - policy safety margin`, validated by Core before preflight.
The configuration catalog supplies `provider_budget_defaults`; existing bindings
retain their own policy/margin. UI input budget is a derived read-only preview,
recomputed on edits and source refresh; it is omitted from the write payload.
The UI does not infer a model's actual loaded window from its name. Explicit
legacy max input remains supported and validated.

### 3. Contracts

- Serialize configuration admission and effect under the same advisory lock.
  Check the Owner/key digest before CAS; use identical canonical keys at every
  read/write. A supplied key must be nonempty, trim-free and at most 256 bytes.
- Runtime settings use their owning revision. Credential ciphertext and
  endpoint/binding tables remain their existing authorities; counters add no
  second copy of their values. Status checks and timestamps alone do not bump
  an endpoint's configuration revision.
- Endpoint saves protect every binding they will delete, including a newly
  attached concurrent binding. Model discovery runs outside the transaction;
  final binding admission rechecks role, endpoint and credential revisions.
- Responses/audits/command recovery contain purpose/resource names and versions,
  never credential plaintext. Browser recovery stores a salted fingerprint,
  original body and key; the user must re-enter an uncommitted original secret.
- Keep all current editors mounted after sibling saves. A late success cannot
  clear a newer credential. Effective defaults come from the actual runtime;
  unconfigured and failed configuration are distinct. Saved Comfy connectivity
  checks do not claim that image generation passed.
- Exact pre-write Provider validation codes, including invalid prompt budget,
  cross the in-process browser boundary as 422 rather than generic preflight
  502. Only known first-attempt validation rejection unlocks a new proposal;
  unknown outcomes require original replay or the explicit server fence above.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Blank/trimmed key or malformed versions | Reject before mutation |
| Reused key with a different body | `409 idempotency_conflict` |
| Stale setting/secret/binding/endpoint | `409 revision_conflict`, retain draft |
| Secret replaced and cleared together | Reject atomically |
| Model directory has an unknown envelope | Explicit error, not a successful empty list |
| Accepted wake-up rearm failed after settings committed | Recover the original receipt/work; do not re-save credentials |

### 5. Good / Base / Bad Cases

Good: credential A commits, the user types B during the wait, and B remains.
Base: blank credential input preserves the previous encrypted value.
Bad: endpoint v1 deletes a newly bound role without checking its binding version.

### 6. Tests Required

At the final unified regression, cover empty/0090 upgrade/rerun; counter
tombstones; concurrent endpoint/binding edits; same-key replay/changed payload;
wakeup post-save recovery; absence of credential plaintext in storage/receipts;
decimal form validation; sibling saves preserving credential input; and actual
consumer effects. Source review/build evidence alone does not pass these cases.

### 7. Wrong vs Correct

Wrong: CAS only `endpoint:id`, then delete all of its role bindings.
Correct: under the shared admission lock, enumerate and CAS every affected
`binding:role`, then commit the endpoint, removals, audit and receipt together.
