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


### Owner original media and library (0084 source, undeployed)

Owner uploads use their own durable producer, never a synthetic Fluctlight/Studio/provider/workflow. Authentication and CSRF precede multipart parsing. One bounded file is spooled privately, sniffed and hashed. An Owner/key admission freezes filename, canonical MIME, byte size, checksum and storage scope. S3 I/O occurs outside result transactions. The same key reuses a verified exact object version; absent versioning or scope changes are explicit failures. A ready asset is committed only after exact-version byte verification. Result commit failure retains the original key/object for recovery. Unknown upload outcomes are queried by key, not silently assigned a new identity.

Library and byte grants share actual producer rules. Library filters execute before keyset pagination; role filtering requires an actual asset owner or media reference. The Owner library can include original uploads without automatically associating them with every character. Readability reports source/storage/lifecycle failures explicitly. Historical source fields are allowlisted; missing snapshots remain unknown. Annotation edits use two revisions and stable command identity, lock the asset in a separate statement before reading the annotation snapshot, and preserve bytes/provenance/permissions. Hiding an annotation is not deletion or authorization revocation.

Upload UI stores only immutable file descriptors and command/result IDs. Refresh confirms the existing producer without resending bytes; unsettled work requires the exact original file. Owner and original-key fences are checked again after asynchronous reads/hashing before any upload. Late responses cannot recreate logout-cleared storage. Retrying an older metadata command cannot overwrite a newer local draft. L09's original-media steps use fixed fixture bytes and truthful checksums; real execution remains separate from pure/controlled checks.

Display-avatar selection reuses owner_surface_records visual_identity/current and its visual:<fid> revision. It accepts an actual readable Owner image plus asset revision, or explicit canonical-follow mode; never a client URL. The short Owner/FID command writes current preference, immutable display selection history, audit and active display reference atomically. Reading validates frozen version/hash and current source/readability. Legacy unproven selection remains explicitly unavailable. This display preference never changes canonical identity, effective Life or model-reference authorization.


### Explicit Studio reference permission (0085 source, undeployed)

A media_reference_grant freezes exact Owner, asset version/revision/checksum/size/MIME, target studio_resource and purpose(scene/animal/object/person). Creation requires explicit confirmation and a currently readable image no larger than 32MiB. Permission provenance is owner_explicit_unverified_content; it never substitutes for a visual review, nonhuman proof, a current body or an Actor identity. Reading an asset or choosing a display avatar does not create this permission. Grant targets and content are immutable; changing them creates another explicit grant. Revocation compares grant revision and persists its history. Actual Studio job snapshots provide usage records without another job-state store.

Image admission and queueing verify the exact grant pair. The normalizer sees the bounded permitted reference and explicit unverified-content semantics without private Actor context. The original MediaWorkflow validates again before reading, retrieves exact object bytes with size/SHA256 checks, and checks revocation after the read and again after reference upload immediately before submitting a new Provider job. Already externalized effects cannot be undone by revocation; existing Provider IDs retain original polling/cancel behavior. No query/body key named undefined may be serialized as an actual pagination cursor. Legacy retained generated nonhuman references retain their existing source checks.

L16 includes actual uploaded reference, a user-selected person file, real generated image bytes and revocation/replay/usage assertions. Those live steps remain user-run after all functionality is complete. PG+controlled transport fixtures and authored migration upgrades remain distinct from real MinIO/Comfy/model acceptance.


### Owner image requests and item consumers (0086 source, undeployed)

General/item image requests use the existing durable Owner request and NativeCognition workflow. HTTP performs no Provider calls. The item route binds its target; optional image scenes and items carry explicit revisions, are validated/frozen on admission, and revalidated under Tool transaction locks. Source metadata only informs a generated picture, never Life, inventory ownership, wearing or actual position. The installed request registry wraps the original DirectTool image capability; the server alone binds owner_media_request and its source inbox. Runtime preflight must implement CapabilityPreflighter.Preflight(context.Context, CapabilityContext), not a similarly named incompatible signature.

One request row lock plus media_intents.owner_request_id UNIQUE allows at most one image intent, including multiple Tool calls or lost final JSON. Recovery consults committed accepted receipts before another Agent run. The original writer/outbox/MediaWorkflow creates and consumes the intent. A ready result publishes request/item references transactionally, using the original inventory media projection. No compatibility chat message/Moment is manufactured. Owner assessment acceptance means queued intent; product reads expose assessment, actual media lifecycle and actual ready/readable asset separately. Request tasks follow actual workflow payload request_id rather than guessed IDs.

Shared UI form/observer/drawer preserve immutable submitted bodies, result checkpoints and newer drafts across refresh/disposal. Unknown identities survive reset. Fresh source reads must succeed before releasing a rejected CAS identity. Rebase captures source selection and checks it after async reads and confirmation, retaining choices made while waiting. Closing observation does not cancel execution. Current-scene and current-outfit entrypoints use this same form, not legacy c1 fixture generation. Existing image planning still owns self visibility, effective appearance, canonical identity and capture constraints.
