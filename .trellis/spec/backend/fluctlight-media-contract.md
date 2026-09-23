# Fluctlight Media Contract

## Scenario: Private S3-Compatible Media With Durable Lifecycle

### 1. Scope / Trigger

- Trigger: the clean-start system generates, uploads, attaches, reads, proxies, versions, tombstones, deletes, backs up, or restores image/video/audio media.
- Go Core `media` owns business identity and authorization. Object storage owns
  bytes. The Go browser boundary is an authorized transport proxy, not a media-state owner.
- The deployment uses an S3-compatible interface; Docker Compose defaults to a pinned MinIO single-node persistent volume.

### 2. Signatures

```python
request_generation(command: RequestMedia, tx: UnitOfWork) -> MediaIntent
record_uploaded(command: RecordUploadedObject, tx: UnitOfWork) -> MediaAsset
attach(command: AttachMedia, tx: UnitOfWork) -> MediaReference
authorize_read(query: AuthorizeMediaRead) -> InternalMediaGrant
tombstone(command: TombstoneMedia, tx: UnitOfWork) -> DeletionIntent
record_deleted(command: RecordObjectDeleted, tx: UnitOfWork) -> MediaAsset
```

Required asset fields:

```text
id / version
owner_fluctlight_id
media_kind / mime_type / byte_size / sha256
bucket / object_key / object_version / etag
provider / provider_request_id / workflow_id
status: pending | uploading | ready | unavailable | tombstoned | deleted
created_at / ready_at / tombstoned_at / deleted_at
```

Media quality state is durable on `media_intents`: `quality_retry_count` tracks
the one media-specific correction, `quality_retry_guidance` retains the
reviewer guidance, `quality_retry_feedback` stores the bounded structured
review result as JSONB, and `quality_verdict` records `pass`, `retry`,
`reject`, `skipped`, or `retry_accepted` (`retry_accepted` fits the existing
`varchar(16)` column).

```go
func (a *App) ProcessMediaIntent(ctx context.Context, intentID string) (map[string]any, error)
```

On completion the result contains `intent_id`, `status`, `quality_verdict`
(the delivery disposition), and `quality_check_verdict` (the actual latest C
review verdict).

`InternalMediaGrant` contains asset/version identity, authorized range policy, short expiry, content metadata, and an internal presigned/object request that is not returned directly to the browser in the default NAS mode.

### Moment Image Contract

- A text Moment is published first. Its optional image starts only from an
  already-frozen `media.image.generate` CapabilityInvocation whose thin input
  is the model-owned `intent`; it never comes from an existing asset list, a
  keyword branch, or renderer inference.
- A wake-up may request a standalone `media.image.generate` call without a
  conversation or Moment output. Its `wake_up` target is represented by the
  stable action provenance; the media intent remains durable and un-attached
  until a later product projection explicitly references the generated asset.
- A direct conversation may execute `media.image.generate` without a
  `conversation.reply` sibling. Core binds the media intent to the durable
  conversation (`conversation_id` with no `message_id`); when the asset becomes
  ready, the media workflow creates one idempotent `media_reference` message.
- `MediaIntent.moment_id` is the durable target reference. It is nullable for
  conversation media and mandatory for Moment-image work; it is backed by the
  `media_intents.moment_id -> moments.id` foreign key.
- The media workflow generates only an image for this branch. Video/h3 does not
  share this target or create a video intent while video capability is deferred.
- Provider success records one ready asset, creates a `MediaReference` with
  `target_type="moment"`, then idempotently adds the asset ID to the original
  Moment. Provider failure preserves the published text Moment and no fake
  asset reference is created.

### 3. Contracts

- Buckets are private. Browser requests use the Go browser boundary media endpoint; Go Core
  performs Actor/Conversation/reference authorization before issuing a
  short-lived internal grant.
- Go may proxy bytes, Range, ETag, Content-Type, Content-Length, and cache
  headers from the grant. It cannot infer authorization, query media tables,
  or mint object grants.
