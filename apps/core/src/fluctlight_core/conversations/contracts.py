"""Framework-free Conversation, Participant, Message and turn contracts."""

from __future__ import annotations

from collections.abc import Sequence
from dataclasses import dataclass, field
from datetime import UTC, datetime
from enum import StrEnum
from typing import Any, Protocol
from uuid import uuid4


class ConversationError(RuntimeError):
    """Base conversation error."""


class ConversationNotFoundError(ConversationError):
    pass


class ConversationAuthorizationError(ConversationError):
    pass


class ConversationConflictError(ConversationError):
    pass


class ConversationProviderError(ConversationError):
    pass


class ParticipantRole(StrEnum):
    OWNER = "owner"
    MEMBER = "member"


class ParticipantStatus(StrEnum):
    ACTIVE = "active"
    LEFT = "left"


class MessageKind(StrEnum):
    USER = "user"
    ASSISTANT = "assistant"
    SYSTEM = "system"
    ACTION_RESULT = "action_result"
    MEDIA_REFERENCE = "media_reference"


class CoreStreamType(StrEnum):
    TOKEN = "token"
    ACTION_RESULT = "action_result"
    COMPLETED = "completed"
    ERROR = "error"
    HEARTBEAT = "heartbeat"


class BrowserStreamType(StrEnum):
    TOKEN = "token"
    MESSAGE = "message"
    MEDIA = "media"
    COMPLETED = "completed"
    ERROR = "error"
    HEARTBEAT = "heartbeat"


def _text(value: str, name: str, limit: int = 4096) -> str:
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


def _refs(values: Sequence[str]) -> tuple[str, ...]:
    result = tuple(_text(value, "attachment_ref", 512) for value in values)
    if len(result) != len(set(result)):
        raise ValueError("attachment references must be unique")
    return result


@dataclass(frozen=True, slots=True)
class Conversation:
    id: str
    created_by_actor_id: str
    title: str | None = None
    revision: int = 0
    created_at: datetime = field(default_factory=lambda: datetime.now(UTC))
    updated_at: datetime = field(default_factory=lambda: datetime.now(UTC))

    def __post_init__(self) -> None:
        object.__setattr__(self, "id", _text(self.id, "conversation.id", 128))
        object.__setattr__(
            self, "created_by_actor_id", _text(self.created_by_actor_id, "created_by_actor_id", 128)
        )
        if self.title is not None:
            object.__setattr__(self, "title", _text(self.title, "title", 256))
        if self.revision < 0:
            raise ValueError("conversation revision cannot be negative")
        object.__setattr__(self, "created_at", _aware(self.created_at, "created_at"))
        object.__setattr__(self, "updated_at", _aware(self.updated_at, "updated_at"))


@dataclass(frozen=True, slots=True)
class Participant:
    conversation_id: str
    actor_id: str
    role: ParticipantRole = ParticipantRole.MEMBER
    status: ParticipantStatus = ParticipantStatus.ACTIVE
    joined_at: datetime = field(default_factory=lambda: datetime.now(UTC))
    left_at: datetime | None = None

    def __post_init__(self) -> None:
        object.__setattr__(
            self, "conversation_id", _text(self.conversation_id, "conversation_id", 128)
        )
        object.__setattr__(self, "actor_id", _text(self.actor_id, "actor_id", 128))
        object.__setattr__(self, "role", ParticipantRole(self.role))
        object.__setattr__(self, "status", ParticipantStatus(self.status))
        object.__setattr__(self, "joined_at", _aware(self.joined_at, "joined_at"))
        if self.left_at is not None:
            object.__setattr__(self, "left_at", _aware(self.left_at, "left_at"))


