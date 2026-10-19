# Fluctlight business backup and isolated recovery

The current implementation is `maintenance.py` with the Go `fluctlight-backupctl-go`
composition root in the Core image. It does not initialize Agents, Providers,
Temporal workers or schedules. The older `fluctlight-backup manifest/verify`
commands are not a current executable dependency.

The v3 data tools page (`#/data`) creates durable Owner maintenance jobs and reads
actual stages/results. `queued` means the local maintenance operator has not yet
executed the job. The browser cannot run host commands, read deployment credentials
or retrieve the private archive. A completed job requires actual artifact,
database-count and media-byte checks. Source code/builds do not count as an
executed backup or recovery drill.

## Prerequisites and strategy

Run the utility **on the deployment host**, with Python 3, Docker Compose and
permission to stop the selected deployment. Build the current Core image first:
it must contain `/usr/local/bin/fluctlight-backupctl-go`. Keep the original private
Compose env file and operator work directory under owner-only permissions
(`chmod 600` for the env file, `chmod 700` for the directory). Never commit either.

This implementation supports the repository's local Compose PostgreSQL/MinIO
volume topology. It verifies that the resolved configuration matches the actual
Core container. External database/storage deployments are rejected rather than
being represented by an unrelated local volume copy. Same-host maintenance runs
are serialized by Docker-daemon/project identity.

The utility stops API, Worker, Web, Temporal and Redis writers. While PostgreSQL
and MinIO remain available it checks all tables in the application, Temporal and
visibility databases, verifies that the deployment settings key can decrypt the
actual encrypted settings, and reads every ready media object's exact version and
SHA-256. Unversioned legacy objects are explicitly counted, not assigned invented
versions. It then cleanly stops the remaining services and archives:

- `fluctlight_postgres`: the whole cluster, including all three databases;
- `fluctlight_minio`: original objects and version metadata;
- Redis and both Core/Worker KEV spool volumes;
- exact deployed container images, the private Compose env and resolved runtime
  env, and the complete source database/media probe.

The manifest contains artifact sizes, SHA-256, volume member counts, actual
migration head, image identities, checks and field-presence metadata. It contains
no credential values. The private env and PostgreSQL archive do contain deployment
secrets or encrypted settings and remain private. Symlinks, external tablespaces,
special files, partial artifacts and checksum mismatches fail explicitly.

A cleanly stopped physical volume snapshot preserves PostgreSQL/Temporal history
and MinIO object version IDs together. Re-uploading S3 objects into another bucket
would create different version IDs and invalidate frozen grants; this utility does
not perform that substitution. Provider-side in-flight LLM/Comfy state cannot be
snapshotted. Original durable workflow/request/provider-job identities are
preserved; the restore tool does not execute those jobs.

## Create and execute a backup

1. Open `#/data`, choose **创建业务备份维护任务**, enter a reason and submit.
2. Open the resulting stable task page and copy its real `backup_...` ID.
3. Run from the project directory, replacing the paths and ID:

```bash
python3 infra/backup/maintenance.py \
  --job backup_REAL_ID \
  --compose infra/compose/fluctlight.compose.yml \
  --env-file /private/deployment/fluctlight.env \
  --workdir /private/backups/fluctlight \
  --maintenance-confirm
```

The task identity reaches a private execution file before database admission.
Output is stage codes and the final manifest SHA, with no subprocess credential
output. The operator journal and each attempt's artifacts remain in the private
work directory. Interrupted runs must keep this directory and use the same
command to reconcile the original attempt. A complete manifest can settle a lost
final receipt without taking another snapshot. An incomplete attempt fails and
must be explicitly retried from the task page; it is never silently overwritten.

After backup, only source PostgreSQL is started to save the durable receipt. The
utility does not automatically start source API/Worker or model execution.
Resume the source deployment through its ordinary deployment procedure when the
operator is ready, then refresh the v3 task page. Preserve the printed manifest
SHA separately from the archive for transport/disaster integrity checking.

## Restore an existing backup into a new isolated target

From the completed backup task, choose **创建全新隔离恢复任务**, confirm a fresh
`fluctlight_restore_...` target and submit a reason. Run the same utility with the
new `restore_...` job ID and the original operator directory. Existing target
volumes or network are rejected before any target is created. Failed restores
retain partial targets for inspection; a retry must explicitly select a new name.

The utility verifies all archive digests, loads the archived images, creates only
new volumes plus an internal Docker network, and starts isolated PostgreSQL and
MinIO without published ports. It compares the entire database/media probe to the
source snapshot **before upgrading**, then runs the migration command from the explicitly supplied `--core-image`
(or, if omitted, the actually deployed source Core image) against the isolated
application database. The actual resulting head is recorded; an older deployed
image is not represented as a newer release. It records a local target
recovery audit and resolves copied source maintenance leases without restarting
business workflows. Verification containers are stopped afterwards. Target
volumes, network and private result files remain available for operator inspection.

The live source databases, volumes, assets and service state are not replaced by
this restore path. No Temporal/API/Worker is automatically started in the clone.
Actual replay/resume involving model work remains a separately recorded acceptance
operation; database counts alone do not prove model execution succeeded.

## Recover when the original deployment is unavailable

A complete private artifact bundle and its separately retained manifest SHA can
also be recovered without a source database or browser job. Supply a locally
available **current-release Core image** for verification and upgrade:

```bash
python3 infra/backup/maintenance.py \
  --bundle /private/recovered-bundle/backup_REAL_ID-attempt-1 \
  --manifest-sha256 THE_RECORDED_64_HEX_DIGEST \
  --core-image CURRENT_RELEASE_CORE_IMAGE \
  --target-name fluctlight_restore_a1b2c3d4e5f6 \
  --workdir /private/disaster-recovery \
  --maintenance-confirm
```

This path has the same create-only isolated target rules, real checks and stopped
verification containers. Its receipt is stored in the private target directory,
and its actual recovery audit is written only into the isolated database after
verification and migration. It does not claim to have created a job in an absent
source database.

## Compatibility and evidence

The artifact schema is `fluctlight.business-backup.v1`. The exact source images
are archived, while the supplied `--core-image` (or explicitly documented deployed default)
performs the target upgrade.
A rejected migration retains the isolated pre-upgrade data and reports failure;
no user database is reset. Application/Temporal IDs and domain histories are not
renumbered or inferred. If a deployment settings key is lost or wrong, preserve
the data and re-enter Provider secrets through the actual settings flow; the tool
does not reconstruct secret values.

`recovery-drill.sh` is now an executable wrapper for a previously created restore
job, limited to an explicitly temporary work directory. It does not print a list
of pretend recovery steps. A drill is passed only when an actual result reports
matching counts/media, successful target migration and the actual manifest/target
identity. Source builds, syntax checks and compile-only tests are separate evidence.

Code-image rollback, isolated database recovery and character timeline rollback
are different operations. The application has no character timeline rollback
button. Cleanup of retained targets is an explicit operator action against the
listed `fluctlight.restore` labels/new target names; this utility never deletes
source or recovery volumes automatically.
