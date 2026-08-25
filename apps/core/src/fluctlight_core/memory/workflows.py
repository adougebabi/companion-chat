"""Durable embedding workflow backed by the configured Provider role."""

from __future__ import annotations

from datetime import timedelta
from typing import Any

from temporalio import activity, workflow

_memory_service: Any | None = None
_provider_runtime: Any | None = None
_unit_of_work: Any | None = None


def configure_embedding_service(
    memory_service: Any,
    provider_runtime: Any,
    unit_of_work: Any,
) -> None:
    global _memory_service, _provider_runtime, _unit_of_work
    _memory_service = memory_service
    _provider_runtime = provider_runtime
    _unit_of_work = unit_of_work


@activity.defn(name="process_memory_embedding")
async def process_memory_embedding(payload: dict[str, Any]) -> dict[str, str]:
    from sqlalchemy import select

    from fluctlight_core.providers import schema as provider_schema

    from . import schema
    from .contracts import EmbeddingResult

    if _memory_service is None or _provider_runtime is None or _unit_of_work is None:
        raise RuntimeError("memory embedding activity is not configured")
    memory_id = str(payload["memory_id"])
    revision = int(payload["revision"])
    async with _unit_of_work.begin(
        command_id=f"memory-embedding-read:{memory_id}:{revision}"
    ) as tx:
        row = (
            (
                await tx.session.execute(
                    select(schema.memories.c.content, schema.memories.c.revision).where(
                        schema.memories.c.id == memory_id
                    )
                )
            )
            .mappings()
            .one_or_none()
        )
        model_id = await tx.session.scalar(
            select(provider_schema.model_roles.c.model_id).where(
                provider_schema.model_roles.c.role == "embedding"
            )
        )
    if row is None or model_id is None:
        raise RuntimeError("embedding memory or role is unavailable")
    model_id = str(model_id)
    if int(row["revision"]) != revision:
        await _memory_service.mark_embedding_stale(memory_id, revision, model_id)
        return {"memory_id": memory_id, "revision": str(revision), "status": "stale"}
    await _memory_service.mark_embedding_pending(memory_id, revision, model_id)
    try:
        vector = await _provider_runtime.embed(str(row["content"]))
        await _memory_service.record_embedding(
            EmbeddingResult(
                memory_id=memory_id,
                revision=revision,
                model_id=model_id,
                vector=vector,
            )
        )
    except Exception:
        await _memory_service.mark_embedding_failed(
            memory_id,
            revision,
            model_id,
            "provider_or_persistence_failure",
        )
        raise
    return {"memory_id": memory_id, "revision": str(revision), "status": "ready"}


@workflow.defn(name="MemoryEmbeddingWorkflow")
class MemoryEmbeddingWorkflow:
    @workflow.run
    async def run(self, payload: dict[str, Any]) -> dict[str, str]:
        memory_id = str(payload.get("memory_id", "")).strip()
        if not memory_id:
            raise ValueError("memory embedding workflow requires memory_id")
        return await workflow.execute_activity(
            process_memory_embedding,
            payload,
            start_to_close_timeout=timedelta(minutes=5),
        )
