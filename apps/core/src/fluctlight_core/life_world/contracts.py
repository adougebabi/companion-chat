"""Framework-free Life World, Schedule and Context contracts."""

from __future__ import annotations

from collections.abc import Mapping, Sequence
from dataclasses import dataclass, field
from datetime import UTC, date, datetime, time, timedelta
from enum import StrEnum
from typing import Any
from uuid import uuid4
from zoneinfo import ZoneInfo, ZoneInfoNotFoundError


class LifeWorldError(RuntimeError):
    pass


class ScheduleValidationError(ValueError):
    pass


class EventStatus(StrEnum):
    CONFIRMED = "confirmed"
    CANCELLED = "cancelled"


class ScheduleStatus(StrEnum):
    PROPOSED = "proposed"
    ACCEPTED = "accepted"
    SUPERSEDED = "superseded"


class ContextSource(StrEnum):
    EVENT = "event"
    SCHEDULE = "schedule"
    PENDING = "pending"


class ActionStatus(StrEnum):
    FROZEN = "frozen"
    PAUSED = "paused"
    DEFERRED = "deferred"
    COMPLETED = "completed"
    CANCELLED = "cancelled"
    FAILED = "failed"


def _text(value: str, name: str, limit: int = 512) -> str:
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{name} is required")
    value = value.strip()
    if len(value) > limit:
        raise ValueError(f"{name} exceeds {limit} characters")
    return value


def _aware(value: datetime, name: str) -> datetime:
    if value.tzinfo is None or value.utcoffset() is None:
        raise ValueError(f"{name} must be timezone-aware")
    return value


def _refs(values: Sequence[str], name: str, required: bool = False) -> tuple[str, ...]:
    refs = tuple(_text(value, name, 512) for value in values)
    if required and not refs:
        raise ValueError(f"{name} requires evidence")
    if len(refs) != len(set(refs)):
        raise ValueError(f"{name} contains duplicate references")
    return refs


def timezone_or_error(value: str) -> ZoneInfo:
    try:
        return ZoneInfo(value)
    except ZoneInfoNotFoundError as exc:
        raise ScheduleValidationError(f"unknown timezone: {value}") from exc


@dataclass(frozen=True, slots=True)
class WorldEvent:
    id: str
    fluctlight_id: str
    kind: str
    start_at: datetime
    end_at: datetime
    scene: str | None
    activity: str | None
    location: str | None
    status: EventStatus = EventStatus.CONFIRMED
    evidence_refs: tuple[str, ...] = ()

    def __post_init__(self) -> None:
        for name in ("id", "fluctlight_id", "kind"):
            object.__setattr__(self, name, _text(getattr(self, name), name, 128))
        object.__setattr__(self, "start_at", _aware(self.start_at, "start_at"))
        object.__setattr__(self, "end_at", _aware(self.end_at, "end_at"))
        if self.end_at <= self.start_at:
            raise ValueError("event end must follow start")
        object.__setattr__(self, "status", EventStatus(self.status))
        object.__setattr__(self, "evidence_refs", _refs(self.evidence_refs, "event.evidence_refs"))

    def contains(self, instant: datetime) -> bool:
        instant = _aware(instant, "instant")
        return self.status is EventStatus.CONFIRMED and self.start_at <= instant < self.end_at


@dataclass(frozen=True, slots=True)
class ScheduleItem:
    id: str
    start_at: datetime
    end_at: datetime
    activity: str
    scene: str
    item_type: str = "planned"
    status: str = "planned"
    priority: float = 0.5
    flexibility: float = 0.5
    interruption_cost: float = 0.5

    def __post_init__(self) -> None:
        object.__setattr__(self, "id", _text(self.id, "schedule.item.id", 128))
        object.__setattr__(self, "start_at", _aware(self.start_at, "schedule.item.start_at"))
        object.__setattr__(self, "end_at", _aware(self.end_at, "schedule.item.end_at"))
        if self.end_at <= self.start_at:
            raise ScheduleValidationError("schedule item end must follow start")
        for name in ("activity", "scene", "item_type", "status"):
            object.__setattr__(self, name, _text(getattr(self, name), f"schedule.item.{name}", 128))
        for name in ("priority", "flexibility", "interruption_cost"):
            value = float(getattr(self, name))
            if not 0.0 <= value <= 1.0:
                raise ScheduleValidationError(f"schedule item {name} must be between 0 and 1")
            object.__setattr__(self, name, value)