@dataclass(frozen=True, slots=True)
class MessageDraft:
    author_actor_id: str
    text: str
    kind: MessageKind
    attachment_refs: tuple[str, ...] = ()
    idempotency_key: str = field(default_factory=lambda: f"message_{uuid4().hex}")

    def __post_init__(self) -> None:
        object.__setattr__(
            self, "author_actor_id", _text(self.author_actor_id, "author_actor_id", 128)
        )
        object.__setattr__(self, "text", _text(self.text, "message.text", 32_000))
        object.__setattr__(self, "kind", MessageKind(self.kind))
        object.__setattr__(self, "attachment_refs", _refs(self.attachment_refs))
        object.__setattr__(
            self, "idempotency_key", _text(self.idempotency_key, "idempotency_key", 256)
        )


@dataclass(frozen=True, slots=True)
class Message:
    id: str
    conversation_id: str
    sequence: int
    author_actor_id: str
    text: str
    kind: MessageKind
    attachment_refs: tuple[str, ...] = ()
    created_at: datetime = field(default_factory=lambda: datetime.now(UTC))
    idempotency_key: str | None = None

    def __post_init__(self) -> None:
        for name in ("id", "conversation_id", "author_actor_id"):
            object.__setattr__(self, name, _text(getattr(self, name), name, 128))
        if self.sequence < 1:
            raise ValueError("message sequence must be positive")
        object.__setattr__(self, "text", _text(self.text, "message.text", 32_000))
        object.__setattr__(self, "kind", MessageKind(self.kind))
        object.__setattr__(self, "attachment_refs", _refs(self.attachment_refs))
        object.__setattr__(self, "created_at", _aware(self.created_at, "created_at"))


@dataclass(frozen=True, slots=True)
class ConversationPage:
    conversation: Conversation
    participants: tuple[Participant, ...]
    messages: tuple[Message, ...]
    next_before_sequence: int | None = None


@dataclass(frozen=True, slots=True)
class ConversationTurn:
    conversation_id: str
    actor_id: str
    text: str
    fluctlight_id: str | None = None
    attachment_refs: tuple[str, ...] = ()
    idempotency_key: str = field(default_factory=lambda: f"turn_{uuid4().hex}")
    turn_id: str = field(default_factory=lambda: f"turn_{uuid4().hex}")
    correlation_id: str = field(default_factory=lambda: f"turn_corr_{uuid4().hex}")

    def __post_init__(self) -> None:
        object.__setattr__(
            self, "conversation_id", _text(self.conversation_id, "conversation_id", 128)
        )
        object.__setattr__(self, "actor_id", _text(self.actor_id, "actor_id", 128))
        object.__setattr__(self, "text", _text(self.text, "turn.text", 32_000))
        if self.fluctlight_id is not None:
            object.__setattr__(
                self, "fluctlight_id", _text(self.fluctlight_id, "fluctlight_id", 128)
            )
        object.__setattr__(self, "attachment_refs", _refs(self.attachment_refs))
        for name in ("idempotency_key", "turn_id", "correlation_id"):
            object.__setattr__(self, name, _text(getattr(self, name), name, 256))


@dataclass(frozen=True, slots=True)
class TurnResponse:
    messages: tuple[MessageDraft, ...]
    events: tuple[dict[str, Any], ...] = ()


@dataclass(frozen=True, slots=True)
class TurnResult:
    turn: ConversationTurn
    user_message: Message
    assistant_messages: tuple[Message, ...]
    events: tuple[dict[str, Any], ...]


@dataclass(frozen=True, slots=True)
class TurnStreamEvent:
    type: str
    message: Message | None = None
    text: str | None = None
    message_ids: tuple[str, ...] = ()

    def __post_init__(self) -> None:
        if self.type not in {"action_result", "token", "completed"}:
            raise ValueError("unsupported turn stream event")
        if self.type == "action_result" and self.message is None:
            raise ValueError("action_result requires a message")
        if self.type == "token" and not self.text:
            raise ValueError("token requires visible text")


class TurnResponder(Protocol):
    async def respond(
        self, turn: ConversationTurn, history: tuple[Message, ...]
    ) -> TurnResponse: ...
