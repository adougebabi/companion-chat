import asyncio
from datetime import UTC, date, datetime
from types import SimpleNamespace

from fluctlight_core.cognition.background import DailyLifeReviewService, daily_review_intent
from fluctlight_core.cognition.contracts import InboxStatus, ProcessOutcome
from fluctlight_core.fluctlights.contracts import (
    BehavioralPolicy,
    FluctlightSnapshot,
    FluctlightStatus,
    Identity,
    InitializationMode,
    Personality,
)


class _Fluctlights:
    def __init__(self) -> None:
        self.snapshot = FluctlightSnapshot(
            id="fluctlight-1",
            initialization_mode=InitializationMode.LLM_DEFINED,
            status=FluctlightStatus.ACTIVE,
            identity=Identity(id="fluctlight-1", name="测试", timezone="Asia/Shanghai"),
            personality=Personality(empathy=0.8),
            behavioral_policy=BehavioralPolicy(response_style="温和简洁"),
        )

    async def get(self, fluctlight_id: str) -> FluctlightSnapshot:
        assert fluctlight_id == "fluctlight-1"
        return self.snapshot

    async def owner_actor_id(self, fluctlight_id: str) -> str:
        assert fluctlight_id == "fluctlight-1"
        return "human-owner"


class _Conversations:
    async def direct_conversation_id(self, **kwargs: str) -> str:
        assert kwargs == {
            "owner_actor_id": "human-owner",
            "fluctlight_actor_id": "fluctlight-1",
        }
        return "conversation-direct"


class _InnerState:
    async def goals_and_intentions(self, _fluctlight_id: str):
        return (
            [
                SimpleNamespace(
                    id="goal-1", description="完成摄影练习", status=SimpleNamespace(value="active")
                )
            ],
            [
                SimpleNamespace(
                    id="intention-1", action="整理画面", status=SimpleNamespace(value="pending")
                )
            ],
        )


class _Cognition:
    def __init__(self, existing_status: InboxStatus | None = None) -> None:
        self.fact = None
        self.existing_status = existing_status
        self.process_calls = 0

    async def inbox_fact_status(self, _fact_id: str, *, fluctlight_id: str):
        assert fluctlight_id == "fluctlight-1"
        return self.existing_status

    async def enqueue(self, fact):
        self.fact = fact
        return SimpleNamespace(status=InboxStatus.PENDING)

    async def process_next(self, fluctlight_id: str, *, worker_id: str):
        self.process_calls += 1
        assert (fluctlight_id, worker_id) == ("fluctlight-1", "lifecycle")
        return ProcessOutcome(
            InboxStatus.COMPLETED,
            action=SimpleNamespace(action_type=SimpleNamespace(value="moment")),
        )


def test_daily_review_creates_one_typed_background_fact_for_model_decision() -> None:
    async def verify() -> None:
        cognition = _Cognition()
        service = DailyLifeReviewService(_Fluctlights(), _Conversations(), _InnerState(), cognition)
        schedule = SimpleNamespace(
            id="schedule-1",
            local_date=date(2026, 8, 27),
            timezone="Asia/Shanghai",
            items=(
                SimpleNamespace(
                    start_at=datetime(2026, 8, 27, tzinfo=UTC),
                    end_at=datetime(2026, 8, 27, 1, tzinfo=UTC),
                    activity="整理照片",
                    scene="家",
                ),
            ),
        )

        result = await service.review_current_day("fluctlight-1", schedule)

        assert result == {
            "status": "completed",
            "fact_id": "background:daily-review:fluctlight-1:2026-08-27",
            "action_type": "moment",
        }
        assert cognition.fact.idempotency_key == "background:daily-review:fluctlight-1:2026-08-27"
        context = cognition.fact.payload["background_context"]
        assert context["conversation_id"] == "conversation-direct"
        assert context["goals"][0]["description"] == "完成摄影练习"
        assert (
            cognition.fact.payload["persona_profile"]["behavioral_policy"]["response_style"]
            == "温和简洁"
        )

    asyncio.run(verify())


def test_daily_review_replays_existing_fact_without_rebuilding_mutable_context() -> None:
    async def verify() -> None:
        cognition = _Cognition(InboxStatus.COMPLETED)

        class NoReadConversations:
            async def direct_conversation_id(self, **_kwargs: str):
                raise AssertionError("existing daily review must not read conversation state")

        class NoReadInnerState:
            async def goals_and_intentions(self, _fluctlight_id: str):
                raise AssertionError("existing daily review must not read inner state")

        service = DailyLifeReviewService(
            _Fluctlights(), NoReadConversations(), NoReadInnerState(), cognition
        )
        schedule = SimpleNamespace(local_date=date(2026, 8, 27))

        result = await service.review_current_day("fluctlight-1", schedule)

        assert result == {
            "status": "already_processed",
            "fact_id": "background:daily-review:fluctlight-1:2026-08-27",
        }
        assert cognition.process_calls == 0

    asyncio.run(verify())


def test_daily_review_replays_existing_pending_fact_without_rebuilding_context() -> None:
    async def verify() -> None:
        cognition = _Cognition(InboxStatus.PENDING)

        class NoReadConversations:
            async def direct_conversation_id(self, **_kwargs: str):
                raise AssertionError("pending replay must not read conversation state")

        class NoReadInnerState:
            async def goals_and_intentions(self, _fluctlight_id: str):
                raise AssertionError("pending replay must not read inner state")

        service = DailyLifeReviewService(
            _Fluctlights(), NoReadConversations(), NoReadInnerState(), cognition
        )
        schedule = SimpleNamespace(local_date=date(2026, 8, 27))

        result = await service.review_current_day("fluctlight-1", schedule)

        assert result["fact_id"] == "background:daily-review:fluctlight-1:2026-08-27"
        assert cognition.process_calls == 1

    asyncio.run(verify())


def test_daily_review_intent_is_stable_for_one_local_date() -> None:
    intent = daily_review_intent("fluctlight-1", "2026-08-27")

    assert intent.intent_id == "daily_review_intent:fluctlight-1:2026-08-27"
    assert intent.workflow_id == "daily_review:fluctlight-1:2026-08-27"
    assert intent.task_queue == "lifecycle"
    assert intent.intent_type == "daily_review.current_day"
