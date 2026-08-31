# MinIO Platform Defaults

The Compose deployment creates the private `fluctlight-media` bucket, enables
versioning, and applies lifecycle expiry for non-current object versions. The
application uses only the S3-compatible data plane; MinIO administration stays
inside deployment setup.

Backup operators must export the versioned private bucket into the T11 backup
manifest with object key, version ID, byte size and sampled SHA-256 values.
Restore the bucket into a disposable MinIO volume first and verify the manifest
before marking PostgreSQL media assets ready. Browser traffic never receives a
bucket URL or credential; the Go Core issues the short-lived internal media
authorization used by the BFF proxy.
