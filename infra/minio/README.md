# Pinned MinIO source images

The public images previously fixed in Compose can return access denied. The
MinIO community project now documents source-only distribution. Build the same
fixed official source versions locally; do not substitute an unknown mirror or
an S3 mock:

```bash
python3 infra/minio/build-source-images.py --workdir /private/tmp/fluctlight-minio-build
```

This requires Go, Git, Docker and network access for the official sources and Go
modules. It verifies each fixed Git commit and a clean source checkout, compiles
for the Docker daemon architecture, retains LICENSE/CREDITS and labels each image
with source provenance. It neither starts a deployment nor modifies existing
storage. Existing work directories with changed sources are rejected.

Set these optional variables in the private deployment env after a successful
build:

```dotenv
MINIO_IMAGE=fluctlight-minio-source:RELEASE.2025-04-22T22-12-26Z
MINIO_CLIENT_IMAGE=fluctlight-mc-source:RELEASE.2025-04-16T18-13-26Z
```

Compose still supports explicitly supplied previously retained image digests.
The default pinned public image names are unchanged for deployments that retain
those images. Source versions are intentionally identical to the existing
storage release; selecting a newer MinIO version is a separate upgrade.
