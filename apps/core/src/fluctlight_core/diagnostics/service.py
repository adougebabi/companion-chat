"""Diagnostics persistence isolated from business transactions."""

from __future__ import annotations

import json
from collections.abc import Callable, Mapping
from datetime import UTC, datetime, timedelta
from typing import Any
from uuid import uuid4

from sqlalchemy import delete, func, insert, select

from fluctlight_core.platform.persistence import UnitOfWorkFactory

from . import schema
from .contracts import (
    DiagnosticEvent,
    DiagnosticModelRun,
    DiagnosticTurn,
    DiagnosticWorkflowLink,
    redact,
)


class DiagnosticsAuthorizationError(PermissionError):
    """Diagnostics are only available to the Owner actor."""


FallbackWriter = Callable[[str], None]


class DiagnosticsService:
    """Store bounded, redacted diagnostics through independent short UoWs."""

    def __init__(
        self,
        unit_of_work: UnitOfWorkFactory,
        *,
        fallback_writer: FallbackWriter | None = None,
        clock: Callable[[], datetime] | None = None,
    ) -> None:
        self._unit_of_work = unit_of_work
        self._fallback_writer = fallback_writer or print
        self._clock = clock or (lambda: datetime.now(UTC))

    async def emit_event(self, event: DiagnosticEvent) -> str:
        event_id = event.event_id or f"diag_{uuid4().hex}"
        try:
            async with self._unit_of_work.begin(command_id=f"diagnostic-event:{event_id}") as tx:
                await tx.session.execute(
                    insert(schema.diagnostic_events).values(
                        id=event_id,
                        event_type=event.event_type,
                        severity=event.severity.value,
                        fluctlight_id=event.fluctlight_id,
                        causation_id=event.causation_id,
                        correlation_id=event.correlation_id,
                        payload=redact(event.payload),
                        created_at=event.created_at,
                    )
                )
                await tx.commit()
        except Exception as exc:  # diagnostics must never break a business command
            self._fallback(
                {
                    "kind": "diagnostic_sink_failure",
                    "event_id": event_id,
                    "correlation_id": event.correlation_id,
                    "error": type(exc).__name__,
                }
            )
        return event_id

    async def emit_model_run(self, run: DiagnosticModelRun) -> str:
        run_id = run.run_id or f"model_run_{uuid4().hex}"
        try:
            async with self._unit_of_work.begin(command_id=f"diagnostic-model:{run_id}") as tx:
                await tx.session.execute(
                    insert(schema.diagnostic_model_runs).values(
                        id=run_id,
                        role=run.role,
                        endpoint_id=run.endpoint_id,
                        model_id=run.model_id,
                        prompt=redact(run.prompt),
                        response=redact(run.response) if run.response is not None else None,
                        status=run.status,
                        error_code=run.error_code,
                        correlation_id=run.correlation_id,
                        created_at=run.created_at,
                    )
                )
                await tx.commit()
        except Exception as exc:
            self._fallback(
                {
                    "kind": "diagnostic_sink_failure",
                    "resource": "model_run",
                    "run_id": run_id,
                    "correlation_id": run.correlation_id,
                    "error": type(exc).__name__,
                }
            )
        return run_id

    async def emit_turn(self, turn: DiagnosticTurn) -> str:
        turn_id = turn.turn_id or f"turn_{uuid4().hex}"
        try:
            async with self._unit_of_work.begin(command_id=f"diagnostic-turn:{turn_id}") as tx:
                await tx.session.execute(
                    insert(schema.diagnostic_turns).values(
                        id=turn_id,
                        fluctlight_id=turn.fluctlight_id,
                        conversation_id=turn.conversation_id,
                        source_event_id=turn.source_event_id,
                        correlation_id=turn.correlation_id,
                        status=turn.status,
                        created_at=turn.created_at,
                    )
                )
                await tx.commit()
        except Exception as exc:
            self._fallback(
                {
                    "kind": "diagnostic_sink_failure",
                    "resource": "turn",
                    "turn_id": turn_id,
                    "correlation_id": turn.correlation_id,
                    "error": type(exc).__name__,
                }
            )
        return turn_id

    async def link_workflow(self, link: DiagnosticWorkflowLink) -> str:
        link_id = link.link_id or f"workflow_link_{uuid4().hex}"
        try:
            async with self._unit_of_work.begin(command_id=f"diagnostic-workflow:{link_id}") as tx:
                await tx.session.execute(
                    insert(schema.diagnostic_workflow_links).values(
                        id=link_id,
                        correlation_id=link.correlation_id,
                        workflow_id=link.workflow_id,
                        intent_id=link.intent_id,
                        event_id=link.event_id,
                        created_at=link.created_at,
                    )
                )
                await tx.commit()
        except Exception as exc:
            self._fallback(
                {
                    "kind": "diagnostic_sink_failure",
                    "resource": "workflow_link",
                    "link_id": link_id,
                    "correlation_id": link.correlation_id,
                    "error": type(exc).__name__,
                }
            )
        return link_id

    async def query_events(
        self,
        *,
        actor_id: str,
        owner_actor_id: str,
        correlation_id: str | None = None,
        fluctlight_id: str | None = None,
        limit: int = 100,
    ) -> list[dict[str, Any]]:
        self._require_owner(actor_id, owner_actor_id)
        bounded_limit = min(max(limit, 1), 500)
        async with self._unit_of_work.begin(command_id=f"diagnostic-query:{uuid4().hex}") as tx:
            statement = (
                select(schema.diagnostic_events)
                .order_by(schema.diagnostic_events.c.created_at.desc())
                .limit(bounded_limit)
            )
            if correlation_id:
                statement = statement.where(
                    schema.diagnostic_events.c.correlation_id == correlation_id
                )
            if fluctlight_id:
                statement = statement.where(
                    schema.diagnostic_events.c.fluctlight_id == fluctlight_id
                )
            rows = (await tx.session.execute(statement)).mappings().all()
        return [self._event_view(row) for row in rows]

    async def query_model_runs(
        self,
        *,
        actor_id: str,
        owner_actor_id: str,
        correlation_id: str | None = None,
        limit: int = 100,
    ) -> list[dict[str, Any]]:
        self._require_owner(actor_id, owner_actor_id)
        bounded_limit = min(max(limit, 1), 500)
        async with self._unit_of_work.begin(
            command_id=f"diagnostic-model-query:{uuid4().hex}"
        ) as tx:
            statement = (
                select(schema.diagnostic_model_runs)
                .order_by(schema.diagnostic_model_runs.c.created_at.desc())
                .limit(bounded_limit)
            )
            if correlation_id:
                statement = statement.where(
                    schema.diagnostic_model_runs.c.correlation_id == correlation_id
                )
            rows = (await tx.session.execute(statement)).mappings().all()
        return [
            {
                "id": row["id"],
                "role": row["role"],
                "endpoint_id": row["endpoint_id"],
                "model_id": row["model_id"],
                "prompt": redact(row["prompt"]),
                "response": redact(row["response"]) if row["response"] is not None else None,
                "status": row["status"],
                "error_code": row["error_code"],
                "correlation_id": row["correlation_id"],
                "created_at": row["created_at"].isoformat(),
            }
            for row in rows
        ]

    async def export_events(
        self, *, actor_id: str, owner_actor_id: str, limit: int = 500
    ) -> list[dict[str, Any]]:
        return await self.query_events(
            actor_id=actor_id, owner_actor_id=owner_actor_id, limit=limit
        )

    async def clear_events(self, *, actor_id: str, owner_actor_id: str) -> int:
        self._require_owner(actor_id, owner_actor_id)
        async with self._unit_of_work.begin(command_id=f"diagnostic-clear:{uuid4().hex}") as tx:
            event_count = await tx.session.scalar(
                select(func.count()).select_from(schema.diagnostic_events)
            )
            model_count = await tx.session.scalar(
                select(func.count()).select_from(schema.diagnostic_model_runs)
            )
            await tx.session.execute(delete(schema.diagnostic_events))
            await tx.session.execute(delete(schema.diagnostic_model_runs))
            await tx.session.execute(delete(schema.diagnostic_turns))
            await tx.session.execute(delete(schema.diagnostic_workflow_links))
            await tx.commit()
        return int(event_count or 0) + int(model_count or 0)

    async def enforce_retention(
        self,
        *,
        actor_id: str,
        owner_actor_id: str,
        retention_days: int = 30,
        max_rows: int = 10_000,
    ) -> int:
        self._require_owner(actor_id, owner_actor_id)
        if retention_days < 1 or max_rows < 1:
            raise ValueError("diagnostic retention bounds must be positive")
        cutoff = self._clock() - timedelta(days=retention_days)
        resources = (
            schema.diagnostic_events,
            schema.diagnostic_model_runs,
            schema.diagnostic_turns,
            schema.diagnostic_workflow_links,
        )
        removed = 0
        async with self._unit_of_work.begin(command_id=f"diagnostic-retention:{uuid4().hex}") as tx:
            for resource in resources:
                expired = await tx.session.execute(
                    delete(resource).where(resource.c.created_at < cutoff)
                )
                removed += int(expired.rowcount or 0)
                ids = (
                    (
                        await tx.session.execute(
                            select(resource.c.id)
                            .order_by(resource.c.created_at.desc())
                            .offset(max_rows)
                        )
                    )
                    .scalars()
                    .all()
                )
                if ids:
                    surplus = await tx.session.execute(
                        delete(resource).where(resource.c.id.in_(ids))
                    )
                    removed += int(surplus.rowcount or 0)
            await tx.commit()
        return removed

    def _require_owner(self, actor_id: str, owner_actor_id: str) -> None:
        if not actor_id or actor_id != owner_actor_id:
            raise DiagnosticsAuthorizationError("diagnostics require Owner authorization")

    def _fallback(self, payload: Mapping[str, Any]) -> None:
        try:
            self._fallback_writer(json.dumps(redact(payload), sort_keys=True, default=str))
        except Exception:
            return

    @staticmethod
    def _event_view(row: Any) -> dict[str, Any]:
        return {
            "id": row["id"],
            "event_type": row["event_type"],
            "severity": row["severity"],
            "fluctlight_id": row["fluctlight_id"],
            "causation_id": row["causation_id"],
            "correlation_id": row["correlation_id"],
            "payload": redact(row["payload"]),
            "created_at": row["created_at"].isoformat() if row["created_at"] else None,
        }
