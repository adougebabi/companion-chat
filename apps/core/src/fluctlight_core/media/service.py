"""Media lifecycle authority and injected Provider recovery adapter."""

from __future__ import annotations

import asyncio
import inspect
from collections.abc import Awaitable, Callable
from datetime import UTC, datetime
from typing import Any
from uuid import uuid4

from sqlalchemy import delete, insert, select, update

from fluctlight_core.platform.object_storage import (
    ObjectDescriptor,
    S3ObjectStorage,
)
from fluctlight_core.platform.outbox import (
    CommittedWorkflowIntent,
    OutboxEvent,
    add_outbox_event,
    commit_workflow_intent,
)
from fluctlight_core.platform.persistence import UnitOfWorkFactory

from . import schema
from .contracts import (
    AuthorizedMediaRead,
    MediaAsset,
    MediaIntent,
    MediaIntentStatus,
    MediaProvider,
    MediaReference,
    MediaStatus,
    MediaWorkflowResult,
)


class MediaService:
    def __init__(
        self,
        unit_of_work: UnitOfWorkFactory,
        storage: S3ObjectStorage,
        owner_authorizer: Callable[[str, str], Awaitable[bool]] | None = None,
    ) -> None:
        self._unit_of_work = unit_of_work
        self._storage = storage
        self._owner_authorizer = owner_authorizer

    async def request_generation(self, intent: MediaIntent) -> MediaIntent:
        async with self._unit_of_work.begin(command_id=f"media-intent:{intent.id}") as tx:
            existing = (
                (
                    await tx.session.execute(
                        select(schema.intents).where(schema.intents.c.id == intent.id)
                    )
                )
                .mappings()
                .one_or_none()
            )
            if existing is not None:
                if (
                    existing["owner_fluctlight_id"] != intent.owner_fluctlight_id
                    or existing["prompt"] != intent.prompt
                    or existing["provider_request_id"] != intent.provider_request_id
                    or existing["workflow_id"] != intent.workflow_id
                ):
                    raise ValueError(
                        "media intent ID was reused with different authoritative content"
                    )
                return self._intent_from_row(existing)
            await tx.session.execute(
                insert(schema.intents).values(
                    id=intent.id,
                    owner_fluctlight_id=intent.owner_fluctlight_id,
                    kind=intent.kind.value,
                    mime_type=intent.mime_type,
                    prompt=intent.prompt,
                    provider_request_id=intent.provider_request_id,
                    workflow_id=intent.workflow_id,
                    status=intent.status.value,
                    revision=intent.revision,
                    created_at=intent.created_at,
                )
            )
            await add_outbox_event(
                tx.session,
                OutboxEvent(
                    id=f"media_intent_requested_{intent.id}",
                    kind="media.intent.requested",
                    aggregate_type="media_intent",
                    aggregate_id=intent.id,
                    fluctlight_id=intent.owner_fluctlight_id,
                    causation_id=intent.id,
                    correlation_id=intent.workflow_id,
                    idempotency_key=f"media-intent:{intent.id}",
                    payload={
                        "intent_id": intent.id,
                        "provider_request_id": intent.provider_request_id,
                        "aggregate_sequence": intent.revision + 1,
                    },
                    attempt_policy={"max_attempts": 8},
                ),
            )
            await commit_workflow_intent(
                tx.session,
                CommittedWorkflowIntent(
                    intent_id=f"media_workflow_intent:{intent.id}",
                    workflow_id=intent.workflow_id,
                    task_queue="media",
                    intent_type="media.generation",
                    payload={
                        "intent_id": intent.id,
                        "provider_request_id": intent.provider_request_id,
                    },
                ),
            )
            await tx.commit()
        return intent

    async def get_intent(self, intent_id: str) -> MediaIntent:
        async with self._unit_of_work.begin(command_id=f"media-intent-read:{intent_id}") as tx:
            row = (
                (
                    await tx.session.execute(
                        select(schema.intents).where(schema.intents.c.id == intent_id)
                    )
                )
                .mappings()
                .one_or_none()
            )
        if row is None:
            raise KeyError(intent_id)
        return self._intent_from_row(row)

    @staticmethod
    def _intent_from_row(row: Any) -> MediaIntent:
        return MediaIntent(
            id=row["id"],
            owner_fluctlight_id=row["owner_fluctlight_id"],
            kind=row["kind"],
            mime_type=row["mime_type"],
            prompt=row["prompt"],
            provider_request_id=row["provider_request_id"],
            workflow_id=row["workflow_id"],
            status=row["status"],
            revision=int(row["revision"]),
            created_at=row["created_at"],
        )

    async def mark_intent_running(self, intent_id: str) -> None:
        async with self._unit_of_work.begin(command_id=f"media-intent-running:{intent_id}") as tx:
            await tx.session.execute(
                update(schema.intents)
                .where(
                    schema.intents.c.id == intent_id,
                    schema.intents.c.status == MediaIntentStatus.PENDING.value,
                )
                .values(status=MediaIntentStatus.RUNNING.value)
            )
            await tx.commit()

    async def settle_provider_output(
        self, intent: MediaIntent, *, content: bytes, content_type: str
    ) -> MediaAsset:
        descriptor = self._storage.put(
            asset_id=f"asset_{intent.id}", version="v1", content=content, content_type=content_type
        )
        return await self.record_uploaded(intent, descriptor)

    async def settle_provider_failure(self, intent_id: str, *, cancelled: bool = False) -> None:
        status = MediaIntentStatus.CANCELLED if cancelled else MediaIntentStatus.FAILED
        async with self._unit_of_work.begin(command_id=f"media-intent-settle:{intent_id}") as tx:
            await tx.session.execute(
                update(schema.intents)
                .where(
                    schema.intents.c.id == intent_id,
                    schema.intents.c.status.in_(
                        [MediaIntentStatus.PENDING.value, MediaIntentStatus.RUNNING.value]
                    ),
                )
                .values(status=status.value)
            )
            await tx.commit()

    async def record_uploaded(
        self, intent: MediaIntent, descriptor: ObjectDescriptor, *, byte_size: int | None = None
    ) -> MediaAsset:
        if descriptor.bucket != self._storage.bucket or descriptor.byte_size != (
            byte_size or descriptor.byte_size
        ):
            raise ValueError("uploaded object does not match the committed media descriptor")
        now = datetime.now(UTC)
        asset = MediaAsset(
            id=f"asset_{intent.id}",
            owner_fluctlight_id=intent.owner_fluctlight_id,
            version="v1",
            kind=intent.kind,
            mime_type=descriptor.content_type,
            byte_size=descriptor.byte_size,
            sha256=descriptor.sha256,
            bucket=descriptor.bucket,
            object_key=descriptor.key,
            object_version=descriptor.version_id,
            etag=descriptor.etag,
            provider_request_id=intent.provider_request_id,
            workflow_id=intent.workflow_id,
            status=MediaStatus.READY,
            created_at=now,
            ready_at=now,
        )
        async with self._unit_of_work.begin(command_id=f"media-uploaded:{asset.id}") as tx:
            existing = (
                (
                    await tx.session.execute(
                        select(schema.assets)
                        .where(schema.assets.c.id == asset.id)
                        .with_for_update()
                    )
                )
                .mappings()
                .one_or_none()
            )
            values = {
                "id": asset.id,
                "owner_fluctlight_id": asset.owner_fluctlight_id,
                "version": asset.version,
                "kind": asset.kind.value,
                "mime_type": asset.mime_type,
                "byte_size": asset.byte_size,
                "sha256": asset.sha256,
                "bucket": asset.bucket,
                "object_key": asset.object_key,
                "object_version": asset.object_version,
                "etag": asset.etag,
                "provider_request_id": asset.provider_request_id,
                "workflow_id": asset.workflow_id,
                "status": asset.status.value,
                "ready_at": now,
            }
            if existing is None:
                await tx.session.execute(insert(schema.assets).values(created_at=now, **values))
            elif existing["sha256"] != asset.sha256 or existing["object_key"] != asset.object_key:
                raise ValueError("media asset identity conflicts with an existing upload")
            else:
                await tx.session.execute(
                    update(schema.assets).where(schema.assets.c.id == asset.id).values(**values)
                )
            await tx.session.execute(
                update(schema.intents)
                .where(schema.intents.c.id == intent.id)
                .values(status=MediaIntentStatus.COMPLETED.value)
            )
            await tx.commit()
        return asset

    async def attach(self, reference: MediaReference, *, actor_id: str) -> MediaReference:
        async with self._unit_of_work.begin(command_id=f"media-attach:{reference.id}") as tx:
            asset = (
                (
                    await tx.session.execute(
                        select(schema.assets).where(schema.assets.c.id == reference.asset_id)
                    )
                )
                .mappings()
                .one_or_none()
            )
            if (
                asset is None
                or asset["owner_fluctlight_id"] != reference.owner_fluctlight_id
                or asset["status"] != MediaStatus.READY.value
            ):
                raise PermissionError("media asset is not attachable")
            if not await self._authorized(reference.owner_fluctlight_id, actor_id):
                raise PermissionError("media attachment authorization failed")
            await tx.session.execute(
                insert(schema.references).values(
                    id=reference.id,
                    asset_id=reference.asset_id,
                    owner_fluctlight_id=reference.owner_fluctlight_id,
                    target_type=reference.target_type,
                    target_id=reference.target_id,
                    created_at=reference.created_at,
                )
            )
            await tx.commit()
        return reference

    async def authorize_read(
        self, asset_id: str, *, actor_id: str, allowed_range: str = "full"
    ) -> AuthorizedMediaRead:
        async with self._unit_of_work.begin(
            command_id=f"media-authorize:{asset_id}:{actor_id}"
        ) as tx:
            row = (
                (
                    await tx.session.execute(
                        select(schema.assets).where(schema.assets.c.id == asset_id)
                    )
                )
                .mappings()
                .one_or_none()
            )
        if (
            row is None
            or row["status"] != MediaStatus.READY.value
            or not await self._authorized(row["owner_fluctlight_id"], actor_id)
        ):
            raise PermissionError("media asset is unavailable")
        asset = self._asset_from_row(row)
        grant = self._storage.grant_read(
            ObjectDescriptor(
                asset.bucket,
                asset.object_key,
                asset.object_version,
                asset.etag,
                asset.mime_type,
                asset.byte_size,
                asset.sha256,
            ),
            allowed_range=allowed_range,
        )
        return AuthorizedMediaRead(asset, grant)

    async def read_object(self, authorized: AuthorizedMediaRead) -> tuple[bytes, str | None]:
        return self._storage.read(authorized.grant)

    async def tombstone(self, asset_id: str, *, actor_id: str, reason: str) -> None:
        now = datetime.now(UTC)
        async with self._unit_of_work.begin(command_id=f"media-tombstone:{asset_id}") as tx:
            row = (
                (
                    await tx.session.execute(
                        select(schema.assets)
                        .where(schema.assets.c.id == asset_id)
                        .with_for_update()
                    )
                )
                .mappings()
                .one_or_none()
            )
            if row is None or row["owner_fluctlight_id"] != actor_id:
                raise PermissionError("media tombstone authorization failed")
            await tx.session.execute(
                delete(schema.references).where(schema.references.c.asset_id == asset_id)
            )
            await tx.session.execute(
                update(schema.assets)
                .where(schema.assets.c.id == asset_id)
                .values(status=MediaStatus.TOMBSTONED.value, tombstoned_at=now)
            )
            await tx.session.execute(
                insert(schema.tombstones).values(
                    id=f"tombstone_{uuid4().hex}", asset_id=asset_id, reason=reason, created_at=now
                )
            )
            await add_outbox_event(
                tx.session,
                OutboxEvent(
                    id=f"media_tombstone_{asset_id}",
                    kind="media.asset.tombstoned",
                    aggregate_type="media_asset",
                    aggregate_id=asset_id,
                    fluctlight_id=row["owner_fluctlight_id"],
                    causation_id=asset_id,
                    correlation_id=asset_id,
                    idempotency_key=f"media-tombstone:{asset_id}",
                    payload={
                        "asset_id": asset_id,
                        "aggregate_sequence": int(row["version"].lstrip("v") or "1"),
                    },
                    attempt_policy={"max_attempts": 8},
                ),
            )
            await tx.commit()

    async def record_deleted(self, asset_id: str) -> None:
        async with self._unit_of_work.begin(command_id=f"media-deleted:{asset_id}") as tx:
            await tx.session.execute(
                update(schema.assets)
                .where(
                    schema.assets.c.id == asset_id,
                    schema.assets.c.status == MediaStatus.TOMBSTONED.value,
                )
                .values(status=MediaStatus.DELETED.value, deleted_at=datetime.now(UTC))
            )
            await tx.commit()

    async def orphan_candidates(self, *, older_than: datetime) -> list[MediaAsset]:
        async with self._unit_of_work.begin(command_id="media-orphan-scan") as tx:
            rows = (
                (
                    await tx.session.execute(
                        select(schema.assets).where(
                            schema.assets.c.status.in_(
                                [MediaStatus.PENDING.value, MediaStatus.UNAVAILABLE.value]
                            ),
                            schema.assets.c.created_at < older_than,
                        )
                    )
                )
                .mappings()
                .all()
            )
        return [self._asset_from_row(row) for row in rows]

    @staticmethod
    def _asset_from_row(row: Any) -> MediaAsset:
        return MediaAsset(
            id=row["id"],
            owner_fluctlight_id=row["owner_fluctlight_id"],
            version=row["version"],
            kind=row["kind"],
            mime_type=row["mime_type"],
            byte_size=int(row["byte_size"]),
            sha256=row["sha256"],
            bucket=row["bucket"],
            object_key=row["object_key"],
            object_version=row["object_version"],
            etag=row["etag"],
            provider_request_id=row["provider_request_id"],
            workflow_id=row["workflow_id"],
            status=row["status"],
            created_at=row["created_at"],
            ready_at=row["ready_at"],
            tombstoned_at=row["tombstoned_at"],
            deleted_at=row["deleted_at"],
        )

    async def _authorized(self, owner_fluctlight_id: str, actor_id: str) -> bool:
        if self._owner_authorizer is not None:
            return await self._owner_authorizer(owner_fluctlight_id, actor_id)
        return owner_fluctlight_id == actor_id


