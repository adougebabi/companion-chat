"""Typed executors for already-frozen autonomous actions."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import date, datetime
from typing import Any

from fluctlight_core.conversations.contracts import MessageDraft, MessageKind
from fluctlight_core.conversations.service import ConversationService
from fluctlight_core.life_world.contracts import ActionStatus, ScheduleItem, ScheduleVersion
from fluctlight_core.life_world.service import LifeWorldService
from fluctlight_core.media.contracts import MediaIntent, MediaKind
from fluctlight_core.media.service import MediaService
from fluctlight_core.memory.contracts import MemoryRecord, MemoryType, MemoryVisibility
from fluctlight_core.memory.service import MemoryService
from fluctlight_core.moments.contracts import Moment, MomentStatus, MomentVisibility
from fluctlight_core.moments.service import MomentsService
from fluctlight_core.relationships.contracts import RelationshipTrend, RelationshipUpdate
from fluctlight_core.relationships.service import RelationshipService


@dataclass(frozen=True, slots=True)
class ActionExecutionResult:
    status: ActionStatus
    reason: str


class AutonomyExecutor:
    """Consume only explicitly typed frozen payloads through public service ports."""

    def __init__(
        self,
        *,
        conversations: ConversationService,
        memory: MemoryService,
        relationships: RelationshipService,
        life_world: LifeWorldService,
        media: MediaService,
        moments: MomentsService,
    ) -> None:
        self._conversations = conversations
        self._memory = memory
        self._relationships = relationships
        self._life_world = life_world
        self._media = media
        self._moments = moments

    async def execute(self, action: Any) -> ActionExecutionResult:
        handlers = {
            "proactive_message": self._proactive_message,
            "memory_candidate": self._memory_candidate,
            "relationship_candidate": self._relationship_candidate,
            "schedule_proposal": self._schedule_proposal,
            "media_request": self._media_request,
            "moment": self._moment,
        }
        handler = handlers.get(action.action_type)
        if handler is None:
            return ActionExecutionResult(ActionStatus.DEFERRED, "action_handler_unavailable")
        await handler(action)
        return ActionExecutionResult(ActionStatus.COMPLETED, "executed")

    async def _proactive_message(self, action: Any) -> None:
        payload = action.payload
        conversation_id = self._text(payload, "conversation_id")
        text = self._text(payload, "text")
        await self._conversations.append_message(
            conversation_id,
            MessageDraft(
                author_actor_id=action.fluctlight_id,
                text=text,
                kind=MessageKind.ASSISTANT,
                idempotency_key=f"autonomy:{action.id}:message",
            ),
            actor_id=action.fluctlight_id,
        )

    async def _memory_candidate(self, action: Any) -> None:
        payload = action.payload
        await self._memory.record(
            MemoryRecord(
                id=f"memory_autonomy_{action.id}",
                owner_fluctlight_id=action.fluctlight_id,
                type=MemoryType(self._text(payload, "type")),
                content=self._text(payload, "content"),
                actor_refs=tuple(self._texts(payload, "actor_refs")),
                conversation_id=payload.get("conversation_id"),
                event_refs=tuple(self._texts(payload, "event_refs")),
                evidence_refs=tuple(self._texts(payload, "evidence_refs")),
                confidence=float(payload["confidence"]),
                importance=float(payload["importance"]),
                emotional_significance=float(payload["emotional_significance"]),
                visibility=MemoryVisibility(self._text(payload, "visibility")),
            )
        )

    async def _relationship_candidate(self, action: Any) -> None:
        payload = action.payload
        await self._relationships.record_update(
            RelationshipUpdate(
                owner_fluctlight_id=action.fluctlight_id,
                target_actor_id=self._text(payload, "target_actor_id"),
                metrics=dict(self._object(payload, "metrics")),
                evidence_refs=tuple(self._texts(payload, "evidence_refs")),
                actor_id=action.fluctlight_id,
                expected_revision=int(payload["expected_revision"]),
                trend=RelationshipTrend(self._text(payload, "trend")),
                summary=payload.get("summary"),
                emotional_association=dict(payload.get("emotional_association", {})),
                idempotency_key=f"autonomy:{action.id}:relationship",
            )
        )

    async def _schedule_proposal(self, action: Any) -> None:
        payload = action.payload
        items = tuple(
            ScheduleItem(
                id=f"schedule_{action.id}:{index}",
                start_at=datetime.fromisoformat(self._text(item, "start_at")),
                end_at=datetime.fromisoformat(self._text(item, "end_at")),
                activity=self._text(item, "activity"),
                scene=self._text(item, "scene"),
                item_type=self._text(item, "item_type"),
                status=self._text(item, "status"),
                priority=float(item["priority"]),
                flexibility=float(item["flexibility"]),
                interruption_cost=float(item["interruption_cost"]),
            )
            for index, item in enumerate(self._objects(payload, "items"))
        )
        await self._life_world.accept_schedule(
            ScheduleVersion(
                id=f"schedule_{action.id}",
                fluctlight_id=action.fluctlight_id,
                local_date=date.fromisoformat(self._text(payload, "local_date")),
                timezone=self._text(payload, "timezone"),
                items=items,
                generated_from=self._text(payload, "generated_from"),
                evidence_refs=tuple(self._texts(payload, "evidence_refs")),
            )
        )

    async def _media_request(self, action: Any) -> None:
        payload = action.payload
        await self._media.request_generation(
            MediaIntent(
                id=f"media_intent_{action.id}",
                owner_fluctlight_id=action.fluctlight_id,
                kind=MediaKind(self._text(payload, "kind")),
                mime_type=self._text(payload, "mime_type"),
                prompt=self._text(payload, "prompt"),
                provider_request_id=action.provider_request_id,
                workflow_id=action.workflow_id,
            )
        )

    async def _moment(self, action: Any) -> None:
        payload = action.payload
        await self._moments.create(
            Moment(
                id=f"moment_{action.id}",
                owner_fluctlight_id=action.fluctlight_id,
                author_actor_id=action.fluctlight_id,
                text=self._text(payload, "text"),
                visibility=MomentVisibility(self._text(payload, "visibility")),
                status=MomentStatus.VISIBLE,
                media_asset_ids=tuple(self._texts(payload, "media_asset_ids")),
            )
        )

    @staticmethod
    def _text(payload: Any, name: str) -> str:
        if not isinstance(payload, dict) or not isinstance(payload.get(name), str):
            raise ValueError(f"autonomy payload requires {name}")
        return payload[name]

    @classmethod
    def _texts(cls, payload: Any, name: str) -> list[str]:
        if not isinstance(payload, dict) or not isinstance(payload.get(name), list):
            raise ValueError(f"autonomy payload requires {name}")
        values = payload[name]
        if not all(isinstance(value, str) for value in values):
            raise ValueError(f"autonomy payload {name} must contain text")
        return list(values)

    @staticmethod
    def _object(payload: Any, name: str) -> dict[str, Any]:
        if not isinstance(payload, dict) or not isinstance(payload.get(name), dict):
            raise ValueError(f"autonomy payload requires {name}")
        return dict(payload[name])

    @classmethod
    def _objects(cls, payload: Any, name: str) -> list[dict[str, Any]]:
        if not isinstance(payload, dict) or not isinstance(payload.get(name), list):
            raise ValueError(f"autonomy payload requires {name}")
        values = payload[name]
        if not all(isinstance(value, dict) for value in values):
            raise ValueError(f"autonomy payload {name} must contain objects")
        return [dict(value) for value in values]
