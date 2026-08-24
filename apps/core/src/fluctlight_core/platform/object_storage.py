"""Private S3-compatible object adapter and internal proxy-grant fixture."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from hashlib import sha256
from typing import Any
from uuid import uuid4


@dataclass(frozen=True, slots=True)
class ObjectDescriptor:
    bucket: str
    key: str
    version_id: str | None
    etag: str | None
    content_type: str
    byte_size: int
    sha256: str


@dataclass(frozen=True, slots=True)
class InternalObjectGrant:
    grant_id: str
    descriptor: ObjectDescriptor
    expires_at: datetime
    allowed_range: str
    request: dict[str, Any]

    def is_expired(self, now: datetime | None = None) -> bool:
        return (now or datetime.now(UTC)) >= self.expires_at


class S3ObjectStorage:
    """Uses only the standard S3 client surface; no MinIO-specific calls."""

    def __init__(self, client: Any, bucket: str) -> None:
        self.client = client
        self.bucket = bucket

    def put(
        self, *, asset_id: str, version: str, content: bytes, content_type: str
    ) -> ObjectDescriptor:
        key = f"media/{asset_id}/{version}"
        digest = sha256(content).hexdigest()
        response = self.client.put_object(
            Bucket=self.bucket,
            Key=key,
            Body=content,
            ContentType=content_type,
            Metadata={"sha256": digest},
        )
        return ObjectDescriptor(
            bucket=self.bucket,
            key=key,
            version_id=response.get("VersionId"),
            etag=response.get("ETag"),
            content_type=content_type,
            byte_size=len(content),
            sha256=digest,
        )

    def grant_read(
        self, descriptor: ObjectDescriptor, *, allowed_range: str, ttl_seconds: int = 60
    ) -> InternalObjectGrant:
        if ttl_seconds < 1 or ttl_seconds > 300:
            raise ValueError("object grant ttl must be between 1 and 300 seconds")
        request = {"Bucket": descriptor.bucket, "Key": descriptor.key}
        if descriptor.version_id:
            request["VersionId"] = descriptor.version_id
        return InternalObjectGrant(
            grant_id=str(uuid4()),
            descriptor=descriptor,
            expires_at=datetime.now(UTC) + timedelta(seconds=ttl_seconds),
            allowed_range=allowed_range,
            request=request,
        )
