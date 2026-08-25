from datetime import UTC, datetime, timedelta

import pytest
from fluctlight_core.inner_state.contracts import (
    PAD,
    AffectDirection,
    Appraisal,
    DriveAssessment,
    DriveName,
    DriveState,
    GoalEvidence,
    GoalSource,
    GoalStatus,
    InnerStateSnapshot,
    IntentionEvidence,
    IntentionStatus,
    Momentum,
    SemanticAssessment,
    SemanticPerception,
    TimeTrigger,
)
from fluctlight_core.inner_state.policy import (
    NumericPolicyError,
    NumericStatePolicy,
    govern_intention,
    propose_goal,
    propose_intention,
    qualify_intention,
    transition_goal,
)


def make_assessment(*, idempotency_key: str = "assessment-1") -> SemanticAssessment:
    return SemanticAssessment(
        schema_version="semantic.assessment.v1",
        perception=SemanticPerception(event_kind="message", sentiment="warm"),
        appraisal=Appraisal(
            relevance=0.9,
            goal_congruence=0.8,
            reward=0.9,
            loss=0.1,
            social_threat=0.1,
            controllability=0.7,
            responsibility=0.5,
            relationship_significance=0.7,
            expected_effect=0.8,
        ),
        direction=AffectDirection.POSITIVE,
        strength=0.9,
        confidence=0.9,
        evidence_refs=("event:1",),
        model="test-model",
        model_version="test-v1",
        prompt_version="prompt-v1",
        source_event_id="event-1",
        idempotency_key=idempotency_key,
        drive_assessments=(
            DriveAssessment(
                drive=DriveName.SOCIAL,
                direction=AffectDirection.POSITIVE,
                strength=0.8,
                confidence=0.9,
                evidence_refs=("event:1",),
            ),
        ),
    )


def make_state(*, updated_at: datetime) -> InnerStateSnapshot:
    return InnerStateSnapshot(
        fluctlight_id="fluctlight-1",
        pad=PAD(pleasure=0.8, arousal=0.4, dominance=-0.2),
        momentum=Momentum(pleasure_momentum=0.7, decay_rate=0.5),
        drives=(
            DriveState(
                name=DriveName.SOCIAL,
                level=0.2,
                baseline=0.1,
                growth_rate=0.5,
                satisfaction_rate=0.4,
            ),
        ),
        revision=2,
        last_updated_at=updated_at,
    )


def test_wall_time_decay_is_lazy_and_approaches_zero_or_baseline() -> None:
    before = datetime(2026, 8, 24, tzinfo=UTC)
    after = before + timedelta(days=1)
    state = make_state(updated_at=before)
    decayed = NumericStatePolicy().apply_wall_time_decay(state, now=after)

    assert abs(decayed.pad.pleasure) < abs(state.pad.pleasure)
    assert abs(decayed.momentum.pleasure_momentum) < abs(state.momentum.pleasure_momentum)
    assert decayed.drives[0].level > 0.1
    assert decayed.drives[0].level < state.drives[0].level
    assert decayed.revision == state.revision + 1
    assert NumericStatePolicy().apply_wall_time_decay(decayed, now=after) == decayed


def test_assessment_policy_computes_and_clamps_numeric_changes() -> None:
    now = datetime(2026, 8, 24, 12, tzinfo=UTC)
    state = make_state(updated_at=now)
    transition = NumericStatePolicy().apply_semantic_assessment(
        state,
        make_assessment(),
        expected_revision=state.revision,
        now=now,
    )

    assert transition.result == "accepted"
    assert transition.current.pad.pleasure <= 1.0
    assert transition.current.pad.arousal <= 1.0
    assert transition.current.revision == state.revision + 1
    assert transition.applied_delta["pad.pleasure"] <= transition.requested_delta["pad.pleasure"]
    assert transition.current.drives[0].level > state.drives[0].level
    assert "mood.intensity" in transition.applied_delta
    assert "drive.social" in transition.applied_delta


def test_assessment_with_elapsed_decay_still_advances_one_revision() -> None:
    before = datetime(2026, 8, 23, tzinfo=UTC)
    state = make_state(updated_at=before)
    transition = NumericStatePolicy().apply_semantic_assessment(
        state,
        make_assessment(idempotency_key="assessment-decay"),
        expected_revision=state.revision,
        now=before + timedelta(days=1),
    )
    assert transition.current.revision == state.revision + 1


def test_assessment_rejects_stale_revision_without_applying_state() -> None:
    state = make_state(updated_at=datetime(2026, 8, 24, tzinfo=UTC))
    with pytest.raises(NumericPolicyError):
        NumericStatePolicy().apply_semantic_assessment(
            state,
            make_assessment(),
            expected_revision=state.revision - 1,
            now=state.last_updated_at,
        )


def test_goal_and_intention_lifecycle_requires_qualification_before_execution() -> None:
    now = datetime(2026, 8, 24, tzinfo=UTC)
    goal = propose_goal(
        GoalEvidence(
            fluctlight_id="fluctlight-1",
            source=GoalSource.HUMAN,
            description="finish a draft",
            evidence_refs=("human:1",),
            importance=0.8,
            urgency=0.5,
        )
    )
    active = transition_goal(goal, GoalStatus.ACTIVE, now=now)
    paused = transition_goal(active, GoalStatus.PAUSED, now=now)
    assert paused.status == GoalStatus.PAUSED
    with pytest.raises(NumericPolicyError):
        transition_goal(paused, GoalStatus.COMPLETED, now=now)

    trigger = TimeTrigger(at=now + timedelta(hours=1))
    intention = propose_intention(
        IntentionEvidence(
            fluctlight_id="fluctlight-1",
            goal_id=goal.id,
            action="write",
            trigger=trigger,
            confidence=0.9,
            expiration=now + timedelta(hours=2),
            evidence_refs=("human:1",),
            preferred_time=trigger.at,
        )
    )
    pending = qualify_intention(intention, now=now)
    assert pending.status == IntentionStatus.PENDING
    cancelled = govern_intention(pending, IntentionStatus.CANCELLED, now=now)
    assert cancelled.status == IntentionStatus.CANCELLED
    paused_intention = govern_intention(pending, IntentionStatus.PAUSED, now=now)
    with pytest.raises(NumericPolicyError):
        govern_intention(paused_intention, IntentionStatus.PENDING, now=now + timedelta(hours=3))
