"""Authoritative Memory persistence, embedding lifecycle and bounded retrieval."""

from __future__ import annotations

from collections.abc import AsyncIterator, Callable
from contextlib import asynccontextmanager
from datetime import UTC, datetime
from typing import Any
from uuid import uuid4

from sqlalchemy import and_, bindparam, func, insert, or_, select, update

from fluctlight_core.platform.outbox import (
    CommittedWorkflowIntent,
    OutboxEvent,
    add_outbox_event,
    commit_workflow_intent,
)
from fluctlight_core.platform.persistence import UnitOfWork, UnitOfWorkFactory

from . import schema
from .contracts import (
    EmbeddingResult,
    EmbeddingStatus,
    MemoryContextItem,
    MemoryQuery,
    MemoryRecord,
    MemoryRevision,
    MemoryStatus,
    MemoryType,
    MemoryVisibility,
    cosine_similarity,
)


class MemoryService:
    def __init__(
        self, unit_of_work: UnitOfWorkFactory, *, clock: Callable[[], datetime] | None = None
    ) -> None:
        self._unit_of_work = unit_of_work
        self._clock = clock or (lambda: datetime.now(UTC))

    @asynccontextmanager
    async def _transaction(
        self, tx: UnitOfWork | None, command_id: str
    ) -> AsyncIterator[UnitOfWork]:
        if tx is not None:
            yield tx
            return
        async with self._unit_of_work.begin(command_id=command_id) as owned:
            yield owned
            await owned.commit()

    async def record(self, memory: MemoryRecord, *, tx: UnitOfWork | None = None) -> MemoryRecord:
        async with self._transaction(tx, f"memory-record:{memory.id}") as transaction:
            await transaction.session.execute(
                insert(schema.memories).values(
                    id=memory.id,
                    owner_fluctlight_id=memory.owner_fluctlight_id,
                    type=memory.type.value,
                    content=memory.content,
                    actor_refs=list(memory.actor_refs),
                    conversation_id=memory.conversation_id,
                    event_refs=list(memory.event_refs),
                    evidence_refs=list(memory.evidence_refs),
                    confidence=memory.confidence,
                    importance=memory.importance,
                    emotional_significance=memory.emotional_significance,
                    visibility=memory.visibility.value,
                    status=memory.status.value,
                    revision=memory.revision,
                    occurred_at=memory.occurred_at,
                    created_at=memory.created_at,
                    last_confirmed_at=memory.last_confirmed_at,
                )
            )
            await self._insert_revision(transaction, memory, actor_id=memory.owner_fluctlight_id)
            await add_outbox_event(
                transaction.session,
                OutboxEvent(
                    id=f"memory_embedding_requested_{memory.id}_{memory.revision}",
                    kind="memory.embedding.requested",
                    aggregate_type="memory",
                    aggregate_id=memory.id,
                    fluctlight_id=memory.owner_fluctlight_id,
                    causation_id=memory.id,
                    correlation_id=f"memory:{memory.id}",
                    idempotency_key=f"memory-embedding:{memory.id}:{memory.revision}",
                    payload={
                        "memory_id": memory.id,
                        "revision": memory.revision,
                        "aggregate_sequence": memory.revision + 1,
                    },
                    attempt_policy={"max_attempts": 8},
                ),
            )
            await commit_workflow_intent(
                transaction.session,
                CommittedWorkflowIntent(
                    intent_id=f"memory-embedding-workflow:{memory.id}:{memory.revision}",
                    workflow_id=f"memory-embedding:{memory.id}:{memory.revision}",
                    task_queue="lifecycle",
                    intent_type="memory.embedding",
                    payload={"memory_id": memory.id, "revision": memory.revision},
                ),
            )
        return memory

    async def revise(
        self,
        memory_id: str,
        *,
        expected_revision: int,
        content: str,
        actor_id: str,
        evidence_refs: tuple[str, ...],
        status: MemoryStatus = MemoryStatus.ACTIVE,
    ) -> MemoryRevision:
        if not content.strip() or not evidence_refs:
            raise ValueError("memory revision requires content and evidence")
        now = self._clock()
        async with self._unit_of_work.begin(
            command_id=f"memory-revise:{memory_id}:{expected_revision}"
        ) as tx:
            row = (
                (
                    await tx.session.execute(
                        select(schema.memories)
                        .where(schema.memories.c.id == memory_id)
                        .with_for_update()
                    )
                )
                .mappings()
                .one_or_none()
            )
            if row is None:
                raise KeyError(memory_id)
            if int(row["revision"]) != expected_revision:
                raise ValueError("memory revision is stale")
            next_revision = expected_revision + 1
            revision = MemoryRevision(
                memory_id,
                next_revision,
                expected_revision,
                content,
                MemoryStatus(status),
                actor_id,
                tuple(evidence_refs),
            )
            result = await tx.session.execute(
                update(schema.memories)
                .where(
                    schema.memories.c.id == memory_id,
                    schema.memories.c.revision == expected_revision,
                )
                .values(
                    content=content,
                    status=revision.status.value,
                    revision=next_revision,
                    evidence_refs=list(revision.evidence_refs),
                    last_confirmed_at=now,
                )
            )
            if result.rowcount != 1:
                raise ValueError("memory compare-and-set failed")
            await self._insert_revision(
                tx,
                self._memory_from_row(
                    row, content=content, revision=next_revision, status=revision.status
                ),
                actor_id=actor_id,
                revision=revision,
            )
            await tx.session.execute(
                update(schema.memory_embeddings)
                .where(schema.memory_embeddings.c.memory_id == memory_id)
                .values(status=EmbeddingStatus.STALE.value)
            )
            await add_outbox_event(
                tx.session,
                OutboxEvent(
                    id=f"memory_embedding_requested_{memory_id}_{next_revision}",
                    kind="memory.embedding.requested",
                    aggregate_type="memory",
                    aggregate_id=memory_id,
                    fluctlight_id=row["owner_fluctlight_id"],
                    causation_id=revision.idempotency_key,
                    correlation_id=f"memory:{memory_id}",
                    idempotency_key=f"memory-embedding:{memory_id}:{next_revision}",
                    payload={
                        "memory_id": memory_id,
                        "revision": next_revision,
                        "aggregate_sequence": next_revision + 1,
                    },
                    attempt_policy={"max_attempts": 8},
                ),
            )
            await commit_workflow_intent(
                tx.session,
                CommittedWorkflowIntent(
                    intent_id=f"memory-embedding-workflow:{memory_id}:{next_revision}",
                    workflow_id=f"memory-embedding:{memory_id}:{next_revision}",
                    task_queue="lifecycle",
                    intent_type="memory.embedding",
                    payload={"memory_id": memory_id, "revision": next_revision},
                ),
            )
            await tx.commit()
        return revision

    async def forget(
        self,
        memory_id: str,
        *,
        expected_revision: int,
        actor_id: str,
        evidence_refs: tuple[str, ...],
    ) -> MemoryRevision:
        async with self._unit_of_work.begin(command_id=f"memory-forget:{memory_id}") as tx:
            row = (
                (
                    await tx.session.execute(
                        select(schema.memories)
                        .where(schema.memories.c.id == memory_id)
                        .with_for_update()
                    )
                )
                .mappings()
                .one_or_none()
            )
            if row is None:
                raise KeyError(memory_id)
            if int(row["revision"]) != expected_revision:
                raise ValueError("memory revision is stale")
            revision = await self._revise_in_tx(
                tx,
                row,
                expected_revision=expected_revision,
                content=row["content"],
                actor_id=actor_id,
                evidence_refs=evidence_refs,
                status=MemoryStatus.FORGOTTEN,
            )
            await tx.commit()
        return revision

    async def record_embedding(self, result: EmbeddingResult) -> None:
        if result.status not in {
            EmbeddingStatus.READY,
            EmbeddingStatus.FAILED,
            EmbeddingStatus.STALE,
            EmbeddingStatus.PENDING,
        }:
            raise ValueError("unsupported embedding status")
        async with self._unit_of_work.begin(
            command_id=f"memory-embed:{result.memory_id}:{result.revision}"
        ) as tx:
            memory = (
                (
                    await tx.session.execute(
                        select(schema.memories).where(schema.memories.c.id == result.memory_id)
                    )
                )
                .mappings()
                .one_or_none()
            )
            if memory is None:
                raise KeyError(result.memory_id)
            if int(memory["revision"]) != result.revision:
                raise ValueError("embedding result targets a stale Memory revision")
            existing = (
                (
                    await tx.session.execute(
                        select(schema.memory_embeddings).where(
                            schema.memory_embeddings.c.memory_id == result.memory_id,
                            schema.memory_embeddings.c.memory_revision == result.revision,
                            schema.memory_embeddings.c.model_id == result.model_id,
                        )
                    )
                )
                .mappings()
                .one_or_none()
            )
            model_dimensions = await tx.session.scalar(
                select(func.max(schema.memory_embeddings.c.dimensions)).where(
                    schema.memory_embeddings.c.model_id == result.model_id,
                    schema.memory_embeddings.c.dimensions > 0,
                )
            )
            if model_dimensions is not None and int(model_dimensions) != len(result.vector):
                raise ValueError("embedding dimensions do not match the model index")
            values = {
                "memory_id": result.memory_id,
                "memory_revision": result.revision,
                "model_id": result.model_id,
                "dimensions": len(result.vector),
                "embedding": list(result.vector),
                "embedding_vector": list(result.vector),
                "status": result.status.value,
                "error_code": result.error_code,
                "embedded_at": self._clock() if result.status is EmbeddingStatus.READY else None,
            }
            if existing is None:
                await tx.session.execute(
                    insert(schema.memory_embeddings).values(id=f"embedding_{uuid4().hex}", **values)
                )
            else:
                if int(existing["dimensions"]) not in {0, len(result.vector)}:
                    raise ValueError("embedding dimensions do not match existing model row")
                await tx.session.execute(
                    update(schema.memory_embeddings)
                    .where(schema.memory_embeddings.c.id == existing["id"])
                    .values(**values)
                )
            await tx.commit()

    async def mark_embedding_pending(self, memory_id: str, revision: int, model_id: str) -> None:
        await self._set_embedding_status(
            memory_id,
            revision,
            model_id,
            status=EmbeddingStatus.PENDING,
            error_code=None,
        )

    async def mark_embedding_failed(
        self, memory_id: str, revision: int, model_id: str, error_code: str
    ) -> None:
        await self._set_embedding_status(
            memory_id,
            revision,
            model_id,
            status=EmbeddingStatus.FAILED,
            error_code=error_code,
        )

    async def mark_embedding_stale(self, memory_id: str, revision: int, model_id: str) -> None:
        await self._set_embedding_status(
            memory_id,
            revision,
            model_id,
            status=EmbeddingStatus.STALE,
            error_code="stale_revision",
        )

    async def _set_embedding_status(
        self,
        memory_id: str,
        revision: int,
        model_id: str,
        *,
        status: EmbeddingStatus,
        error_code: str | None,
    ) -> None:
        async with self._unit_of_work.begin(
            command_id=f"memory-embed-failed:{memory_id}:{revision}"
        ) as tx:
            row = (
                (
                    await tx.session.execute(
                        select(schema.memory_embeddings).where(
                            schema.memory_embeddings.c.memory_id == memory_id,
                            schema.memory_embeddings.c.memory_revision == revision,
                            schema.memory_embeddings.c.model_id == model_id,
                        )
                    )
                )
                .mappings()
                .one_or_none()
            )
            if row is None:
                await tx.session.execute(
                    insert(schema.memory_embeddings).values(
                        id=f"embedding_{uuid4().hex}",
                        memory_id=memory_id,
                        memory_revision=revision,
                        model_id=model_id,
                        dimensions=0,
                        embedding=[],
                        embedding_vector=None,
                        status=status.value,
                        error_code=error_code,
                    )
                )
            else:
                await tx.session.execute(
                    update(schema.memory_embeddings)
                    .where(schema.memory_embeddings.c.id == row["id"])
                    .values(status=status.value, error_code=error_code)
                )
            await tx.commit()

    async def retrieve(self, query: MemoryQuery) -> list[MemoryRecord]:
        async with self._unit_of_work.begin(
            command_id=f"memory-retrieve:{query.owner_fluctlight_id}"
        ) as tx:
            conditions: list[Any] = [
                schema.memories.c.owner_fluctlight_id == query.owner_fluctlight_id,
                schema.memories.c.status.in_([status.value for status in query.statuses]),
            ]
            if query.allowed_types:
                conditions.append(
                    schema.memories.c.type.in_([item.value for item in query.allowed_types])
                )
            if query.conversation_scope:
                conditions.append(schema.memories.c.conversation_id == query.conversation_scope)
            actor_scope = or_(
                *[
                    schema.memories.c.actor_refs.contains([actor_id])
                    for actor_id in query.authorized_actor_ids
                ]
            )
            owner_scope = schema.memories.c.owner_fluctlight_id.in_(query.authorized_actor_ids)
            conditions.append(
                or_(
                    and_(
                        schema.memories.c.visibility == MemoryVisibility.PRIVATE.value,
                        owner_scope,
                    ),
                    and_(
                        schema.memories.c.visibility == MemoryVisibility.OWNER.value,
                        or_(owner_scope, actor_scope),
                    ),
                    and_(
                        schema.memories.c.visibility == MemoryVisibility.PARTICIPANTS.value,
                        actor_scope,
                    ),
                )
            )
            statement = select(schema.memories).where(*conditions)
            if query.query_text:
                search_query = func.plainto_tsquery("simple", query.query_text)
                search_rank = func.ts_rank_cd(
                    schema.memories.c.search_document, search_query
                ).label("_search_rank")
                statement = statement.add_columns(search_rank).where(
                    schema.memories.c.search_document.op("@@")(search_query)
                )
            rows = (await tx.session.execute(statement)).mappings().all()
            memories = [self._memory_from_row(row) for row in rows]
            embedding_statement = select(schema.memory_embeddings).where(
                schema.memory_embeddings.c.memory_id.in_([memory.id for memory in memories]),
                schema.memory_embeddings.c.status == EmbeddingStatus.READY.value,
                schema.memory_embeddings.c.model_id == query.embedding_model_id
                if query.embedding_model_id
                else True,
            )
            if query.query_embedding:
                embedding_statement = embedding_statement.where(
                    schema.memory_embeddings.c.dimensions == len(query.query_embedding),
                    schema.memory_embeddings.c.embedding_vector.is_not(None),
                )
                vector_parameter = bindparam(
                    "query_embedding",
                    value=list(query.query_embedding),
                    type_=schema.PgVector(),
                )
                distance = schema.memory_embeddings.c.embedding_vector.op("<=>")(
                    vector_parameter
                ).label("_vector_distance")
                embedding_statement = embedding_statement.add_columns(distance)
            embeddings = (
                (await tx.session.execute(embedding_statement)).mappings().all() if memories else []
            )
        search_ranks = {
            row["id"]: float(row.get("_search_rank") or 0.0)
            for row in rows
            if row.get("_search_rank") is not None
        }
        vectors = {
            row["memory_id"]: (
                tuple(row["embedding_vector"] or row["embedding"] or ()),
                float(row["_vector_distance"]) if row.get("_vector_distance") is not None else None,
            )
            for row in embeddings
        }
        scored: list[tuple[float, MemoryRecord]] = []
        for memory in memories:
            score = memory.importance + memory.emotional_significance * 0.5
            score += search_ranks.get(memory.id, 0.0)
            vector, distance = vectors.get(memory.id, ((), None))
            if query.query_embedding and vector and distance is not None:
                score += max(0.0, 1.0 - distance)
            elif query.query_embedding and vector and len(vector) == len(query.query_embedding):
                score += cosine_similarity(query.query_embedding, vector)
            scored.append((score, memory))
        scored.sort(key=lambda item: (item[0], item[1].created_at), reverse=True)
        return [memory for _, memory in scored[: query.limit]]

    async def prompt_context(self, query: MemoryQuery) -> list[MemoryContextItem]:
        if any(status is not MemoryStatus.ACTIVE for status in query.statuses):
            raise ValueError("prompt context only accepts active Memory status")
        results = await self.retrieve(query)
        budget = query.token_budget
        context: list[MemoryContextItem] = []
        for memory in results:
            estimated = max(1, len(memory.content.split()))
            if estimated > budget:
                continue
            context.append(
                MemoryContextItem(
                    id=memory.id,
                    type=memory.type,
                    content=memory.content,
                    confidence=memory.confidence,
                    evidence_refs=memory.evidence_refs,
                    source_refs=memory.event_refs,
                    revision=memory.revision,
                )
            )
            budget -= estimated
        return context

    async def _revise_in_tx(
        self,
        tx: UnitOfWork,
        row: Any,
        *,
        expected_revision: int,
        content: str,
        actor_id: str,
        evidence_refs: tuple[str, ...],
        status: MemoryStatus,
    ) -> MemoryRevision:
        next_revision = expected_revision + 1
        revision = MemoryRevision(
            row["id"], next_revision, expected_revision, content, status, actor_id, evidence_refs
        )
        await tx.session.execute(
            update(schema.memories)
            .where(
                schema.memories.c.id == row["id"], schema.memories.c.revision == expected_revision
            )
            .values(
                content=content,
                status=status.value,
                revision=next_revision,
                evidence_refs=list(evidence_refs),
                last_confirmed_at=self._clock(),
            )
        )
        await self._insert_revision(
            tx,
            self._memory_from_row(row, content=content, revision=next_revision, status=status),
            actor_id=actor_id,
            revision=revision,
        )
        await tx.session.execute(
            update(schema.memory_embeddings)
            .where(schema.memory_embeddings.c.memory_id == row["id"])
            .values(status=EmbeddingStatus.STALE.value)
        )
        return revision

    async def _insert_revision(
        self,
        tx: UnitOfWork,
        memory: MemoryRecord,
        *,
        actor_id: str,
        revision: MemoryRevision | None = None,
    ) -> None:
        revision = revision or MemoryRevision(
            memory.id,
            memory.revision,
            max(memory.revision - 1, 0),
            memory.content,
            memory.status,
            actor_id,
            memory.evidence_refs,
        )
        await tx.session.execute(
            insert(schema.memory_revisions).values(
                id=f"memory_revision_{uuid4().hex}",
                memory_id=revision.memory_id,
                revision=revision.revision,
                base_revision=revision.base_revision,
                content=revision.content,
                status=revision.status.value,
                actor_id=revision.actor_id,
                evidence_refs=list(revision.evidence_refs),
                idempotency_key=revision.idempotency_key,
                created_at=revision.created_at,
            )
        )

    @staticmethod
    def _memory_from_row(
        row: Any,
        *,
        content: str | None = None,
        revision: int | None = None,
        status: MemoryStatus | None = None,
    ) -> MemoryRecord:
        return MemoryRecord(
            id=row["id"],
            owner_fluctlight_id=row["owner_fluctlight_id"],
            type=MemoryType(row["type"]),
            content=content if content is not None else row["content"],
            actor_refs=tuple(row["actor_refs"] or ()),
            conversation_id=row["conversation_id"],
            event_refs=tuple(row["event_refs"] or ()),
            evidence_refs=tuple(row["evidence_refs"] or ()),
            confidence=float(row["confidence"]),
            importance=float(row["importance"]),
            emotional_significance=float(row["emotional_significance"]),
            visibility=MemoryVisibility(row["visibility"]),
            status=status or MemoryStatus(row["status"]),
            revision=int(revision if revision is not None else row["revision"]),
            occurred_at=row["occurred_at"],
            created_at=row["created_at"],
            last_confirmed_at=row["last_confirmed_at"],
        )
