"""Typed Moments/feed interaction contracts."""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import UTC, datetime
from enum import StrEnum


class MomentVisibility(StrEnum):
    PRIVATE = "private"
    OWNER = "owner"
    PARTICIPANTS = "participants"


class MomentStatus(StrEnum):
    VISIBLE = "visible"
    HIDDEN = "hidden"
    DELETED = "deleted"


class ReactionKind(StrEnum):
    LIKE = "like"
    CARE = "care"
    CELEBRATE = "celebrate"


def _text(value: str, name: str, limit: int = 32_000) -> str:
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{name} is required")
    value = value.strip()
    if len(value) > limit:
        raise ValueError(f"{name} exceeds {limit} characters")
    return value


@dataclass(frozen=True, slots=True)
class Moment:
    id: str
    owner_fluctlight_id: str
    author_actor_id: str
    text: str
    visibility: MomentVisibility = MomentVisibility.OWNER
    status: MomentStatus = MomentStatus.VISIBLE
    media_asset_ids: tuple[str, ...] = ()
    created_at: datetime = field(default_factory=lambda: datetime.now(UTC))

    def __post_init__(self) -> None:
        for name in ("id", "owner_fluctlight_id", "author_actor_id"):
            object.__setattr__(self, name, _text(getattr(self, name), name, 128))
        object.__setattr__(self, "text", _text(self.text, "moment.text"))
        object.__setattr__(self, "visibility", MomentVisibility(self.visibility))
        object.__setattr__(self, "status", MomentStatus(self.status))
        object.__setattr__(
            self,
            "media_asset_ids",
            tuple(_text(item, "media_asset_id", 128) for item in self.media_asset_ids),
        )


@dataclass(frozen=True, slots=True)
class MomentComment:
    id: str
    moment_id: str
    author_actor_id: str
    text: str
    created_at: datetime = field(default_factory=lambda: datetime.now(UTC))


@dataclass(frozen=True, slots=True)
class MomentReaction:
    moment_id: str
    actor_id: str
    kind: ReactionKind = ReactionKind.LIKE
    created_at: datetime = field(default_factory=lambda: datetime.now(UTC))


@dataclass(frozen=True, slots=True)
class UnreadMarker:
    owner_fluctlight_id: str
    actor_id: str
    last_seen_at: datetime | None = None