@dataclass(frozen=True, slots=True)
class ScheduleVersion:
    id: str
    fluctlight_id: str
    local_date: date
    timezone: str
    items: tuple[ScheduleItem, ...]
    generated_from: str
    evidence_refs: tuple[str, ...]
    status: ScheduleStatus = ScheduleStatus.PROPOSED
    previous_version_id: str | None = None
    revision: int = 0
    generated_at: datetime = field(default_factory=lambda: datetime.now(UTC))
    reschedule_policy: Mapping[str, Any] = field(default_factory=dict)

    def __post_init__(self) -> None:
        object.__setattr__(self, "id", _text(self.id, "schedule.id", 128))
        object.__setattr__(self, "fluctlight_id", _text(self.fluctlight_id, "fluctlight_id", 128))
        timezone_or_error(self.timezone)
        object.__setattr__(
            self, "generated_from", _text(self.generated_from, "generated_from", 128)
        )
        object.__setattr__(
            self, "evidence_refs", _refs(self.evidence_refs, "evidence_refs", required=True)
        )
        object.__setattr__(self, "status", ScheduleStatus(self.status))
        object.__setattr__(self, "generated_at", _aware(self.generated_at, "generated_at"))
        if self.revision < 0:
            raise ValueError("schedule revision cannot be negative")
        if self.items:
            self.validate_full_day()

    def validate_full_day(self) -> None:
        zone = timezone_or_error(self.timezone)
        day_start = datetime.combine(self.local_date, time.min, tzinfo=zone)
        next_day = datetime.combine(self.local_date + timedelta(days=1), time.min, tzinfo=zone)
        ordered = sorted(self.items, key=lambda item: item.start_at)
        if ordered[0].start_at != day_start or ordered[-1].end_at != next_day:
            raise ScheduleValidationError("accepted schedule must cover the complete local day")
        for previous, current in zip(ordered, ordered[1:]):
            if current.start_at != previous.end_at:
                raise ScheduleValidationError(
                    "schedule items must be contiguous with explicit gaps"
                )


@dataclass(frozen=True, slots=True)
class PresenceOverlay:
    actor_id: str
    current_task: str | None = None
    user_presence: str | None = None


@dataclass(frozen=True, slots=True)
class ContextSnapshot:
    fluctlight_id: str
    source: ContextSource
    instant: datetime
    event_id: str | None = None
    schedule_id: str | None = None
    scene: str | None = None
    activity: str | None = None
    location: str | None = None
    user_presence: str | None = None
    current_task: str | None = None


@dataclass(frozen=True, slots=True)
class AutonomyPolicy:
    mode: str = "active"
    allowed_actions: frozenset[str] = frozenset(
        {
            "proactive_message",
            "memory_candidate",
            "relationship_candidate",
            "schedule_proposal",
            "media_request",
            "moment",
        }
    )
    budget_remaining: float = 1.0
    quiet_hours: tuple[tuple[time, time], ...] = ()
    cooldown_until: datetime | None = None
    concurrency_limit: int = 1

    def allows(self, action_type: str, at: datetime, cost: float) -> tuple[bool, str]:
        if self.mode != "active":
            return False, "autonomy_paused"
        if action_type not in self.allowed_actions:
            return False, "action_not_allowed"
        if cost < 0 or cost > self.budget_remaining:
            return False, "budget_exhausted"
        if self.cooldown_until and at < self.cooldown_until:
            return False, "cooldown_active"
        local_time = at.timetz().replace(tzinfo=None)
        if any(start <= local_time < end for start, end in self.quiet_hours if start <= end):
            return False, "quiet_hours"
        return True, "allowed"


@dataclass(frozen=True, slots=True)
class FrozenAutonomousAction:
    id: str
    fluctlight_id: str
    action_type: str
    payload: Mapping[str, Any]
    policy_snapshot: Mapping[str, Any]
    expected_revisions: Mapping[str, int]
    status: ActionStatus = ActionStatus.FROZEN
    workflow_id: str = field(default_factory=lambda: f"autonomy_workflow_{uuid4().hex}")
    provider_request_id: str = field(default_factory=lambda: f"autonomy_provider_{uuid4().hex}")
    created_at: datetime = field(default_factory=lambda: datetime.now(UTC))


@dataclass(frozen=True, slots=True)
class AutonomousActionRequest:
    fluctlight_id: str
    action_id: str
    action_type: str
    payload: Mapping[str, Any]
    policy: AutonomyPolicy
    expected_revisions: Mapping[str, int]
    cost: float = 0.0
    requested_at: datetime = field(default_factory=lambda: datetime.now(UTC))


@dataclass(frozen=True, slots=True)
class AutonomyDecision:
    accepted: bool
    reason_code: str
    action: FrozenAutonomousAction | None = None
