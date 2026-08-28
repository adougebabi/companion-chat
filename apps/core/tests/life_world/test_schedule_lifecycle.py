import asyncio
from datetime import UTC, datetime, timedelta

import pytest
from fluctlight_core.fluctlights.contracts import (
    BehavioralPolicy,
    FluctlightSnapshot,
    FluctlightStatus,
    Identity,
    InitializationMode,
    Personality,
)
from fluctlight_core.life_world.lifecycle import schedule_lifecycle_intent
from fluctlight_core.life_world.workflows import (
    CurrentDayScheduleWorkflow,
    DailyLifeReviewWorkflow,
    _next_local_midnight_delay,
    configure_current_day_schedule_service,
    ensure_current_day_schedule,
    process_daily_life_review,
)
from temporalio.worker.workflow_sandbox import SandboxedWorkflowRunner
from temporalio.workflow import _Definition


def _fluctlight(*, status: FluctlightStatus = FluctlightStatus.ACTIVE) -> FluctlightSnapshot:
    return FluctlightSnapshot(
        id="fluctlight-1",
        initialization_mode=InitializationMode.BLANK_SLATE,
        status=status,
        identity=Identity(id="fluctlight-1", timezone="UTC+8"),
        personality=Personality.neutral(),
        behavioral_policy=BehavioralPolicy(),
    )


class _Fluctlights:
    def __init__(self, snapshot: FluctlightSnapshot) -> None:
        self.snapshot = snapshot
        self.requested: list[str] = []

    async def get(self, fluctlight_id: str) -> FluctlightSnapshot:
        self.requested.append(fluctlight_id)
        return self.snapshot


class _Schedules:
    def __init__(self, result: object | None = object()) -> None:
        self.result = result
        self.ensured: list[FluctlightSnapshot] = []

    async def ensure_for(self, fluctlight: FluctlightSnapshot) -> object | None:
        self.ensured.append(fluctlight)
        return self.result


class _DailyReview:
    def __init__(self) -> None:
        self.calls: list[tuple[str, object]] = []

    async def review_current_day(self, fluctlight_id: str, schedule: object) -> dict[str, str]:
        self.calls.append((fluctlight_id, schedule))
        return {"status": "completed", "action_type": "moment"}


class _LifeWorld:
    def __init__(self, schedule: object | None) -> None:
        self.schedule = schedule

    async def accepted_schedule(self, _fluctlight_id: str, _instant: datetime) -> object | None:
        return self.schedule


def test_schedule_lifecycle_intent_is_stable_per_fluctlight() -> None:
    intent = schedule_lifecycle_intent("fluctlight-1")

    assert intent.intent_id == "schedule_intent:fluctlight-1"
    assert intent.workflow_id == "schedule:fluctlight-1"
    assert intent.task_queue == "lifecycle"
    assert intent.intent_type == "schedule.current_day"
    assert intent.payload == {"fluctlight_id": "fluctlight-1"}


def test_current_day_schedule_activity_uses_latest_active_fluctlight() -> None:
    async def verify() -> None:
        fluctlights = _Fluctlights(_fluctlight())
        schedules = _Schedules()
        configure_current_day_schedule_service(fluctlights, schedules)

        result = await ensure_current_day_schedule({"fluctlight_id": "fluctlight-1"})

        assert result["fluctlight_id"] == "fluctlight-1"
        assert result["timezone"] == "Asia/Shanghai"
        assert result["status"] == "ready"
        assert int(result["next_local_midnight_delay_seconds"]) > 0
        assert fluctlights.requested == ["fluctlight-1"]
        assert schedules.ensured == [fluctlights.snapshot]

    asyncio.run(verify())


def test_current_day_schedule_activity_reports_pending_when_current_day_is_still_missing() -> None:
    async def verify() -> None:
        fluctlights = _Fluctlights(_fluctlight())
        schedules = _Schedules(result=None)
        configure_current_day_schedule_service(fluctlights, schedules)

        result = await ensure_current_day_schedule({"fluctlight_id": "fluctlight-1"})

        assert result["status"] == "pending"

    asyncio.run(verify())


def test_current_day_schedule_activity_reenters_background_cognition_after_schedule_ready() -> None:
    async def verify() -> None:
        fluctlights = _Fluctlights(_fluctlight())
        schedule = object()
        schedules = _Schedules(result=schedule)
        daily_review = _DailyReview()
        configure_current_day_schedule_service(fluctlights, schedules, daily_review)

        result = await ensure_current_day_schedule({"fluctlight_id": "fluctlight-1"})

        assert result["daily_review_status"] == "completed"
        assert result["daily_review_action_type"] == "moment"
        assert daily_review.calls == [("fluctlight-1", schedule)]

    asyncio.run(verify())


def test_daily_review_activity_waits_for_schedule_then_uses_the_same_review_service() -> None:
    async def verify() -> None:
        daily_review = _DailyReview()
        schedule = object()
        configure_current_day_schedule_service(
            _Fluctlights(_fluctlight()),
            _Schedules(),
            daily_review,
            _LifeWorld(schedule),
        )

        result = await process_daily_life_review({"fluctlight_id": "fluctlight-1"})

        assert result == {"status": "completed", "action_type": "moment"}
        assert daily_review.calls == [("fluctlight-1", schedule)]

    asyncio.run(verify())


def test_current_day_schedule_activity_does_not_plan_for_an_inactive_fluctlight() -> None:
    async def verify() -> None:
        fluctlights = _Fluctlights(_fluctlight(status=FluctlightStatus.PAUSED))
        schedules = _Schedules()
        configure_current_day_schedule_service(fluctlights, schedules)

        result = await ensure_current_day_schedule({"fluctlight_id": "fluctlight-1"})

        assert result["status"] == "inactive"
        assert schedules.ensured == []

    asyncio.run(verify())


def test_current_day_schedule_activity_requires_fluctlight_id() -> None:
    async def verify() -> None:
        configure_current_day_schedule_service(_Fluctlights(_fluctlight()), _Schedules())
        with pytest.raises(ValueError, match="requires fluctlight_id"):
            await ensure_current_day_schedule({})

    asyncio.run(verify())


def test_next_schedule_timer_targets_local_midnight() -> None:
    now = datetime(2026, 8, 26, 15, tzinfo=UTC)

    assert _next_local_midnight_delay(now, "Asia/Shanghai") == timedelta(hours=1)
    assert _next_local_midnight_delay(
        datetime(2026, 3, 8, 5, tzinfo=UTC), "America/New_York"
    ) == timedelta(hours=23)


def test_current_day_schedule_workflow_loads_in_temporal_sandbox() -> None:
    async def prepare() -> None:
        SandboxedWorkflowRunner().prepare_workflow(
            _Definition.from_class(CurrentDayScheduleWorkflow)
        )
        SandboxedWorkflowRunner().prepare_workflow(_Definition.from_class(DailyLifeReviewWorkflow))

    asyncio.run(prepare())
