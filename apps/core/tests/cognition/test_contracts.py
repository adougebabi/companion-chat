from datetime import UTC, datetime

import pytest
from fluctlight_core.cognition.contracts import (
    ActionType,
    CognitionFact,
    DecisionProposal,
    FrozenAction,
    ReflectionWindow,
    stable_action_id,
    stable_provider_request_id,
)
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