- Application code uses only the S3-compatible interface. MinIO-specific administration remains deployment tooling.
- Object keys are stable generated identities such as `media/{asset_id}/{object_version}` and never user-controlled filenames or local absolute paths.
- PostgreSQL records SHA-256 and byte size; ETag alone is not a content-integrity guarantee.
- Generation/upload happens after a committed media intent. The final transaction validates workflow ID, asset revision, checksum, size, and references before marking ready.
- An image capability call carries the cognition-time `context_binding`
  snapshot. Core preserves subject/pose/style fields, but aligns missing or
  conflicting scene/activity/location (and concrete outfit/hair/mood values
  when present) with that snapshot unless `context_override.explicit` is true.
  The media prompt Provider receives the same binding and must not invent a
  different room or activity.
- Renderer constraints must be resolved consistently for every configured
  ComfyUI workflow: accept the canonical top-level `renderer_constraints` and
  the cognition-time `context_binding.visual_identity.renderer_constraints`
  shape, with explicit top-level values taking precedence. A workflow variant
  must not silently lose `chest_lora_weight` when the same frozen concept is
  routed through it.
- The external Provider job ID is persisted on the committed media intent immediately after submission. Activity retries reuse that ID for polling and cancellation; they never submit a second Provider job for the same intent. A ready asset is the authoritative replay boundary, so retries only reapply idempotent conversation/Moment projections.
- Visual Identity seed/candidate/character-sheet media intents are workflow-owned
  assets. They do not bind to the direct conversation or create assistant chat
  messages; Diagnostics and the Visual Identity timeline are their projection
  surfaces.
- A downloaded image candidate is inspected by the C-stage visual consistency review before Core writes a `ready` asset or creates a media reference. C reuses the configured `media_prompt` model with the frozen media concept, authoritative context, final provider prompt, and a bounded private image input. An infrastructure-only C failure (model unavailable/timeout/unsupported vision/invalid response or unsafe candidate read) records `skipped` and fail-open publishes the Provider-successful candidate. On the first explicit non-pass (`retry` or `reject`), Core persists the bounded verdict, violations, observed facts, and guidance in `media_intents.quality_retry_feedback`, then runs the formal Media Prompt Agent with that feedback and the previous provider prompt before submitting one second Provider job for the same intent and frozen concept. A passing second candidate follows normal upload/publish. A second `retry` or `reject` is recorded truthfully, changes the delivery state to `retry_accepted`, and still uploads/publishes that second candidate. It must not fail the media target solely for that second quality verdict or start a third quality-driven generation. This one correction is a media lifecycle rule and does not limit general Tool invocations.
- Long media activities record an initial and periodic Temporal heartbeat while
  prompt generation, Provider submission, object download, or polling is in
  flight. Once `provider_job_id` is persisted, retries skip prompt generation
  and poll that same job; a heartbeat timeout must not cause a second Provider
  submission.
- When the final media Activity attempt fails, persist a bounded application
  error to both the media target and its workflow intent before reconciliation
  can apply a generic terminal fallback. Reconciliation may read the terminal
  Temporal history to recover the leaf failure message, but a transient history
  read failure must leave the intent eligible for a later pass instead of
  permanently committing `workflow_terminal_failure`.
- A media retry locks the media row and the optional workflow-intent row in
  separate statements (PostgreSQL cannot `FOR UPDATE` the nullable side of a
  `LEFT JOIN`). It rejects paused, started, and cancellation-requested
  executions, repairs a missing legacy workflow intent, preserves the frozen
  provider prompt, and distinguishes transient runtime restart failures from
  permanent payload/identity errors.
