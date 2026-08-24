# MinIO Platform Defaults

The Compose deployment creates the private `fluctlight-media` bucket, enables
versioning, and applies lifecycle expiry for non-current object versions. The
application uses only the S3-compatible data plane; MinIO administration stays
inside deployment setup.
