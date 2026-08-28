"""Typed background facts that re-enter the normal cognition inbox."""

from __future__ import annotations

from datetime import UTC, datetime
from typing import Any
from zoneinfo import ZoneInfo

from fluctlight_core.cognition.contracts import CognitionFact, InboxStatus
from fluctlight_core.fluctlights.contracts import FluctlightStatus
from fluctlight_core.platform.outbox import CommittedWorkflowIntent, commit_workflow_intent
from fluctlight_core.platform.persistence import UnitOfWorkFactory
from fluctlight_core.platform.timezones import canonical_timezone


class DailyLifeReviewService:
    """Turn a ready local-day Schedule into one LLM-owned autonomy opportunity."""

    def __init__(
        self, fluctlights: Any, conversations: Any, inner_state: Any, cognition: Any
    ) -> None:
        self._fluctlights = fluctlights
        self._conversations = conversations
        self._inner_state = inner_state
        self._cognition = cognition

    async def review_current_day(self, fluctlight_id: str, schedule: Any) -> dict[str, str]:
        fluctlight = await self._fluctlights.get(fluctlight_id)
        if fluctlight.status is not FluctlightStatus.ACTIVE:
            return {"status": "inactive"}
        local_date = schedule.local_date.isoformat()
        fact_id = f"background:daily-review:{fluctlight_id}:{local_date}"
        existing_status = await self._cognition.inbox_fact_status(
            fact_id, fluctlight_id=fluctlight_id
        )
        if existing_status is not None and existing_status is not InboxStatus.PENDING:
            return {"status": "already_processed", "fact_id": fact_id}

        if existing_status is InboxStatus.PENDING:
            outcome = await self._cognition.process_next(fluctlight_id, worker_id="lifecycle")
            if outcome is None:
                return {"status": "queued", "fact_id": fact_id}
            return {
                "status": outcome.status.value,
                "fact_id": fact_id,
                "action_type": outcome.action.action_type.value if outcome.action else "no_op",
            }

        owner_actor_id = await self._fluctlights.owner_actor_id(fluctlight_id)
        conversation_id = await self._conversations.direct_conversation_id(
            owner_actor_id=owner_actor_id,
            fluctlight_actor_id=fluctlight_id,
        )
        goals, intentions = await self._inner_state.goals_and_intentions(fluctlight_id)
        identity = fluctlight.identity.as_payload()
        persona_profile = {
            "identity": {key: value for key, value in identity.items() if key != "id"},
            "personality": fluctlight.personality.as_payload(),
            "behavioral_policy": fluctlight.behavioral_policy.as_payload(),
            "life_profile": fluctlight.life_profile.as_payload(),
        }
        background_context = {
            "kind": "daily_schedule_ready",
            "local_date": local_date,
            "timezone": schedule.timezone,
            "schedule_id": schedule.id,
            "schedule_items": [
                {
                    "start_at": item.start_at.isoformat(),
                    "end_at": item.end_at.isoformat(),
                    "activity": item.activity,
                    "scene": item.scene,
                }
                for item in schedule.items
            ],
            "goals": [
                {"id": goal.id, "description": goal.description, "status": goal.status.value}
                for goal in goals
                if goal.status.value == "active"
            ],
            "intentions": [
                {"id": intention.id, "action": intention.action, "status": intention.status.value}
                for intention in intentions
                if intention.status.value == "pending"
            ],
            "conversation_id": conversation_id,
        }
        fact = CognitionFact(
            id=fact_id,
            fluctlight_id=fluctlight_id,
            event_type="life_world.daily_review",
            payload={
                "background_context": background_context,
                "persona_profile": persona_profile,
            },
            causation_id=f"schedule:{schedule.id}",
            correlation_id=f"daily-review:{fluctlight_id}:{local_date}",
            idempotency_key=fact_id,
            occurred_at=datetime.now(UTC),
        )
        enqueued = await self._cognition.enqueue(fact)
        if enqueued.status is not InboxStatus.PENDING:
            return {"status": "already_processed", "fact_id": fact_id}
        outcome = await self._cognition.process_next(fluctlight_id, worker_id="lifecycle")
        if outcome is None:
            return {"status": "queued", "fact_id": fact_id}
        return {
            "status": outcome.status.value,
            "fact_id": fact_id,
            "action_type": outcome.action.action_type.value if outcome.action else "no_op",
        }


class DailyLifeReviewRegistrar:
    """Commit one current-local-day review workflow request per active Fluctlight."""

    def __init__(self, unit_of_work: UnitOfWorkFactory) -> None:
        self._unit_of_work = unit_of_work

    async def register(self, fluctlight: Any) -> CommittedWorkflowIntent:
        timezone = canonical_timezone(
            str(fluctlight.identity.as_payload().get("timezone") or "Asia/Shanghai")
        )
        local_date = datetime.now(UTC).astimezone(ZoneInfo(timezone)).date()
        intent = daily_review_intent(fluctlight.id, local_date.isoformat())
        async with self._unit_of_work.begin(command_id=intent.intent_id) as tx:
            persisted = await commit_workflow_intent(tx.session, intent)
            await tx.commit()
        return persisted

    async def register_active(self, fluctlights: list[Any]) -> int:
        for fluctlight in fluctlights:
            await self.register(fluctlight)
        return len(fluctlights)


def daily_review_intent(fluctlight_id: str, local_date: str) -> CommittedWorkflowIntent:
    if not isinstance(fluctlight_id, str) or not fluctlight_id.strip():
        raise ValueError("daily review requires fluctlight_id")
    if not isinstance(local_date, str) or len(local_date) != 10:
        raise ValueError("daily review requires local_date")
    return CommittedWorkflowIntent(
        intent_id=f"daily_review_intent:{fluctlight_id}:{local_date}",
        workflow_id=f"daily_review:{fluctlight_id}:{local_date}",
        task_queue="lifecycle",
        intent_type="daily_review.current_day",
        payload={"fluctlight_id": fluctlight_id, "local_date": local_date},
    )
