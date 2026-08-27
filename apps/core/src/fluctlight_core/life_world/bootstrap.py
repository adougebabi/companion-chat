"""LLM-owned initial daily Schedule generation for newly activated Fluctlights."""

from __future__ import annotations

from collections.abc import Callable, Mapping
from dataclasses import dataclass
from datetime import UTC, date, datetime
from typing import Any, Protocol
from uuid import uuid4

from fluctlight_core.diagnostics.contracts import DiagnosticEvent, DiagnosticSeverity
from fluctlight_core.diagnostics.service import DiagnosticsService
from fluctlight_core.fluctlights.contracts import FluctlightSnapshot
from fluctlight_core.platform.timezones import canonical_timezone

from .contracts import ScheduleItem, ScheduleValidationError, ScheduleVersion, timezone_or_error
from .service import LifeWorldService


class InitialScheduleGenerator(Protocol):
    async def generate_initial_schedule(
        self,
        *,
        fluctlight_id: str,
        identity: Mapping[str, Any],
        local_date: date,
        timezone: str,
    ) -> dict[str, Any]: ...


@dataclass(slots=True)
class InitialScheduleService:
    life_world: LifeWorldService
    generator: InitialScheduleGenerator
    diagnostics: DiagnosticsService | None = None
    default_timezone: str = "Asia/Shanghai"
    clock: Callable[[], datetime] = lambda: datetime.now(UTC)

    async def ensure_for(self, fluctlight: FluctlightSnapshot) -> ScheduleVersion | None:
        identity = fluctlight.identity.as_payload()
        raw_timezone = str(identity.get("timezone") or self.default_timezone)
        correlation_id = f"schedule-initialization:{fluctlight.id}"
        try:
            timezone = canonical_timezone(raw_timezone)
            zone = timezone_or_error(timezone)
            now = self.clock()
            if await self.life_world.accepted_schedule(fluctlight.id, now) is not None:
                return None
            local_date = now.astimezone(zone).date()
            correlation_id = f"{correlation_id}:{local_date.isoformat()}"
            payload = await self.generator.generate_initial_schedule(
                fluctlight_id=fluctlight.id,
                identity=identity,
                local_date=local_date,
                timezone=timezone,
            )
            proposal = self._proposal(
                fluctlight_id=fluctlight.id,
                local_date=local_date,
                timezone=timezone,
                payload=payload,
            )
            schedule = await self.life_world.accept_schedule(proposal)
        except Exception as exc:
            await self._record_failure(fluctlight.id, correlation_id, raw_timezone, exc)
            return None
        if self.diagnostics is not None:
            await self.diagnostics.emit_event(
                DiagnosticEvent(
                    event_type="schedule.initialization.completed",
                    severity=DiagnosticSeverity.INFO,
                    fluctlight_id=fluctlight.id,
                    correlation_id=correlation_id,
                    payload={"schedule_id": schedule.id, "local_date": local_date.isoformat()},
                )
            )
        return schedule

    @staticmethod
    def _proposal(
        *,
        fluctlight_id: str,
        local_date: date,
        timezone: str,
        payload: Mapping[str, Any],
    ) -> ScheduleVersion:
        raw_items = payload.get("items")
        if not isinstance(raw_items, list) or not raw_items:
            raise ScheduleValidationError("initial schedule response has no items")
        items = tuple(
            ScheduleItem(
                id=f"schedule_item_{uuid4().hex}",
                start_at=datetime.fromisoformat(str(item["start_at"])),
                end_at=datetime.fromisoformat(str(item["end_at"])),
                activity=str(item["activity"]),
                scene=str(item["scene"]),
                item_type=str(item.get("item_type", "planned")),
                status=str(item.get("status", "planned")),
                priority=float(item.get("priority", 0.5)),
                flexibility=float(item.get("flexibility", 0.5)),
                interruption_cost=float(item.get("interruption_cost", 0.5)),
            )
            for item in raw_items
            if isinstance(item, Mapping)
        )
        if len(items) != len(raw_items):
            raise ScheduleValidationError("initial schedule items must be objects")
        return ScheduleVersion(
            id=f"schedule_initial_{uuid4().hex}",
            fluctlight_id=fluctlight_id,
            local_date=local_date,
            timezone=timezone,
            items=items,
            generated_from="initialization",
            evidence_refs=(f"foundation:{fluctlight_id}",),
            reschedule_policy=dict(payload.get("reschedule_policy", {})),
        )

    async def _record_failure(
        self,
        fluctlight_id: str,
        correlation_id: str,
        timezone: str,
        exc: Exception,
    ) -> None:
        if self.diagnostics is None:
            return
        code = str(exc).strip().lower().replace(" ", "_")[:120]
        await self.diagnostics.emit_event(
            DiagnosticEvent(
                event_type="schedule.initialization.failed",
                severity=DiagnosticSeverity.ERROR,
                fluctlight_id=fluctlight_id,
                correlation_id=correlation_id,
                payload={
                    "error_code": code or "schedule_initialization_failed",
                    "timezone": timezone,
                },
            )
        )
