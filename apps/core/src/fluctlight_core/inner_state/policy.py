"""Server-owned numeric policy and lifecycle transitions."""

from __future__ import annotations

from collections.abc import Iterable
from dataclasses import replace
from datetime import UTC, datetime, timedelta
from math import exp

from .contracts import (
    AffectDirection,
    DriveAssessment,
    DriveState,
    Goal,
    GoalEvidence,
    GoalStatus,
    InnerStateSnapshot,
    InnerStateTransition,
    Intention,
    IntentionEvidence,
    IntentionStatus,
    NumericPolicyError,
    SemanticAssessment,
    _aware,
)


def _clamp(value: float, lower: float, upper: float) -> float:
    return min(upper, max(lower, value))


def _signed_direction(direction: AffectDirection) -> float:
    if direction == AffectDirection.POSITIVE:
        return 1.0
    if direction == AffectDirection.NEGATIVE:
        return -1.0
    if direction == AffectDirection.NEUTRAL:
        return 0.0
    return 0.0


def _toward(value: float, target: float, factor: float) -> float:
    return value + (target - value) * factor


def _decay_factor(rate: float, elapsed_seconds: float) -> float:
    return exp(-rate * max(0.0, elapsed_seconds) / 86_400.0)


class NumericStatePolicy:
    """Apply typed semantic signals without allowing model-owned deltas.

    All formulas operate on structured bounded fields.  No natural-language
    content is inspected here, and an invalid assessment has no fallback
    state transition.
    """

    def __init__(self, *, policy_version: str = "t04-numeric-v1") -> None:
        self.policy_version = policy_version

    def apply_wall_time_decay(
        self, snapshot: InnerStateSnapshot, *, now: datetime | None = None
    ) -> InnerStateSnapshot:
        current_time = now or datetime.now(UTC)
        _aware(current_time, "now")
        elapsed = (current_time - snapshot.last_updated_at).total_seconds()
        if elapsed <= 0:
            return snapshot

        pad_factor = _decay_factor(snapshot.regulation.natural_decay_rate, elapsed)
        momentum_factor = _decay_factor(snapshot.momentum.decay_rate, elapsed)
        mood_factor = _decay_factor(snapshot.regulation.natural_decay_rate, elapsed)
        drive_factor = _decay_factor(snapshot.regulation.natural_decay_rate, elapsed)
        drives = tuple(
            replace(
                drive,
                level=_clamp(
                    _toward(drive.level, drive.baseline, 1.0 - drive_factor),
                    0.0,
                    1.0,
                ),
            )
            for drive in snapshot.drives
        )
        return replace(
            snapshot,
            pad=replace(
                snapshot.pad,
                pleasure=snapshot.pad.pleasure * pad_factor,
                arousal=snapshot.pad.arousal * pad_factor,
                dominance=snapshot.pad.dominance * pad_factor,
            ),
            mood=replace(snapshot.mood, intensity=snapshot.mood.intensity * mood_factor),
            momentum=replace(
                snapshot.momentum,
                pleasure_momentum=snapshot.momentum.pleasure_momentum * momentum_factor,
                arousal_momentum=snapshot.momentum.arousal_momentum * momentum_factor,
                dominance_momentum=snapshot.momentum.dominance_momentum * momentum_factor,
            ),
            drives=drives,
            revision=snapshot.revision + 1,
            last_updated_at=current_time,
        )

    def apply_semantic_assessment(
        self,
        snapshot: InnerStateSnapshot,
        assessment: SemanticAssessment,
        *,
        expected_revision: int | None = None,
        now: datetime | None = None,
    ) -> InnerStateTransition:
        if expected_revision is not None and expected_revision != snapshot.revision:
            raise NumericPolicyError("inner-state revision is stale")
        if not isinstance(assessment, SemanticAssessment):
            raise NumericPolicyError("a typed semantic assessment is required")
        current_time = now or datetime.now(UTC)
        _aware(current_time, "now")
        # Decay and the semantic assessment are one auditable state event.  A
        # lazy decay updates the values and timestamp, while this event owns
        # the single revision increment.
        decayed = self.apply_wall_time_decay(snapshot, now=current_time)
        current = replace(decayed, revision=snapshot.revision)

        direction = _signed_direction(assessment.direction)
        strength = assessment.strength * assessment.confidence
        appraisal = assessment.appraisal
        valence = (
            (appraisal.reward - appraisal.loss) * 0.55
            + (appraisal.goal_congruence - appraisal.social_threat) * 0.25
            + (appraisal.expected_effect - 0.5) * 0.20
        )
        if assessment.direction == AffectDirection.POSITIVE:
            valence = abs(valence)
        elif assessment.direction == AffectDirection.NEGATIVE:
            valence = -abs(valence)
        elif assessment.direction == AffectDirection.NEUTRAL:
            valence = 0.0
        else:
            valence *= 0.5

        pleasure_delta = _clamp(valence * strength * 0.6, -0.35, 0.35)
        arousal_delta = _clamp(
            (appraisal.relevance * 0.55 + appraisal.social_threat * 0.45 - 0.5) * strength * 0.5,
            -0.25,
            0.25,
        )
        dominance_delta = _clamp(
            (appraisal.controllability - 0.5) * direction * strength * 0.5,
            -0.25,
            0.25,
        )
        requested = {
            "pad.pleasure": pleasure_delta,
            "pad.arousal": arousal_delta,
            "pad.dominance": dominance_delta,
            "momentum.pleasure_momentum": pleasure_delta * 0.5,
            "momentum.arousal_momentum": arousal_delta * 0.5,
            "momentum.dominance_momentum": dominance_delta * 0.5,
        }
        pad = replace(
            current.pad,
            pleasure=_clamp(current.pad.pleasure + pleasure_delta, -1.0, 1.0),
            arousal=_clamp(current.pad.arousal + arousal_delta, -1.0, 1.0),
            dominance=_clamp(current.pad.dominance + dominance_delta, -1.0, 1.0),
        )
        momentum = replace(
            current.momentum,
            pleasure_momentum=_clamp(
                current.momentum.pleasure_momentum + pleasure_delta * 0.5, -1.0, 1.0
            ),
            arousal_momentum=_clamp(
                current.momentum.arousal_momentum + arousal_delta * 0.5, -1.0, 1.0
            ),
            dominance_momentum=_clamp(
                current.momentum.dominance_momentum + dominance_delta * 0.5, -1.0, 1.0
            ),
        )
        mood = replace(
            current.mood,
            label=assessment.perception.sentiment,
            intensity=_clamp(max(current.mood.intensity, strength), 0.0, 1.0),
            source="semantic_assessment",
            started_at=current_time,
            expected_decay_at=current_time + timedelta(days=1),
        )
        requested["mood.intensity"] = mood.intensity - current.mood.intensity
        drives = self._apply_drive_assessments(current.drives, assessment.drive_assessments)
        for item in assessment.drive_assessments:
            name = str(item.drive)
            drive = next((drive for drive in current.drives if str(drive.name) == name), None)
            if drive is None:
                raise NumericPolicyError(f"unknown drive assessment: {name}")
            sign = _signed_direction(item.direction)
            requested[f"drive.{name}"] = (
                item.strength * item.confidence * drive.growth_rate * 0.2
                if sign >= 0
                else -item.strength * item.confidence * drive.satisfaction_rate * 0.2
            )
        next_state = replace(
            current,
            pad=pad,
            mood=mood,
            momentum=momentum,
            drives=drives,
            revision=current.revision + 1,
            last_updated_at=current_time,
        )
        applied_values = {
            "pad.pleasure": next_state.pad.pleasure - current.pad.pleasure,
            "pad.arousal": next_state.pad.arousal - current.pad.arousal,
            "pad.dominance": next_state.pad.dominance - current.pad.dominance,
            "momentum.pleasure_momentum": next_state.momentum.pleasure_momentum
            - current.momentum.pleasure_momentum,
            "momentum.arousal_momentum": next_state.momentum.arousal_momentum
            - current.momentum.arousal_momentum,
            "momentum.dominance_momentum": next_state.momentum.dominance_momentum
            - current.momentum.dominance_momentum,
            "mood.intensity": next_state.mood.intensity - current.mood.intensity,
        }
        applied_values.update(
            {
                f"drive.{drive.name}": next_state.drives[index].level - drive.level
                for index, drive in enumerate(current.drives)
            }
        )
        applied = {key: applied_values[key] for key in requested}
        return InnerStateTransition(
            previous=snapshot,
            current=next_state,
            result="accepted",
            reason_code="semantic_assessment_applied",
            policy_version=self.policy_version,
            model_version=assessment.model_version,
            requested_delta=requested,
            applied_delta=applied,
            idempotency_key=assessment.idempotency_key,
            source_event_id=assessment.source_event_id,
            evidence_refs=assessment.evidence_refs,
        )

    @staticmethod
    def _apply_drive_assessments(
        drives: tuple[DriveState, ...], assessments: Iterable[DriveAssessment]
    ) -> tuple[DriveState, ...]:
        by_name = {str(drive.name): drive for drive in drives}
        seen: set[str] = set()
        for assessment in assessments:
            name = str(assessment.drive)
            if name in seen:
                raise NumericPolicyError(f"duplicate drive assessment: {name}")
            seen.add(name)
            drive = by_name.get(name)
            if drive is None:
                raise NumericPolicyError(f"unknown drive assessment: {name}")
            sign = _signed_direction(assessment.direction)
            if sign >= 0:
                delta = assessment.strength * assessment.confidence * drive.growth_rate * 0.2
            else:
                delta = -assessment.strength * assessment.confidence * drive.satisfaction_rate * 0.2
            by_name[name] = replace(drive, level=_clamp(drive.level + delta, 0.0, 1.0))
        return tuple(by_name.values())


