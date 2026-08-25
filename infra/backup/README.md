# Fluctlight Backup And Recovery

The authoritative backup is a single operator manifest plus the component
artifacts it names:

1. PostgreSQL snapshot covering the application database and the Temporal
   `temporal` and `temporal_visibility` databases.
2. The private S3/MinIO bucket with object version IDs, byte counts and sampled
   SHA-256 results.
3. The deployment `.env` file stored under the NAS operator's private file
   permissions. The manifest records field presence only and never stores
   credentials or `FLUCTLIGHT_SETTINGS_KEY` plaintext.

Create a manifest after the PostgreSQL/object snapshots exist:

```bash
fluctlight-backup manifest /backup/fluctlight/manifest.json
```

The command is intentionally conservative: operators must supply the snapshot
identity and object inventory in the manifest review before calling it
restorable. Verify the artifact before restore:

```bash
fluctlight-backup verify /backup/fluctlight/manifest.json
```

Restore into disposable PostgreSQL/MinIO/Temporal volumes first. Apply the
application Alembic upgrade explicitly, then boot the restored Temporal
default/visibility stores and Worker. The API and Worker only verify the
deployed migration head; they never run migrations automatically.

If `FLUCTLIGHT_SETTINGS_KEY` is lost, keep PostgreSQL/object data and re-enter
Provider secrets through the Owner Settings flow. Do not decrypt, copy or
reconstruct secrets from old SQLite data or old environment variables.

Cleanup is a dry-run-first operation: diagnostics retention may remove only
diagnostic rows, while tombstoned media is physically removed only after
reference invalidation and checksum/version verification. Domain revisions,
evidence and governance history are never part of diagnostic cleanup.
