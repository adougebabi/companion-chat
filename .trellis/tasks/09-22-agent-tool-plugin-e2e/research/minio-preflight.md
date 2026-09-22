# Isolated MinIO preflight

Compose up --wait returned 1 because minio-init is an exited one-shot. Inspected only this task project: minio running/healthy, minio-init exited with code0; initialized private bucket is ready. Configuration loaded to private test env; no values logged.
