# Fluctlight Media Contract

## Scenario: Private S3-Compatible Media With Durable Lifecycle

### 1. Scope / Trigger

- Trigger: the clean-start system generates, uploads, attaches, reads, proxies, versions, tombstones, deletes, backs up, or restores image/video/audio media.
- Python `media` owns business identity and authorization. Object storage owns bytes. Node BFF is an authorized transport proxy, not a media-state owner.
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

`InternalMediaGrant` contains asset/version identity, authorized range policy, short expiry, content metadata, and an internal presigned/object request that is not returned directly to the browser in the default NAS mode.

### 3. Contracts

- Buckets are private. Browser requests use the Node BFF media endpoint; Python performs Actor/Conversation/reference authorization before issuing a short-lived internal grant.
- Node may proxy bytes, Range, ETag, Content-Type, Content-Length, and cache headers from the grant. It cannot infer authorization, query media tables, or mint object grants.
- Application code uses only the S3-compatible interface. MinIO-specific administration remains deployment tooling.
- Object keys are stable generated identities such as `media/{asset_id}/{object_version}` and never user-controlled filenames or local absolute paths.
- PostgreSQL records SHA-256 and byte size; ETag alone is not a content-integrity guarantee.
- Generation/upload happens after a committed media intent. The final transaction validates workflow ID, asset revision, checksum, size, and references before marking ready.
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
| Backup lacks matching database/object manifest verification | Backup is incomplete and cannot be marked restorable. |

### 5. Good / Base / Bad Cases

- Good: a private video request is authorized by Python, Node proxies a valid byte range, and the browser can seek without seeing bucket credentials.
- Good: a deleted Message removes its final media reference, commits a tombstone, and retries physical deletion after an object outage.
- Base: an uploaded object exists before its result transaction; retry finds and verifies the same stable key, then marks one asset ready.
- Bad: save absolute Provider paths, expose a public bucket, trust ETag as SHA-256, delete an object before removing references, or let Node query authorization tables.

### 6. Tests Required

- Media-intent/reference transaction tests for rollback, stable IDs, ownership, and outbox atomicity.
- Upload/recovery tests for checksum/size mismatch, duplicate upload, success-before-crash, orphan collection, and idempotent result commit.
- Authorization tests across Actor, Conversation, Message, Moment, and tombstoned/deleted states.
- BFF proxy tests for internal grant expiry, Range, ETag, MIME, cache headers, stream abort, unavailable object, and no leaked bucket credentials.
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
