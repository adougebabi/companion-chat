import asyncio
from datetime import UTC, date, datetime
from typing import Any
from zoneinfo import ZoneInfo

from fluctlight_core.fluctlights.contracts import (
    BehavioralPolicy,
    FluctlightSnapshot,
    FluctlightStatus,
    Identity,
    InitializationMode,
    Personality,
)
from fluctlight_core.life_world.bootstrap import InitialScheduleService


class LifeWorldRecorder:
    def __init__(self) -> None:
        self.accepted: list[Any] = []

    async def accepted_schedule(self, *_args):
        return None

    async def accept_schedule(self, schedule):
        self.accepted.append(schedule)
        return schedule


class ScheduleGenerator:
    def __init__(self) -> None:
        self.calls: list[Any] = []

    async def generate_initial_schedule(self, **kwargs):
        self.calls.append(kwargs)
        zone = ZoneInfo(kwargs["timezone"])
        day = kwargs["local_date"]
        return {
            "items": [
                {
                    "start_at": datetime(day.year, day.month, day.day, tzinfo=zone).isoformat(),
                    "end_at": datetime(day.year, day.month, day.day + 1, tzinfo=zone).isoformat(),
                    "activity": "model-defined activity",
                    "scene": "model-defined scene",
                    "item_type": "planned",
                    "status": "planned",
                    "priority": 0.5,
                    "flexibility": 0.5,
                    "interruption_cost": 0.5,
                }
            ]
        }


def test_initial_schedule_is_generated_by_model_and_covers_the_local_day() -> None:
    life_world = LifeWorldRecorder()
    generator = ScheduleGenerator()
    service = InitialScheduleService(
        life_world,  # type: ignore[arg-type]
        generator,  # type: ignore[arg-type]
        clock=lambda: datetime(2026, 8, 26, 4, tzinfo=UTC),
    )
    fluctlight = FluctlightSnapshot(
        id="fluctlight-1",
        initialization_mode=InitializationMode.BLANK_SLATE,
        status=FluctlightStatus.ACTIVE,
        identity=Identity(id="fluctlight-1", timezone="Asia/Shanghai"),
        personality=Personality.neutral(),
        behavioral_policy=BehavioralPolicy(),
    )

    schedule = asyncio.run(service.ensure_for(fluctlight))

    assert schedule is not None
    assert generator.calls[0]["local_date"] == date(2026, 8, 26)
    assert schedule.generated_from == "initialization"
    assert schedule.items[0].activity == "model-defined activity"
    assert life_world.accepted == [schedule]


def test_initial_schedule_normalizes_existing_utc_plus_eight_foundations() -> None:
    life_world = LifeWorldRecorder()
    generator = ScheduleGenerator()
    service = InitialScheduleService(
        life_world,  # type: ignore[arg-type]
        generator,  # type: ignore[arg-type]
        clock=lambda: datetime(2026, 8, 26, 4, tzinfo=UTC),
    )
    fluctlight = FluctlightSnapshot(
        id="fluctlight-utc-eight",
        initialization_mode=InitializationMode.BLANK_SLATE,
        status=FluctlightStatus.ACTIVE,
        identity=Identity(id="fluctlight-utc-eight", timezone="UTC+8"),
        personality=Personality.neutral(),
        behavioral_policy=BehavioralPolicy(),
    )

    schedule = asyncio.run(service.ensure_for(fluctlight))

    assert schedule is not None
    assert generator.calls[0]["timezone"] == "Asia/Shanghai"