class MediaWorkflowAdapter:
    """Provider-success/recovery seam with stable IDs and cooperative cancellation."""

    async def run(
        self,
        intent: MediaIntent,
        provider: MediaProvider,
        *,
        heartbeat: Callable[[str], Awaitable[None] | None] | None = None,
        cancelled: Callable[[], bool] | None = None,
        max_polls: int = 120,
        poll_interval_seconds: float = 1.0,
    ) -> MediaWorkflowResult:
        request_id = await provider.submit(intent)
        if request_id != intent.provider_request_id:
            raise ValueError("Provider returned an unexpected stable request ID")
        for _ in range(max_polls):
            if cancelled and cancelled():
                await provider.cancel(request_id)
                return MediaWorkflowResult(request_id, False, {"status": "cancelled"})
            if heartbeat:
                heartbeat_result = heartbeat(request_id)
                if inspect.isawaitable(heartbeat_result):
                    await heartbeat_result
            result = await provider.poll(request_id)
            if result and result.get("status") == "completed":
                return MediaWorkflowResult(request_id, True, dict(result))
            if result and result.get("status") == "failed":
                return MediaWorkflowResult(request_id, False, dict(result))
            await asyncio.sleep(poll_interval_seconds)
        return MediaWorkflowResult(
            request_id, False, {"status": "deferred", "reason": "poll_limit"}
        )