def propose_goal(evidence: GoalEvidence) -> Goal:
    return Goal(
        id=evidence.goal_id,
        fluctlight_id=evidence.fluctlight_id,
        source=evidence.source,
        description=evidence.description,
        importance=evidence.importance,
        urgency=evidence.urgency,
        progress=0.0,
        status=GoalStatus.CANDIDATE,
        evidence_refs=evidence.evidence_refs,
        deadline=evidence.deadline,
    )


_GOAL_TRANSITIONS: dict[GoalStatus, frozenset[GoalStatus]] = {
    GoalStatus.CANDIDATE: frozenset(
        {GoalStatus.ACTIVE, GoalStatus.ABANDONED, GoalStatus.CANCELLED}
    ),
    GoalStatus.ACTIVE: frozenset(
        {GoalStatus.PAUSED, GoalStatus.COMPLETED, GoalStatus.ABANDONED, GoalStatus.CANCELLED}
    ),
    GoalStatus.PAUSED: frozenset({GoalStatus.ACTIVE, GoalStatus.ABANDONED, GoalStatus.CANCELLED}),
    GoalStatus.COMPLETED: frozenset(),
    GoalStatus.ABANDONED: frozenset(),
    GoalStatus.CANCELLED: frozenset(),
}


def transition_goal(goal: Goal, target: GoalStatus, *, now: datetime | None = None) -> Goal:
    target = GoalStatus(target)
    if target not in _GOAL_TRANSITIONS[goal.status]:
        raise NumericPolicyError(f"invalid goal transition {goal.status.value}->{target.value}")
    current_time = now or datetime.now(UTC)
    _aware(current_time, "now")
    return replace(goal, status=target, revision=goal.revision + 1, updated_at=current_time)


