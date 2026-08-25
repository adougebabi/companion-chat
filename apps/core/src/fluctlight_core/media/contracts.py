"""Private media identity, lifecycle and Provider recovery contracts."""

from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass, field
from datetime import UTC, datetime
from enum import StrEnum
from typing import Any, Protocol

from fluctlight_core.platform.object_storage import InternalObjectGrant


class MediaKind(StrEnum):
    IMAGE = "image"
    VIDEO = "video"
    AUDIO = "audio"


class MediaStatus(StrEnum):
    PENDING = "pending"
    UPLOADING = "uploading"
    READY = "ready"
    UNAVAILABLE = "unavailable"
    TOMBSTONED = "tombstoned"
    DELETED = "deleted"


class MediaIntentStatus(StrEnum):
    PENDING = "pending"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    CANCELLED = "cancelled"


def _text(value: str, name: str, limit: int = 512) -> str:
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{name} is required")
    value = value.strip()
    if len(value) > limit:
        raise ValueError(f"{name} exceeds {limit} characters")
    return value


@dataclass(frozen=True, slots=True)
class MediaIntent:
    id: str
    owner_fluctlight_id: str
    kind: MediaKind
    mime_type: str
    prompt: str
    provider_request_id: str
    workflow_id: str
    status: MediaIntentStatus = MediaIntentStatus.PENDING
    revision: int = 0
    created_at: datetime = field(default_factory=lambda: datetime.now(UTC))

    def __post_init__(self) -> None:
        for name in (
            "id",
            "owner_fluctlight_id",
            "mime_type",
            "prompt",
            "provider_request_id",
            "workflow_id",
        ):
            object.__setattr__(self, name, _text(getattr(self, name), name))
        object.__setattr__(self, "kind", MediaKind(self.kind))
        object.__setattr__(self, "status", MediaIntentStatus(self.status))
        if self.revision < 0:
            raise ValueError("media intent revision cannot be negative")


@dataclass(frozen=True, slots=True)
class MediaAsset:
    id: str
    owner_fluctlight_id: str
    version: str
    kind: MediaKind
    mime_type: str
    byte_size: int
    sha256: str
    bucket: str
    object_key: str
    object_version: str | None
    etag: str | None
    provider_request_id: str
    workflow_id: str
    status: MediaStatus = MediaStatus.PENDING
    created_at: datetime = field(default_factory=lambda: datetime.now(UTC))
    ready_at: datetime | None = None
    tombstoned_at: datetime | None = None
    deleted_at: datetime | None = None

    def __post_init__(self) -> None:
        for name in (
            "id",
            "owner_fluctlight_id",
            "version",
            "mime_type",
            "sha256",
            "bucket",
            "object_key",
            "provider_request_id",
            "workflow_id",
        ):
            object.__setattr__(self, name, _text(getattr(self, name), name))
        object.__setattr__(self, "kind", MediaKind(self.kind))
        object.__setattr__(self, "status", MediaStatus(self.status))
        if self.byte_size < 0:
            raise ValueError("media byte_size cannot be negative")


@dataclass(frozen=True, slots=True)
class MediaReference:
    id: str
    asset_id: str
    owner_fluctlight_id: str
    target_type: str
    target_id: str
    created_at: datetime = field(default_factory=lambda: datetime.now(UTC))


@dataclass(frozen=True, slots=True)
class MediaProgress:
    intent_id: str
    stage: str
    progress: float
    provider_request_id: str
    detail: str | None = None


class MediaProvider(Protocol):
    async def submit(self, intent: MediaIntent) -> str: ...
    async def poll(self, provider_request_id: str) -> Mapping[str, Any] | None: ...
    async def cancel(self, provider_request_id: str) -> None: ...


@dataclass(frozen=True, slots=True)
class MediaWorkflowResult:
    provider_request_id: str
    completed: bool
    output: Mapping[str, Any] = field(default_factory=dict)


@dataclass(frozen=True, slots=True)
class AuthorizedMediaRead:
    asset: MediaAsset
    grant: InternalObjectGrant
