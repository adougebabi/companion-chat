"""Cognitive inbox orchestration and frozen-action persistence."""

from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator, Awaitable, Callable
from contextlib import asynccontextmanager
from dataclasses import replace
from datetime import UTC, datetime
from hashlib import sha256
from typing import Any

from sqlalchemy import insert, select, update

from fluctlight_core.platform.outbox import (
    CommittedWorkflowIntent,
    OutboxEvent,
    add_outbox_event,
    commit_workflow_intent,
)
from fluctlight_core.platform.persistence import UnitOfWork, UnitOfWorkFactory

from . import schema
from .contracts import (
    ActionStatus,
    ActionType,
    AssessmentEnvelope,
    AssessmentProvider,
    CognitionConflictError,
    CognitionFact,
    EnqueuedFact,
    FrozenAction,
    InboxClaim,
    InboxStatus,
    ProcessOutcome,
    ProviderExecutionError,
    RealizationProvider,
    RealizationResult,
    ReflectionApplier,
    ReflectionProposal,
    ReflectionProvider,
    ReflectionWindow,
    StateApplier,
    lease_expiry,
    stable_action_id,
    stable_provider_request_id,
)


class CognitionService:
    """One ordered writer per Fluctlight, with external calls outside commits."""

    def __init__(
        self,
        unit_of_work: UnitOfWorkFactory,
        assessment_provider: AssessmentProvider,
        realization_provider: RealizationProvider,
        *,
        reflection_provider: ReflectionProvider | None = None,
        reflection_applier: ReflectionApplier | None = None,
        state_applier: StateApplier | None = None,
        autonomy_freezer: Callable[[FrozenAction], Awaitable[None]] | None = None,
        diagnostics: Any | None = None,
        clock: Callable[[], datetime] | None = None,
        lease_seconds: int = 30,
    ) -> None:
        self._unit_of_work = unit_of_work
        self._assessment_provider = assessment_provider
        self._realization_provider = realization_provider
        self._reflection_provider = reflection_provider
        self._reflection_applier = reflection_applier
        self._state_applier = state_applier
        self._autonomy_freezer = autonomy_freezer
        self._diagnostics = diagnostics
        self._clock = clock or (lambda: datetime.now(UTC))
        self._lease_seconds = lease_seconds

    @staticmethod
    def _lease_is_active(head: Any, now: datetime) -> bool:
        lease_until = head["writer_lease_until"]
        return bool(head["writer_owner"] and lease_until is not None and lease_until > now)

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

    async def enqueue(self, fact: CognitionFact, *, tx: UnitOfWork | None = None) -> EnqueuedFact:
        """Append a fact and assign a monotonic sequence in its Fluctlight inbox."""

        async with self._transaction(tx, f"cognition-enqueue:{fact.id}") as transaction:
            existing = (
                (
                    await transaction.session.execute(
                        select(schema.inbox).where(schema.inbox.c.id == fact.id)
                    )
                )
                .mappings()
                .one_or_none()
            )
            if existing is not None:
                if existing["idempotency_key"] != fact.idempotency_key or existing[
                    "payload"
                ] != dict(fact.payload):
                    raise CognitionConflictError(
                        "inbox event id was reused with a different payload"
                    )
                return EnqueuedFact(
                    fact, int(existing["sequence"]), InboxStatus(existing["status"])
                )

            duplicate = (
                (
                    await transaction.session.execute(
                        select(schema.inbox).where(
                            schema.inbox.c.idempotency_key == fact.idempotency_key
                        )
                    )
                )
                .mappings()
                .one_or_none()
            )
            if duplicate is not None:
                if duplicate["id"] != fact.id:
                    raise CognitionConflictError("inbox idempotency key was reused")
                return EnqueuedFact(
                    fact, int(duplicate["sequence"]), InboxStatus(duplicate["status"])
                )

            head = (
                (
                    await transaction.session.execute(
                        select(schema.inbox_heads)
                        .where(schema.inbox_heads.c.fluctlight_id == fact.fluctlight_id)
                        .with_for_update()
                    )
                )
                .mappings()
                .one_or_none()
            )
            if head is None:
                sequence = 1
                await transaction.session.execute(
                    insert(schema.inbox_heads).values(
                        fluctlight_id=fact.fluctlight_id,
                        next_sequence=2,
                        last_processed_sequence=0,
                    )
                )
            else:
                sequence = int(head["next_sequence"])
                await transaction.session.execute(
                    update(schema.inbox_heads)
                    .where(schema.inbox_heads.c.fluctlight_id == fact.fluctlight_id)
                    .values(next_sequence=sequence + 1)
                )
            await transaction.session.execute(
                insert(schema.inbox).values(
                    id=fact.id,
                    fluctlight_id=fact.fluctlight_id,
                    sequence=sequence,
                    event_type=fact.event_type,
                    payload=dict(fact.payload),
                    causation_id=fact.causation_id,
                    correlation_id=fact.correlation_id,
                    idempotency_key=fact.idempotency_key,
                    occurred_at=fact.occurred_at,
                    status=InboxStatus.PENDING.value,
                )
            )
            await add_outbox_event(
                transaction.session,
                OutboxEvent(
                    id=f"cognition_event_{fact.id}",
                    kind="cognition.inbox.accepted",
                    aggregate_type="cognition_inbox",
                    aggregate_id=fact.fluctlight_id,
                    fluctlight_id=fact.fluctlight_id,
                    causation_id=fact.causation_id,
                    correlation_id=fact.correlation_id,
                    idempotency_key=f"cognition-inbox:{fact.id}",
                    payload={
                        "event_id": fact.id,
                        "sequence": sequence,
                        "aggregate_sequence": sequence,
                        "event_type": fact.event_type,
                    },
                    attempt_policy={"max_attempts": 8},
                ),
            )
        return EnqueuedFact(fact, sequence)

    async def recent_history(
        self, fluctlight_id: str, *, limit: int = 20
    ) -> list[dict[str, object]]:
        bounded = min(max(limit, 1), 50)
        async with self._unit_of_work.begin(command_id=f"cognition-history:{fluctlight_id}") as tx:
            rows = (
                (
                    await tx.session.execute(
                        select(schema.frozen_actions)
                        .where(schema.frozen_actions.c.fluctlight_id == fluctlight_id)
                        .order_by(schema.frozen_actions.c.frozen_at.desc())
                        .limit(bounded)
                    )
                )
                .mappings()
                .all()
            )
        return [
            {
                "id": row["id"],
                "action_type": row["action_type"],
                "status": row["status"],
                "error_code": row["error_code"],
                "frozen_at": row["frozen_at"].isoformat(),
                "completed_at": row["completed_at"].isoformat() if row["completed_at"] else None,
            }
            for row in rows
        ]

    async def claim_next(self, fluctlight_id: str, *, worker_id: str) -> InboxClaim | None:
        now = self._clock()
        async with self._unit_of_work.begin(command_id=f"cognition-claim:{fluctlight_id}") as tx:
            head = (
                (
                    await tx.session.execute(
                        select(schema.inbox_heads)
                        .where(schema.inbox_heads.c.fluctlight_id == fluctlight_id)
                        .with_for_update()
                    )
                )
                .mappings()
                .one_or_none()
            )
            if head is None:
                return None
            if self._lease_is_active(head, now):
                return None
            next_sequence = int(head["last_processed_sequence"]) + 1
            row = (
                (
                    await tx.session.execute(
                        select(schema.inbox)
                        .where(
                            schema.inbox.c.fluctlight_id == fluctlight_id,
                            schema.inbox.c.sequence == next_sequence,
                            schema.inbox.c.status.in_(
                                [
                                    InboxStatus.PENDING.value,
                                    InboxStatus.CLAIMED.value,
                                    InboxStatus.FROZEN.value,
                                ]
                            ),
                        )
                        .with_for_update()
                    )
                )
                .mappings()
                .one_or_none()
            )
            if row is None:
                return None
            await tx.session.execute(
                update(schema.inbox)
                .where(schema.inbox.c.id == row["id"])
                .values(
                    status=InboxStatus.CLAIMED.value,
                    attempt_count=int(row["attempt_count"]) + 1,
                    claimed_by=worker_id,
                    claimed_at=now,
                )
            )
            await tx.session.execute(
                update(schema.inbox_heads)
                .where(schema.inbox_heads.c.fluctlight_id == fluctlight_id)
                .values(
                    writer_owner=worker_id, writer_lease_until=lease_expiry(self._lease_seconds)
                )
            )
            await tx.commit()
        return InboxClaim(
            fact=CognitionFact(
                id=row["id"],
                fluctlight_id=row["fluctlight_id"],
                event_type=row["event_type"],
                payload=dict(row["payload"]),
                causation_id=row["causation_id"],
                correlation_id=row["correlation_id"],
                idempotency_key=row["idempotency_key"],
                occurred_at=row["occurred_at"],
            ),
            sequence=next_sequence,
            attempt=int(row["attempt_count"]) + 1,
            worker_id=worker_id,
        )

    async def process_next(self, fluctlight_id: str, *, worker_id: str) -> ProcessOutcome | None:
        claim = await self.claim_next(fluctlight_id, worker_id=worker_id)
        if claim is None:
            return None
        if self._diagnostics is not None:
            await self._diagnostics.emit_turn(self._diagnostics_turn(claim, status="claimed"))
        try:
            envelope = await self._assessment_provider.assess(
                claim.fact, correlation_id=claim.fact.correlation_id
            )
            self._validate_envelope(claim, envelope)
        except Exception as exc:
            code = self._error_code(exc, "assessment_failed")
            await self._settle_failure(claim, code)
            return ProcessOutcome(InboxStatus.FAILED, error_code=code)

        try:
            action = await self._freeze(claim, envelope)
        except Exception as exc:
            code = self._error_code(exc, "freeze_failed")
            await self._settle_failure(claim, code)
            return ProcessOutcome(InboxStatus.FAILED, error_code=code)

        if action.action_type is ActionType.NO_OP:
            result = RealizationResult(action.provider_request_id, {}, status="no_op")
        else:
            try:
                result = await self._realization_provider.realize(
                    action, correlation_id=claim.fact.correlation_id
                )
                if result.provider_request_id != action.provider_request_id:
                    raise ProviderExecutionError("realization returned an unexpected request id")
                action = self._action_after_realization(action, result)
                await self._freeze_autonomy(action)
            except Exception as exc:
                code = self._error_code(exc, "realization_failed")
                await self._settle_failure(claim, code, action=action)
                return ProcessOutcome(InboxStatus.FAILED, action=action, error_code=code)
        await self._settle_success(claim, action, result)
        return ProcessOutcome(InboxStatus.COMPLETED, action=action, realization=result)

    async def stream_next(self, fluctlight_id: str, *, worker_id: str) -> AsyncIterator[str]:
        """Process one claim while yielding visible realization chunks as they arrive."""

        claim = await self.claim_next(fluctlight_id, worker_id=worker_id)
        if claim is None:
            return
        if self._diagnostics is not None:
            await self._diagnostics.emit_turn(self._diagnostics_turn(claim, status="claimed"))
        try:
            envelope = await self._assessment_provider.assess(
                claim.fact, correlation_id=claim.fact.correlation_id
            )
            self._validate_envelope(claim, envelope)
            action = await self._freeze(claim, envelope)
        except Exception as exc:
            code = self._error_code(exc, "assessment_failed")
            await self._settle_failure(claim, code)
            raise

        if action.action_type is ActionType.NO_OP:
            await self._settle_success(
                claim,
                action,
                RealizationResult(action.provider_request_id, {}, status="no_op"),
            )
            return

        stream_realize = getattr(self._realization_provider, "stream_realize", None)
        chunks: list[str] = []
        try:
            if action.action_type is ActionType.MEDIA_REQUEST:
                result = await self._realization_provider.realize(
                    action, correlation_id=claim.fact.correlation_id
                )
                if result.provider_request_id != action.provider_request_id:
                    raise ProviderExecutionError("realization returned an unexpected request id")
                text = result.payload.get("text")
                if not isinstance(text, str) or not text.strip():
                    raise ProviderExecutionError("media realization returned no visible text")
                chunks.append(text)
                yield text
                action = self._action_after_realization(action, result)
                await self._freeze_autonomy(action)
                await self._settle_success(claim, action, result)
                return
            elif stream_realize is None:
                result = await self._realization_provider.realize(
                    action, correlation_id=claim.fact.correlation_id
                )
                text = result.payload.get("text")
                if not isinstance(text, str) or not text.strip():
                    raise ProviderExecutionError("realization returned no visible text")
                for start in range(0, len(text), 64):
                    chunk = text[start : start + 64]
                    chunks.append(chunk)
                    yield chunk
            else:
                async for chunk in stream_realize(action, correlation_id=claim.fact.correlation_id):
                    chunks.append(chunk)
                    yield chunk
            text = "".join(chunks)
            if not text.strip():
                raise ProviderExecutionError("realization returned no visible text")
            result = RealizationResult(
                action.provider_request_id,
                {"text": text, "correlation_id": claim.fact.correlation_id},
            )
            action = self._action_after_realization(action, result)
            await self._freeze_autonomy(action)
            await self._settle_success(claim, action, result)
        except asyncio.CancelledError:
            await self._settle_failure(claim, "realization_cancelled", action=action)
            raise
        except Exception as exc:
            code = self._error_code(exc, "realization_failed")
            await self._settle_failure(claim, code, action=action)
            raise

    async def begin_reflection(
        self,
        fluctlight_id: str,
        *,
        to_sequence: int,
        base_state_revision: int,
    ) -> ReflectionWindow:
        if to_sequence < 1 or base_state_revision < 0:
            raise ValueError("reflection bounds are invalid")
        async with self._unit_of_work.begin(command_id=f"reflection-begin:{fluctlight_id}") as tx:
            row = (
                (
                    await tx.session.execute(
                        select(schema.reflection_windows)
                        .where(schema.reflection_windows.c.fluctlight_id == fluctlight_id)
                        .with_for_update()
                    )
                )
                .mappings()
                .one_or_none()
            )
            if row is None:
                await tx.session.execute(
                    insert(schema.reflection_windows).values(
                        fluctlight_id=fluctlight_id,
                        watermark=0,
                        state_revision=base_state_revision,
                        status="running",
                    )
                )
                watermark = 0
            else:
                if row["status"] == "running":
                    raise CognitionConflictError("reflection is already running")
                watermark = int(row["watermark"])
                if base_state_revision < int(row["state_revision"]):
                    raise CognitionConflictError("reflection state revision is stale")
                await tx.session.execute(
                    update(schema.reflection_windows)
                    .where(schema.reflection_windows.c.fluctlight_id == fluctlight_id)
                    .values(
                        status="running",
                        state_revision=base_state_revision,
                        updated_at=self._clock(),
                    )
                )
            if to_sequence <= watermark:
                raise CognitionConflictError("reflection has no new inbox evidence")
            await tx.commit()
        return ReflectionWindow(
            fluctlight_id, watermark + 1, to_sequence, base_state_revision, watermark
        )

    async def run_reflection(
        self,
        fluctlight_id: str,
        *,
        to_sequence: int,
        base_state_revision: int,
        correlation_id: str,
    ) -> ReflectionProposal:
        if self._reflection_provider is None:
            raise ProviderExecutionError("reflection provider is not configured")
        window = await self.begin_reflection(
            fluctlight_id, to_sequence=to_sequence, base_state_revision=base_state_revision
        )
        try:
            proposal = await self._reflection_provider.reflect(
                window, correlation_id=correlation_id
            )
            if (
                proposal.fluctlight_id != fluctlight_id
                or proposal.to_sequence != window.to_sequence
            ):
                raise CognitionConflictError("reflection provider returned an unexpected window")
            await self.commit_reflection(proposal, expected_watermark=window.watermark)
            if self._reflection_applier is not None:
                await self._reflection_applier.apply(proposal)
            return proposal
        except Exception:
            await self._reset_reflection(fluctlight_id)
            raise

    async def run_current_reflection(
        self, fluctlight_id: str, *, correlation_id: str
    ) -> ReflectionProposal | None:
        """Reflect over the latest settled cognition window, or explicitly no-op."""

        async with self._unit_of_work.begin(command_id=f"reflection-bounds:{fluctlight_id}") as tx:
            to_sequence = await tx.session.scalar(
                select(schema.inbox_heads.c.last_processed_sequence).where(
                    schema.inbox_heads.c.fluctlight_id == fluctlight_id
                )
            )
            action = (
                (
                    await tx.session.execute(
                        select(schema.frozen_actions.c.state_revision)
                        .join(
                            schema.inbox,
                            schema.frozen_actions.c.inbox_id == schema.inbox.c.id,
                        )
                        .where(
                            schema.frozen_actions.c.fluctlight_id == fluctlight_id,
                            schema.inbox.c.status == InboxStatus.COMPLETED.value,
                        )
                        .order_by(schema.inbox.c.sequence.desc())
                        .limit(1)
                    )
                )
                .mappings()
                .one_or_none()
            )
        if not to_sequence or action is None:
            return None
        try:
            return await self.run_reflection(
                fluctlight_id,
                to_sequence=int(to_sequence),
                base_state_revision=int(action["state_revision"]),
                correlation_id=correlation_id,
            )
        except CognitionConflictError:
            return None

    async def commit_reflection(
        self, proposal: ReflectionProposal, *, expected_watermark: int
    ) -> None:
        async with self._unit_of_work.begin(
            command_id=f"reflection-commit:{proposal.proposal_id}"
        ) as tx:
            row = (
                (
                    await tx.session.execute(
                        select(schema.reflection_windows)
                        .where(schema.reflection_windows.c.fluctlight_id == proposal.fluctlight_id)
                        .with_for_update()
                    )
                )
                .mappings()
                .one_or_none()
            )
            if row is None or int(row["watermark"]) != expected_watermark:
                raise CognitionConflictError("reflection watermark is stale")
            if int(row["state_revision"]) != proposal.base_state_revision:
                raise CognitionConflictError("reflection state revision is stale")
            await tx.session.execute(
                insert(schema.reflection_proposals).values(
                    id=proposal.proposal_id,
                    fluctlight_id=proposal.fluctlight_id,
                    from_sequence=proposal.from_sequence,
                    to_sequence=proposal.to_sequence,
                    base_state_revision=proposal.base_state_revision,
                    payload=dict(proposal.payload),
                    evidence_refs=list(proposal.evidence_refs),
                    correlation_id=proposal.provenance.correlation_id,
                )
            )
            await tx.session.execute(
                update(schema.reflection_windows)
                .where(schema.reflection_windows.c.fluctlight_id == proposal.fluctlight_id)
                .values(watermark=proposal.to_sequence, status="idle", updated_at=self._clock())
            )
            await tx.commit()

    async def _freeze(self, claim: InboxClaim, envelope: AssessmentEnvelope) -> FrozenAction:
        assessment = envelope.assessment
        decision = envelope.decision
        action_payload = dict(decision.payload)
        if claim.fact.event_type == "conversation.message":
            source_text = claim.fact.payload.get("text")
            if not isinstance(source_text, str) or not source_text:
                raise ProviderExecutionError("conversation action has no source message")
            action_payload["source_text"] = source_text
            persona_profile = claim.fact.payload.get("persona_profile")
            if isinstance(persona_profile, dict):
                action_payload["persona_profile"] = dict(persona_profile)
        else:
            background_context = claim.fact.payload.get("background_context")
            if isinstance(background_context, dict):
                action_payload["background_context"] = dict(background_context)
                conversation_id = background_context.get("conversation_id")
                if isinstance(conversation_id, str) and conversation_id:
                    action_payload["conversation_id"] = conversation_id
            persona_profile = claim.fact.payload.get("persona_profile")
            if isinstance(persona_profile, dict):
                action_payload["persona_profile"] = dict(persona_profile)
        if decision.action_type is ActionType.PROACTIVE_MESSAGE and not isinstance(
            action_payload.get("conversation_id"), str
        ):
            raise ProviderExecutionError("proactive message has no direct conversation target")
        if decision.action_type is ActionType.MEDIA_REQUEST:
            conversation_id = claim.fact.payload.get("conversation_id")
            if not isinstance(conversation_id, str) or not conversation_id:
                raise ProviderExecutionError("media request has no conversation target")
            action_payload["conversation_id"] = conversation_id
        # Provider decision IDs are opaque metadata, not globally unique database keys.
        # Scope them to the immutable inbox fact so retries remain idempotent while
        # repeated provider IDs cannot poison later turns.
        decision_key = f"{claim.fact.id}:{decision.decision_id}"
        decision_id = f"decision_{sha256(decision_key.encode()).hexdigest()}"
        action_id = stable_action_id(claim.fact.id, decision_id)
        provider_request_id = stable_provider_request_id(action_id)
        assessment_id = f"assessment_{assessment.idempotency_key}"
        async with self._unit_of_work.begin(command_id=f"cognition-freeze:{action_id}") as tx:
            existing = (
                (
                    await tx.session.execute(
                        select(schema.frozen_actions).where(schema.frozen_actions.c.id == action_id)
                    )
                )
                .mappings()
                .one_or_none()
            )
            if existing is not None:
                return self._action_from_row(existing)
            state_revision = 0
            if self._state_applier is not None:
                state_revision = await self._state_applier.apply_assessment(
                    claim.fact.fluctlight_id, assessment, tx=tx
                )
            state_revision = max(state_revision, claim.sequence, 1)
            await tx.session.execute(
                insert(schema.assessments).values(
                    id=assessment_id,
                    inbox_id=claim.fact.id,
                    fluctlight_id=claim.fact.fluctlight_id,
                    payload=assessment.as_payload(),
                    schema_version=assessment.schema_version,
                    model=assessment.model,
                    model_version=assessment.model_version,
                    prompt_version=assessment.prompt_version,
                    correlation_id=claim.fact.correlation_id,
                )
            )
            await tx.session.execute(
                insert(schema.decision_proposals).values(
                    id=decision_id,
                    assessment_id=assessment_id,
                    fluctlight_id=claim.fact.fluctlight_id,
                    action_type=decision.action_type.value,
                    payload=action_payload,
                    confidence=str(decision.confidence),
                    evidence_refs=list(decision.evidence_refs),
                    expires_at=decision.expires_at,
                )
            )
            await tx.session.execute(
                insert(schema.frozen_actions).values(
                    id=action_id,
                    decision_id=decision_id,
                    inbox_id=claim.fact.id,
                    fluctlight_id=claim.fact.fluctlight_id,
                    action_type=decision.action_type.value,
                    payload=action_payload,
                    state_revision=state_revision,
                    provider_request_id=provider_request_id,
                    status=ActionStatus.FROZEN.value,
                )
            )
            await tx.session.execute(
                update(schema.inbox)
                .where(schema.inbox.c.id == claim.fact.id)
                .values(status=InboxStatus.FROZEN.value)
            )
            await add_outbox_event(
                tx.session,
                OutboxEvent(
                    id=f"cognition_action_frozen_{action_id}",
                    kind="cognition.action.frozen",
                    aggregate_type="cognition_action",
                    aggregate_id=action_id,
                    fluctlight_id=claim.fact.fluctlight_id,
                    causation_id=claim.fact.id,
                    correlation_id=claim.fact.correlation_id,
                    idempotency_key=f"cognition-action:{action_id}",
                    payload={
                        "action_id": action_id,
                        "action_type": decision.action_type.value,
                        "aggregate_sequence": 1,
                    },
                    attempt_policy={"max_attempts": 8},
                ),
            )
            await tx.commit()
        return FrozenAction(
            action_id,
            decision_id,
            claim.fact.id,
            claim.fact.fluctlight_id,
            decision.action_type,
            action_payload,
            state_revision,
            provider_request_id,
        )

    async def _freeze_autonomy(self, action: FrozenAction) -> None:
        freezer = getattr(self, "_autonomy_freezer", None)
        if freezer is not None:
            await freezer(action)

    @staticmethod
    def _action_after_realization(action: FrozenAction, result: RealizationResult) -> FrozenAction:
        payload = dict(action.payload)
        if action.action_type is ActionType.MEDIA_REQUEST:
            media_request = result.payload.get("media_request")
            if not isinstance(media_request, dict) or not media_request:
                raise ProviderExecutionError("media realization returned no media request")
            payload["media_request"] = media_request
        elif action.action_type in {ActionType.PROACTIVE_MESSAGE, ActionType.MOMENT}:
            text = result.payload.get("text")
            if not isinstance(text, str) or not text.strip():
                raise ProviderExecutionError("background realization returned no visible text")
            payload["text"] = text
            if action.action_type is ActionType.MOMENT:
                media_request = payload.get("moment_media_request")
                if media_request is not None:
                    if not isinstance(media_request, dict) or not media_request:
                        raise ProviderExecutionError("moment has an invalid frozen media request")
        else:
            return action
        return replace(action, payload=payload)

    async def _settle_success(
        self, claim: InboxClaim, action: FrozenAction, result: RealizationResult
    ) -> None:
        now = self._clock()
        async with self._unit_of_work.begin(
            command_id=f"cognition-complete:{action.action_id}"
        ) as tx:
            await tx.session.execute(
                update(schema.frozen_actions)
                .where(schema.frozen_actions.c.id == action.action_id)
                .values(
                    status=ActionStatus.COMPLETED.value,
                    realization_payload=dict(result.payload),
                    completed_at=now,
                )
            )
            await self._mark_processed(tx, claim, InboxStatus.COMPLETED, None, now)
            await add_outbox_event(
                tx.session,
                OutboxEvent(
                    id=f"cognition_action_completed_{action.action_id}",
                    kind="cognition.action.completed",
                    aggregate_type="cognition_action",
                    aggregate_id=action.action_id,
                    fluctlight_id=claim.fact.fluctlight_id,
                    causation_id=claim.fact.id,
                    correlation_id=claim.fact.correlation_id,
                    idempotency_key=f"cognition-complete:{action.action_id}",
                    payload={
                        "action_id": action.action_id,
                        "status": result.status,
                        "aggregate_sequence": 2,
                    },
                    attempt_policy={"max_attempts": 8},
                ),
            )
            await commit_workflow_intent(
                tx.session,
                CommittedWorkflowIntent(
                    intent_id=f"reflection_intent:{action.action_id}",
                    workflow_id=f"reflection:{claim.fact.fluctlight_id}:{action.action_id}",
                    task_queue="lifecycle",
                    intent_type="reflection.run",
                    payload={
                        "fluctlight_id": claim.fact.fluctlight_id,
                        "correlation_id": claim.fact.correlation_id,
                    },
                ),
            )
            await tx.commit()
        if self._diagnostics is not None:
            from fluctlight_core.diagnostics.contracts import (
                DiagnosticEvent,
                DiagnosticSeverity,
            )

            await self._diagnostics.emit_turn(self._diagnostics_turn(claim, status="completed"))
            await self._diagnostics.emit_event(
                DiagnosticEvent(
                    event_type="cognition.turn.completed",
                    severity=DiagnosticSeverity.INFO,
                    fluctlight_id=claim.fact.fluctlight_id,
                    causation_id=claim.fact.id,
                    correlation_id=claim.fact.correlation_id,
                    payload={"status": "completed", "action_type": action.action_type.value},
                )
            )

    async def _settle_failure(
        self, claim: InboxClaim, error_code: str, *, action: FrozenAction | None = None
    ) -> None:
        now = self._clock()
        async with self._unit_of_work.begin(command_id=f"cognition-failed:{claim.fact.id}") as tx:
            if action is not None:
                await tx.session.execute(
                    update(schema.frozen_actions)
                    .where(schema.frozen_actions.c.id == action.action_id)
                    .values(
                        status=ActionStatus.FAILED.value, error_code=error_code, completed_at=now
                    )
                )
            await self._mark_processed(tx, claim, InboxStatus.FAILED, error_code, now)
            await tx.commit()
        if self._diagnostics is not None:
            from fluctlight_core.diagnostics.contracts import (
                DiagnosticEvent,
                DiagnosticSeverity,
            )

            await self._diagnostics.emit_turn(self._diagnostics_turn(claim, status="failed"))
            await self._diagnostics.emit_event(
                DiagnosticEvent(
                    event_type="cognition.turn.failed",
                    severity=DiagnosticSeverity.ERROR,
                    fluctlight_id=claim.fact.fluctlight_id,
                    causation_id=claim.fact.id,
                    correlation_id=claim.fact.correlation_id,
                    payload={"status": "failed", "error_code": error_code},
                )
            )

    async def _mark_processed(
        self,
        tx: UnitOfWork,
        claim: InboxClaim,
        status: InboxStatus,
        error_code: str | None,
        now: datetime,
    ) -> None:
        await tx.session.execute(
            update(schema.inbox)
            .where(schema.inbox.c.id == claim.fact.id)
            .values(status=status.value, error_code=error_code, processed_at=now)
        )
        await tx.session.execute(
            update(schema.inbox_heads)
            .where(schema.inbox_heads.c.fluctlight_id == claim.fact.fluctlight_id)
            .values(
                last_processed_sequence=claim.sequence,
                writer_owner=None,
                writer_lease_until=None,
            )
        )

    async def _reset_reflection(self, fluctlight_id: str) -> None:
        async with self._unit_of_work.begin(command_id=f"reflection-reset:{fluctlight_id}") as tx:
            await tx.session.execute(
                update(schema.reflection_windows)
                .where(schema.reflection_windows.c.fluctlight_id == fluctlight_id)
                .values(status="idle", updated_at=self._clock())
            )
            await tx.commit()

    @staticmethod
    def _validate_envelope(claim: InboxClaim, envelope: AssessmentEnvelope) -> None:
        if envelope.assessment.source_event_id != claim.fact.id:
            raise CognitionConflictError("assessment source event does not match inbox claim")
        if envelope.assessment.idempotency_key != claim.fact.idempotency_key:
            raise CognitionConflictError("assessment idempotency key does not match inbox claim")
        if envelope.provenance.correlation_id != claim.fact.correlation_id:
            raise CognitionConflictError("Provider provenance correlation does not match claim")

    @staticmethod
    def _action_from_row(row: Any) -> FrozenAction:
        from .contracts import ActionType

        return FrozenAction(
            action_id=row["id"],
            decision_id=row["decision_id"],
            inbox_id=row["inbox_id"],
            fluctlight_id=row["fluctlight_id"],
            action_type=ActionType(row["action_type"]),
            payload=dict(row["payload"]),
            state_revision=int(row["state_revision"]),
            provider_request_id=row["provider_request_id"],
            status=ActionStatus(row["status"]),
        )

    @staticmethod
    def _error_code(exc: Exception, default: str) -> str:
        message = str(exc).strip().lower().replace(" ", "_")
        return message[:120] or default

    @staticmethod
    def _diagnostics_turn(claim: InboxClaim, *, status: str) -> Any:
        from fluctlight_core.diagnostics.contracts import DiagnosticTurn

        return DiagnosticTurn(
            fluctlight_id=claim.fact.fluctlight_id,
            correlation_id=claim.fact.correlation_id,
            status=status,
            source_event_id=claim.fact.id,
        )