- Deletion first removes/invalidates active references and commits a tombstone/outbox intent. Physical object/version deletion is retryable and only then marks `deleted`.
- Upload success followed by database failure reuses the same object key/request identity on retry or is collected as an orphan. It never creates a second user-visible asset.
- Bucket versioning is enabled. Lifecycle rules remove obsolete/non-current versions according to an explicit retention policy.
- PostgreSQL and object storage are backed up under one manifest containing database snapshot identity, bucket/version scope, counts, and integrity results.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Actor is not authorized for the referenced asset | Return not-found/forbidden policy result; issue no grant. |
| Asset is not `ready` or is tombstoned/deleted | Issue no read grant. |
| Browser attempts to use bucket/key directly | Unsupported; browser contract accepts media asset ID only. |
| Upload checksum/size differs from committed intent/result | Keep asset unavailable/failed; do not attach it. |
| Object upload succeeds and result transaction crashes | Retry with stable object key or recover the existing object; avoid duplicate asset. |
| Tombstone commits and physical delete fails | Keep tombstoned, hide from reads, retry deletion. |
| Duplicate delete succeeds/object already absent | Treat as idempotent success after validating the intended object version. |
| Range is invalid or outside byte size | Return bounded range error without reading another object/version. |
| Object storage unavailable | Preserve authoritative media state and retry according to workflow; never delete references speculatively. |
| First quality review returns `retry` or `reject` | Persist structured feedback and guidance, optimize the provider prompt with the formal Media Prompt Agent, and generate once more using the same intent/target. |
| Second quality review still returns `retry` or `reject` | Preserve the actual review verdict, mark delivery `retry_accepted`, publish the second candidate, and do not generate a third image. |
| Backup lacks matching database/object manifest verification | Backup is incomplete and cannot be marked restorable. |

### 5. Good / Base / Bad Cases

- Good: a private video request is authorized by Go Core, the Go browser boundary proxies a valid byte range, and the browser can seek without seeing bucket credentials.
- Good: the first image review identifies a concrete mismatch, the Media Prompt Agent receives its structured feedback, and a second image is published with `quality_verdict=retry_accepted` if it still does not pass.
- Good: a deleted Message removes its final media reference, commits a tombstone, and retries physical deletion after an object outage.
- Base: an uploaded object exists before its result transaction; retry finds and verifies the same stable key, then marks one asset ready.
- Bad: fail immediately on the first quality `reject`, hide the second candidate when it still returns `retry`, or submit a third quality-driven image; also save absolute Provider paths, expose a public bucket, trust ETag as SHA-256, delete an object before removing references, or let the browser boundary query authorization tables.

### 6. Tests Required

- Media-intent/reference transaction tests for rollback, stable IDs, ownership, and outbox atomicity.
- Upload/recovery tests for checksum/size mismatch, duplicate upload, success-before-crash, orphan collection, and idempotent result commit.
- Provider retry tests assert that a persisted external job ID is polled without a second submission, ready-asset replay does not upload again, and cancellation targets the external job ID.
- Media quality retry tests assert that first-pass violations/observations/guidance reach the next Media Prompt Agent input, second-pass non-pass still publishes the same second candidate with `retry_accepted`, the actual verdict remains observable, and no third generation occurs. The PostgreSQL case also asserts durable JSONB feedback and candidate SHA identity.
- Authorization tests across Actor, Conversation, Message, Moment, and tombstoned/deleted states.
- browser boundary proxy tests for internal grant expiry, Range, ETag, MIME, cache headers, stream abort, unavailable object, and no leaked bucket credentials.
- Deletion tests for last-reference policy, tombstone/read denial, object failure/retry, object-already-absent, and version-specific deletion.
- S3 adapter contract tests run against the default pinned MinIO container and a fake adapter.
- Backup/restore tests validate one manifest, row/object counts, sampled SHA-256, missing versions, and restore into empty PostgreSQL/bucket stores.

### 7. Wrong vs Correct

#### Wrong

```typescript
app.get("/media/:key", async (req, res) => {
  return minio.getObject("public", req.params.key);
});
```

#### Correct

```typescript
app.get("/media/:assetId", async (req, res) => {
  const grant = await core.authorizeMediaRead({
    actorId: req.session.actorId,
    assetId: req.params.assetId,
    range: req.headers.range,
  });
  return proxyInternalMediaGrant(grant, res);
});
```

#### Wrong

```go
if verdict == "reject" || retryCount > 0 {
	return failMediaIntent() // hides the second candidate
}
```

#### Correct

```go
if retryCount == 0 && verdict != "pass" {
	return preparePromptCorrectionAndRetryOnce(review)
}
if retryCount == 1 && verdict != "pass" {
	return publishSameSecondCandidateWithVerdict("retry_accepted", review.Verdict)
}
```
