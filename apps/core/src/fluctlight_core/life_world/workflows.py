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
_daily_review: Any | None = None
_life_world: Any | None = None


def configure_current_day_schedule_service(
    fluctlights: Any,
    schedules: Any,
    daily_review: Any | None = None,
    life_world: Any | None = None,
) -> None:
    global _daily_review, _fluctlights, _life_world, _schedules
    _fluctlights = fluctlights
    _schedules = schedules
    _daily_review = daily_review
    _life_world = life_world or getattr(schedules, "life_world", None)


def _next_local_midnight_delay(now: datetime, timezone: str) -> timedelta:
    # Temporal's workflow clock carries a sandbox `_RestrictedProxy` tzinfo.
    # Rebuild the instant with the concrete UTC tzinfo before calling zoneinfo
    # conversion; otherwise datetime.astimezone rejects the proxy object.
    utc_now = datetime(
        now.year,
        now.month,
        now.day,
        now.hour,
        now.minute,
        now.second,
        now.microsecond,
        tzinfo=UTC,
    )
    zone = ZoneInfo(timezone)
    local_now = utc_now.astimezone(zone)
    next_midnight = datetime.combine(local_now.date() + timedelta(days=1), time.min, tzinfo=zone)
    return max(next_midnight.astimezone(UTC) - utc_now, timedelta(seconds=1))


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
    result = {
        "fluctlight_id": fluctlight_id,
        "timezone": timezone,
        "status": "ready" if schedule is not None else "pending",
    }
    if schedule is not None and _daily_review is not None:
        review = await _daily_review.review_current_day(fluctlight_id, schedule)
        result["daily_review_status"] = str(review.get("status", "unknown"))
        result["daily_review_action_type"] = str(review.get("action_type", "no_op"))
    return result


@activity.defn(name="process_daily_life_review")
async def process_daily_life_review(payload: dict[str, Any]) -> dict[str, str]:
    if _daily_review is None or _life_world is None:
        raise RuntimeError("daily life review activity is not configured")
    fluctlight_id = str(payload.get("fluctlight_id", "")).strip()
    if not fluctlight_id:
        raise ValueError("daily life review requires fluctlight_id")
    schedule = await _life_world.accepted_schedule(fluctlight_id, datetime.now(UTC))
    if schedule is None:
        return {"status": "pending"}
    return await _daily_review.review_current_day(fluctlight_id, schedule)


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


@workflow.defn(name="DailyLifeReviewWorkflow")
class DailyLifeReviewWorkflow:
    """Run one current-day review or retry only until its Schedule becomes ready."""

    @workflow.run
    async def run(self, payload: dict[str, Any]) -> dict[str, str]:
        if not str(payload.get("fluctlight_id", "")).strip():
            raise ValueError("daily life review workflow requires fluctlight_id")
        if not str(payload.get("intent_id", "")).strip():
            raise ValueError("daily life review workflow requires intent_id")
        result = await workflow.execute_activity(
            process_daily_life_review,
            payload,
            start_to_close_timeout=timedelta(minutes=10),
        )
        if result.get("status") == "pending":
            await workflow.sleep(timedelta(minutes=5))
            workflow.continue_as_new(payload)
            raise AssertionError("workflow.continue_as_new must not return")
        return result
