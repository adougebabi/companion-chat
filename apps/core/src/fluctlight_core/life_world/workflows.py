"""Temporal adapter for current-local-day Schedule generation."""

from __future__ import annotations

from datetime import UTC, datetime, time, timedelta
from typing import Any
from zoneinfo import ZoneInfo

from temporalio import activity, workflow

with workflow.unsafe.imports_passed_through():
    from fluctlight_core.fluctlights.contracts import FluctlightStatus
    from fluctlight_core.platform.timezones import canonical_timezone

_fluctlights: Any | None = None
_schedules: Any | None = None


def configure_current_day_schedule_service(fluctlights: Any, schedules: Any) -> None:
    global _fluctlights, _schedules
    _fluctlights = fluctlights
    _schedules = schedules


def _next_local_midnight_delay(now: datetime, timezone: str) -> timedelta:
    zone = ZoneInfo(timezone)
    local_now = now.astimezone(zone)
    next_midnight = datetime.combine(local_now.date() + timedelta(days=1), time.min, tzinfo=zone)
    return max(next_midnight.astimezone(UTC) - now.astimezone(UTC), timedelta(seconds=1))


@activity.defn(name="ensure_current_day_schedule")
async def ensure_current_day_schedule(payload: dict[str, Any]) -> dict[str, str]:
    if _fluctlights is None or _schedules is None:
        raise RuntimeError("current-day schedule activity is not configured")
    fluctlight_id = str(payload.get("fluctlight_id", "")).strip()
    if not fluctlight_id:
        raise ValueError("current-day schedule activity requires fluctlight_id")
    fluctlight = await _fluctlights.get(fluctlight_id)
    timezone = canonical_timezone(
        str(fluctlight.identity.as_payload().get("timezone") or "Asia/Shanghai")
    )
    if fluctlight.status is not FluctlightStatus.ACTIVE:
        return {"fluctlight_id": fluctlight_id, "timezone": timezone, "status": "inactive"}
    schedule = await _schedules.ensure_for(fluctlight)
    if schedule is None:
        life_world = getattr(_schedules, "life_world", None)
        if life_world is not None:
            schedule = await life_world.accepted_schedule(fluctlight_id, datetime.now(UTC))
    return {
        "fluctlight_id": fluctlight_id,
        "timezone": timezone,
        "status": "ready" if schedule is not None else "pending",
    }


@workflow.defn(name="CurrentDayScheduleWorkflow")
class CurrentDayScheduleWorkflow:
    """Ensure one local day, then roll the durable timer into a fresh history."""

    @workflow.run
    async def run(self, payload: dict[str, Any]) -> dict[str, str]:
        fluctlight_id = str(payload.get("fluctlight_id", "")).strip()
        if not fluctlight_id:
            raise ValueError("current-day schedule workflow requires fluctlight_id")
        if not str(payload.get("intent_id", "")).strip():
            raise ValueError("current-day schedule workflow requires intent_id")
        result = await workflow.execute_activity(
            ensure_current_day_schedule,
            payload,
            start_to_close_timeout=timedelta(minutes=10),
        )
        if result["status"] == "inactive":
            return result
        if result["status"] == "pending":
            await workflow.sleep(timedelta(minutes=5))
            workflow.continue_as_new(payload)
            raise AssertionError("workflow.continue_as_new must not return")
        await workflow.sleep(_next_local_midnight_delay(workflow.now(), result["timezone"]))
        workflow.continue_as_new(payload)
        raise AssertionError("workflow.continue_as_new must not return")
