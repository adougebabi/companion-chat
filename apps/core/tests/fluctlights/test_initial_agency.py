import asyncio
from datetime import UTC, datetime

from fluctlight_core.fluctlights.contracts import (
    BehavioralPolicy,
    FluctlightSnapshot,
    FluctlightStatus,
    Identity,
    InitializationMode,
    Personality,
)
from fluctlight_core.fluctlights.creation import InitialAgencyService


class _InnerState:
    def __init__(self) -> None:
        self.goals: list[object] = []
        self.intentions: list[object] = []
        self.goal_transition_actor_ids: list[str] = []
        self.intention_qualification_actor_ids: list[str] = []

    async def goals_and_intentions(self, _fluctlight_id: str):
        return self.goals, self.intentions

    async def create_goal(self, evidence):
        goal = type("Goal", (), {"id": evidence.goal_id, "revision": 0})()
        self.goals.append(goal)
        return goal

    async def transition_goal(self, goal_id, **kwargs):
        self.goal_transition_actor_ids.append(kwargs["actor_id"])
        return next(goal for goal in self.goals if goal.id == goal_id)

    async def create_intention(self, evidence):
        intention = type("Intention", (), {"id": evidence.intention_id, "revision": 0})()
        self.intentions.append(intention)
        return intention

    async def qualify_intention(self, intention_id, **kwargs):
        self.intention_qualification_actor_ids.append(kwargs["actor_id"])
        return next(intention for intention in self.intentions if intention.id == intention_id)


def test_initial_agency_persists_model_owned_goal_and_pending_intention() -> None:
    async def verify() -> None:
        inner_state = _InnerState()
        service = InitialAgencyService(inner_state)  # type: ignore[arg-type]
        fluctlight = FluctlightSnapshot(
            id="fluctlight-1",
            initialization_mode=InitializationMode.LLM_DEFINED,
            status=FluctlightStatus.ACTIVE,
            identity=Identity(id="fluctlight-1"),
            personality=Personality(),
            behavioral_policy=BehavioralPolicy(),
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
        )
        await service.ensure_for(
            fluctlight,
            actor_id="human-owner",
            goals=[{"description": "完成摄影练习", "importance": 0.8, "urgency": 0.4}],
            intentions=[
                {
                    "goal_index": 0,
                    "action": "整理今天想记录的画面",
                    "confidence": 0.7,
                    "expiration_hours": 24,
                }
            ],
        )
        assert len(inner_state.goals) == 1
        assert len(inner_state.intentions) == 1
        assert inner_state.goal_transition_actor_ids == ["human-owner"]
        assert inner_state.intention_qualification_actor_ids == ["human-owner"]

    asyncio.run(verify())
