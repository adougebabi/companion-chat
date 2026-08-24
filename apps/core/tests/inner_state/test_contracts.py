from datetime import UTC, datetime, timedelta

import pytest
from fluctlight_core.inner_state.contracts import (
    AffectDirection,
    Appraisal,
    DriveAssessment,
    DriveName,
    EventTrigger,
    GoalEvidence,
    GoalSource,
    InnerStateValidationError,
    IntentionEvidence,
    NumericPolicyError,
    SemanticAssessment,
    SemanticPerception,
    SemanticTrigger,
    TimeTrigger,
)


def assessment(**overrides) -> SemanticAssessment:
    values = {
        "schema_version": "semantic.assessment.v1",
        "perception": SemanticPerception(
            event_kind="message",
            observed_intent="inform",
            sentiment="warm",
            social_signals=("connection",),
        ),
        "appraisal": Appraisal(
            relevance=0.8,
            goal_congruence=0.7,
            reward=0.8,
            loss=0.1,
            social_threat=0.1,
            controllability=0.6,
            responsibility=0.5,
            relationship_significance=0.7,
            expected_effect=0.8,
        ),
        "direction": AffectDirection.POSITIVE,
        "strength": 0.8,
        "confidence": 0.9,
        "evidence_refs": ("event:1",),
        "model": "test-model",
        "model_version": "v1",
        "prompt_version": "prompt-v1",
        "source_event_id": "event-1",
        "idempotency_key": "assessment-1",
    }
    values.update(overrides)
    return SemanticAssessment(**values)


def test_pad_and_normalized_values_have_canonical_ranges() -> None:
    with pytest.raises(ValueError):
        Appraisal(
            relevance=1.1,
            goal_congruence=0.0,
            reward=0.0,
            loss=0.0,
            social_threat=0.0,
            controllability=0.0,
            responsibility=0.0,
            relationship_significance=0.0,
            expected_effect=0.0,
        )


def test_assessment_requires_evidence_and_rejects_raw_numeric_delta() -> None:
    with pytest.raises(InnerStateValidationError):
        assessment(evidence_refs=())
    with pytest.raises(NumericPolicyError):
        assessment(raw_numeric_delta={"pad.pleasure": 0.3})
    with pytest.raises(NumericPolicyError):
        assessment(raw_numeric_delta={})


def test_goal_and_intention_inputs_use_typed_evidence_and_triggers() -> None:
    goal = GoalEvidence(
        fluctlight_id="fluctlight-1",
        source=GoalSource.HUMAN,
        description="finish the sketch",
        evidence_refs=("human:request-1",),
        importance=0.7,
        urgency=0.4,
    )
    assert goal.source.value == "human"
    trigger = TimeTrigger(at=datetime.now(UTC) + timedelta(hours=1))
    intention = IntentionEvidence(
        fluctlight_id="fluctlight-1",
        goal_id=goal.goal_id,
        action="prepare a draft",
        trigger=trigger,
        confidence=0.8,
        expiration=datetime.now(UTC) + timedelta(hours=2),
        evidence_refs=("human:request-1",),
        preferred_time=trigger.at,
    )
    assert intention.trigger.type.value == "time"
    with pytest.raises(InnerStateValidationError):
        SemanticTrigger(schema_version="semantic.trigger.v1", evidence_refs=())
    with pytest.raises(InnerStateValidationError):
        IntentionEvidence(
            fluctlight_id="fluctlight-1",
            goal_id=goal.goal_id,
            action="prepare a draft",
            trigger="keyword:later",  # type: ignore[arg-type]
            confidence=0.8,
            expiration=datetime.now(UTC) + timedelta(hours=2),
            evidence_refs=("human:request-1",),
        )
    assert EventTrigger(event_type="inbox.fact").type.value == "event"


def test_drive_assessment_requires_its_own_evidence() -> None:
    with pytest.raises(InnerStateValidationError):
        DriveAssessment(
            drive=DriveName.SOCIAL,
            direction=AffectDirection.POSITIVE,
            strength=0.5,
            confidence=0.8,
            evidence_refs=(),
        )
