import asyncio
from datetime import UTC, datetime, timedelta

import pytest
from fluctlight_core.autonomy.bridge import CognitionAutonomyBridge
from fluctlight_core.cognition.contracts import (
    ActionType,
    CognitionFact,
    DecisionProposal,
    FrozenAction,
    ReflectionWindow,
    stable_action_id,
    stable_provider_request_id,
)
from fluctlight_core.cognition.service import CognitionService
from fluctlight_core.inner_state.contracts import (
    AffectDirection,
    Appraisal,
    SemanticAssessment,
    SemanticPerception,
)


def semantic_assessment(event_id: str = "fact-1", key: str = "fact-1") -> SemanticAssessment:
    return SemanticAssessment(
        schema_version="semantic.assessment.v1",
        perception=SemanticPerception(event_kind="message", observed_intent="inform"),
        appraisal=Appraisal(
            relevance=0.5,
            goal_congruence=0.5,
            reward=0.5,
            loss=0.1,
            social_threat=0.1,
            controllability=0.5,
            responsibility=0.5,
            relationship_significance=0.5,
            expected_effect=0.5,
        ),
        direction=AffectDirection.NEUTRAL,
        strength=0.5,
        confidence=0.8,
        evidence_refs=(f"event:{event_id}",),
        model="test-model",
        model_version="v1",
        prompt_version="prompt-v1",
        source_event_id=event_id,
        idempotency_key=key,
    )


def test_fact_and_decision_require_typed_values() -> None:
    fact = CognitionFact(
        id="fact-1",
        fluctlight_id="fl-1",
        event_type="conversation.message",
        payload={"text": "hello"},
        causation_id="message-1",
        correlation_id="corr-1",
        idempotency_key="fact-1",
        occurred_at=datetime.now(UTC),
    )
    assert fact.payload == {"text": "hello"}
    decision = DecisionProposal(
        action_type=ActionType.REPLY,
        payload={"channel": "conversation"},
        confidence=0.9,
        evidence_refs=("fact-1",),
    )
    assert decision.action_type is ActionType.REPLY
    with pytest.raises(ValueError):
        DecisionProposal(
            action_type=ActionType.REPLY,
            payload={},
            confidence=1.1,
            evidence_refs=("fact-1",),
        )


def test_action_and_provider_ids_are_stable() -> None:
    action_id = stable_action_id("fact-1", "decision-1")
    assert action_id == stable_action_id("fact-1", "decision-1")
    assert stable_provider_request_id(action_id) == stable_provider_request_id(action_id)
    action = FrozenAction(
        action_id=action_id,
        decision_id="decision-1",
        inbox_id="fact-1",
        fluctlight_id="fl-1",
        action_type=ActionType.REPLY,
        payload={"text": "visible"},
        state_revision=4,
        provider_request_id=stable_provider_request_id(action_id),
    )
    assert action.state_revision == 4


def test_reflection_windows_reject_invalid_ranges() -> None:
    with pytest.raises(ValueError):
        ReflectionWindow("fl-1", from_sequence=3, to_sequence=2, base_state_revision=1, watermark=2)


def test_active_cognition_writer_lease_blocks_same_and_different_worker_reclaim() -> None:
    now = datetime.now(UTC)
    active = {"writer_owner": "worker-a", "writer_lease_until": now + timedelta(seconds=1)}
    expired = {"writer_owner": "worker-a", "writer_lease_until": now - timedelta(seconds=1)}

    assert CognitionService._lease_is_active(active, now) is True
    assert CognitionService._lease_is_active(expired, now) is False


class _BridgeSettings:
    async def runtime_value(self, key: str):
        assert key == "product.autonomy"
        return {"mode": "active", "allowed_actions": ["memory_candidate"]}


class _BridgeAutonomy:
    def __init__(self) -> None:
        self.request = None

    async def freeze_action(self, request):
        self.request = request


def test_cognition_autonomy_bridge_routes_only_explicit_candidate_actions() -> None:
    autonomy = _BridgeAutonomy()
    bridge = CognitionAutonomyBridge(autonomy, _BridgeSettings())
    action = FrozenAction(
        action_id="action-1",
        decision_id="decision-1",
        inbox_id="fact-1",
        fluctlight_id="fl-1",
        action_type=ActionType.MEMORY_CANDIDATE,
        payload={"content": "explicit", "cost": 0.2},
        state_revision=4,
        provider_request_id="provider-1",
    )

    asyncio.run(bridge(action))
    assert autonomy.request.action_id == "autonomy_action-1"
    assert autonomy.request.expected_revisions == {"cognition": 4}

    asyncio.run(
        bridge(
            FrozenAction(
                action_id="action-2",
                decision_id="decision-2",
                inbox_id="fact-2",
                fluctlight_id="fl-1",
                action_type=ActionType.REPLY,
                payload={"text": "visible"},
                state_revision=5,
                provider_request_id="provider-2",
            )
        )
    )
    assert autonomy.request.action_id == "autonomy_action-1"