def propose_intention(evidence: IntentionEvidence) -> Intention:
    return Intention(
        id=evidence.intention_id,
        fluctlight_id=evidence.fluctlight_id,
        goal_id=evidence.goal_id,
        action=evidence.action,
        preferred_time=evidence.preferred_time,
        trigger=evidence.trigger,
        confidence=evidence.confidence,
        expiration=evidence.expiration,
        evidence_refs=evidence.evidence_refs,
        permission_snapshot=evidence.permission_snapshot,
        budget_snapshot=evidence.budget_snapshot,
        status=IntentionStatus.CANDIDATE,
    )


def qualify_intention(intention: Intention, *, now: datetime | None = None) -> Intention:
    if intention.status != IntentionStatus.CANDIDATE:
        raise NumericPolicyError("only candidate intentions can be qualified")
    current_time = now or datetime.now(UTC)
    _aware(current_time, "now")
    if intention.expiration <= current_time:
        raise NumericPolicyError("expired intention cannot be qualified")
    return replace(
        intention,
        status=IntentionStatus.PENDING,
        revision=intention.revision + 1,
        updated_at=current_time,
    )


def govern_intention(
    intention: Intention,
    target: IntentionStatus,
    *,
    now: datetime | None = None,
) -> Intention:
    target = IntentionStatus(target)
    current_time = now or datetime.now(UTC)
    _aware(current_time, "now")
    if target == IntentionStatus.EXPIRED:
        if current_time < intention.expiration:
            raise NumericPolicyError("intention cannot expire before its expiration")
        if intention.status not in {IntentionStatus.PENDING, IntentionStatus.PAUSED}:
            raise NumericPolicyError("only pending or paused intentions can expire")
    elif target == IntentionStatus.PAUSED:
        if intention.status != IntentionStatus.PENDING:
            raise NumericPolicyError("only pending intentions can be paused")
    elif target == IntentionStatus.PENDING:
        if intention.status != IntentionStatus.PAUSED:
            raise NumericPolicyError("only paused intentions can resume")
        if current_time >= intention.expiration:
            raise NumericPolicyError("expired intention cannot resume")
    elif target in {IntentionStatus.COMPLETED, IntentionStatus.CANCELLED}:
        if intention.status not in {IntentionStatus.PENDING, IntentionStatus.PAUSED}:
            raise NumericPolicyError("only pending or paused intentions can be closed")
    else:
        raise NumericPolicyError("candidate intentions must be qualified before governance")
    return replace(
        intention, status=target, revision=intention.revision + 1, updated_at=current_time
    )


def validate_semantic_assessment(assessment: SemanticAssessment) -> SemanticAssessment:
    if not isinstance(assessment, SemanticAssessment):
        raise NumericPolicyError("a typed semantic assessment is required")
    return assessment
