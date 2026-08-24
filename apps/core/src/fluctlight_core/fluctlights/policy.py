"""Deterministic governance policy for foundation revisions."""

from __future__ import annotations

from collections.abc import Collection, Mapping
from dataclasses import replace
from datetime import UTC, datetime
from typing import Any

from .contracts import (
    BEHAVIORAL_POLICY_FIELDS,
    IDENTITY_FIELDS,
    PERSONALITY_FIELDS,
    BehavioralPolicy,
    FluctlightSnapshot,
    FoundationRevisionRequest,
    MutabilityClass,
    Personality,
    RevisionSource,
)


class RevisionConflictError(RuntimeError):
    """Raised when a revision is based on a stale foundation revision."""


class RevisionGovernanceError(ValueError):
    """Raised when a change is not allowed by the server-owned policy."""


def classify_field(field_name: str) -> MutabilityClass:
    if field_name == "id":
        return MutabilityClass.IMMUTABLE
    if field_name in PERSONALITY_FIELDS:
        return MutabilityClass.LIVED
    if field_name in IDENTITY_FIELDS or field_name in BEHAVIORAL_POLICY_FIELDS:
        return MutabilityClass.HUMAN_GOVERNED
    raise RevisionGovernanceError(f"unknown foundation field: {field_name}")


def _apply_identity(identity, changes: Mapping[str, Any]):
    values = identity.as_payload()
    for key, value in changes.items():
        if key == "core_values" and isinstance(value, list):
            value = tuple(value)
        values[key] = value
    values["id"] = identity.id
    return type(identity)(**values)


def _apply_personality(personality: Personality, changes: Mapping[str, Any]) -> Personality:
    values = personality.as_payload()
    values.update(changes)
    policy = values.pop("update_policy")
    values["update_policy"] = (
        personality.update_policy
        if not isinstance(policy, dict)
        else type(personality.update_policy)(**policy)
    )
    return Personality(**values)


def _apply_behavior(policy: BehavioralPolicy, changes: Mapping[str, Any]) -> BehavioralPolicy:
    values = policy.as_payload()
    values.update(changes)
    return BehavioralPolicy(**values)


def apply_changes(
    snapshot: FluctlightSnapshot, changes: Mapping[str, Any], *, revision: int, now: datetime
) -> FluctlightSnapshot:
    identity_changes: dict[str, Any] = {}
    personality_changes: dict[str, Any] = {}
    behavior_changes: dict[str, Any] = {}
    for key, value in changes.items():
        if key in IDENTITY_FIELDS:
            identity_changes[key] = value
        elif key in PERSONALITY_FIELDS:
            personality_changes[key] = value
        elif key in BEHAVIORAL_POLICY_FIELDS:
            behavior_changes[key] = value
        else:
            raise RevisionGovernanceError(f"unknown foundation field: {key}")
    return replace(
        snapshot,
        identity=_apply_identity(snapshot.identity, identity_changes)
        if identity_changes
        else snapshot.identity,
        personality=_apply_personality(snapshot.personality, personality_changes)
        if personality_changes
        else snapshot.personality,
        behavioral_policy=_apply_behavior(snapshot.behavioral_policy, behavior_changes)
        if behavior_changes
        else snapshot.behavioral_policy,
        current_revision=revision,
        updated_at=now,
    )


def validate_revision(
    snapshot: FluctlightSnapshot,
    request: FoundationRevisionRequest,
    *,
    now: datetime | None = None,
    evidence_event_count: int | None = None,
    last_personality_revision_at: datetime | None = None,
    authorized_evidence_refs: Collection[str] | None = None,
) -> None:
    """Validate ownership, evidence, bounds, and optimistic concurrency.

    This function never interprets natural language. It only validates typed
    field names, actor/source ownership, evidence references, and numeric
    policy limits.
    """

    if request.fluctlight_id != snapshot.id:
        raise RevisionGovernanceError("revision targets a different Fluctlight")
    if request.expected_revision != snapshot.current_revision:
        raise RevisionConflictError("foundation revision is stale")
    if not request.changes:
        raise RevisionGovernanceError("revision must contain at least one change")
    if request.source == RevisionSource.REFLECTION and not request.evidence_refs:
        raise RevisionGovernanceError("reflection revision requires evidence references")
    if request.source == RevisionSource.LIVED_FACT and not request.evidence_refs:
        raise RevisionGovernanceError("lived-field revision requires evidence references")
    if authorized_evidence_refs is not None and not set(request.evidence_refs) <= set(
        authorized_evidence_refs
    ):
        raise RevisionGovernanceError("revision contains foreign evidence references")

    personality_changes = []
    for key, value in request.changes.items():
        mutability = classify_field(key)
        if mutability == MutabilityClass.IMMUTABLE:
            raise RevisionGovernanceError(f"immutable field cannot be changed: {key}")
        if mutability == MutabilityClass.HUMAN_GOVERNED and request.source not in {
            RevisionSource.HUMAN,
            RevisionSource.INITIALIZATION,
            RevisionSource.ROLLBACK,
        }:
            raise RevisionGovernanceError(f"{key} requires human governance")
        if mutability == MutabilityClass.LIVED and request.source not in {
            RevisionSource.REFLECTION,
            RevisionSource.INITIALIZATION,
            RevisionSource.ROLLBACK,
        }:
            raise RevisionGovernanceError(f"{key} requires lived evidence or reflection")
        if mutability == MutabilityClass.LIVED:
            personality_changes.append((key, value))

    if personality_changes:
        policy = snapshot.personality.update_policy
        if (
            request.source == RevisionSource.REFLECTION
            and request.confidence < policy.minimum_confidence
        ):
            raise RevisionGovernanceError("personality reflection confidence is too low")
        count = (
            evidence_event_count if evidence_event_count is not None else len(request.evidence_refs)
        )
        if request.source == RevisionSource.REFLECTION and count < policy.evidence_window_events:
            raise RevisionGovernanceError("personality evidence window is not satisfied")
        if request.source == RevisionSource.REFLECTION and last_personality_revision_at is not None:
            current = now or datetime.now(UTC)
            elapsed = (current - last_personality_revision_at).total_seconds()
            if elapsed < policy.cooldown_seconds:
                raise RevisionGovernanceError("personality revision cooldown is active")
        for key, value in personality_changes:
            if not isinstance(value, int | float) or isinstance(value, bool):
                raise RevisionGovernanceError(f"{key} must be numeric")
            old = float(getattr(snapshot.personality, key))
            if abs(float(value) - old) > policy.max_delta:
                raise RevisionGovernanceError(f"{key} exceeds the configured maximum delta")

    apply_changes(
        snapshot,
        request.changes,
        revision=snapshot.current_revision + 1,
        now=now or datetime.now(UTC),
    )
