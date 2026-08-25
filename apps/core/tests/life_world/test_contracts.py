from datetime import date, datetime, time, timedelta

import pytest
from fluctlight_core.life_world.contracts import (
    ContextSource,
    ScheduleItem,
    ScheduleValidationError,
    ScheduleVersion,
    timezone_or_error,
)


def schedule_items(day: date, timezone: str = "UTC") -> tuple[ScheduleItem, ...]:
    start = datetime.combine(day, time.min, tzinfo=timezone_or_error(timezone))
    middle = start.replace(hour=12)
    end = datetime.combine(day, time.max, tzinfo=timezone_or_error(timezone)) + timedelta(
        microseconds=1
    )
    return (
        ScheduleItem("item-1", start, middle, "free time", "home", item_type="free_time"),
        ScheduleItem("item-2", middle, end, "rest", "home", item_type="rest"),
    )


def test_schedule_requires_explicit_complete_local_day() -> None:
    day = date(2026, 8, 25)
    schedule = ScheduleVersion(
        "schedule-1", "fl-1", day, "UTC", schedule_items(day), "reflection", ("event-1",)
    )
    assert schedule.items[0].start_at.hour == 0
    with pytest.raises(ScheduleValidationError):
        ScheduleVersion(
            "schedule-2",
            "fl-1",
            day,
            "UTC",
            (
                ScheduleItem(
                    "item",
                    datetime(2026, 8, 25, 1, tzinfo=timezone_or_error("UTC")),
                    datetime(2026, 8, 25, 2, tzinfo=timezone_or_error("UTC")),
                    "work",
                    "office",
                ),
            ),
            "reflection",
            ("event-1",),
        )


def test_context_source_is_explicit() -> None:
    assert ContextSource.EVENT.value == "event"
