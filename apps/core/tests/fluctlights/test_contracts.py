from datetime import UTC, datetime
from math import nan

import pytest
from fluctlight_core.fluctlights.contracts import (
    BehavioralPolicy,
    CreateFluctlight,
    FoundationRevisionRequest,
    FoundationValidationError,
    Identity,
    Personality,
    PersonalityUpdatePolicy,
    RevisionSource,
)


def test_create_fluctlight_keeps_identity_id_stable_for_blank_slate() -> None:
    command = CreateFluctlight(actor_id="human-owner", id="fluctlight-1")

    assert command.identity is not None
    assert command.identity.id == command.id
    assert command.initialization_mode.value == "blank_slate"


def test_identity_rejects_an_identity_from_a_different_fluctlight() -> None:
    with pytest.raises(FoundationValidationError):
        CreateFluctlight(
            actor_id="human-owner",
            id="fluctlight-1",
            identity=Identity(id="fluctlight-2"),
        )


def test_foundation_request_copies_changes_and_requires_unique_evidence() -> None:
    request = FoundationRevisionRequest(
        fluctlight_id="fluctlight-1",
        actor_id="human-owner",
        source=RevisionSource.HUMAN,
        changes={"name": "Mira"},
        evidence_refs=("owner:form",),
        expected_revision=0,
        idempotency_key="request-1",
        requested_at=datetime(2026, 8, 24, tzinfo=UTC),
    )
    assert request.changes == {"name": "Mira"}
    with pytest.raises(TypeError):
        request.changes["name"] = "changed"  # type: ignore[index]

    with pytest.raises(FoundationValidationError):
        FoundationRevisionRequest(
            fluctlight_id="fluctlight-1",
            actor_id="human-owner",
            source=RevisionSource.HUMAN,
            changes={"name": "Mira"},
            evidence_refs=("same", "same"),
            expected_revision=0,
            idempotency_key="request-2",
        )


def test_foundation_revision_reason_is_bounded_and_retained() -> None:
    request = FoundationRevisionRequest(
        fluctlight_id="fluctlight-1",
        actor_id="human-owner",
        source=RevisionSource.HUMAN,
        changes={"name": "Mira"},
        evidence_refs=("owner-governance:1",),
        expected_revision=0,
        idempotency_key="request-with-reason",
        reason="Correct the chosen display name",
    )
    assert request.reason == "Correct the chosen display name"
    with pytest.raises(FoundationValidationError):
        FoundationRevisionRequest(
            fluctlight_id="fluctlight-1",
            actor_id="human-owner",
            source=RevisionSource.HUMAN,
            changes={"name": "Mira"},
            evidence_refs=("owner-governance:2",),
            expected_revision=0,
            idempotency_key="request-with-long-reason",
            reason="x" * 1025,
        )


def test_personality_update_policy_has_explicit_slow_change_defaults() -> None:
    personality = Personality(
        update_policy=PersonalityUpdatePolicy(
            evidence_window_events=4,
            max_delta=0.02,
            cooldown_seconds=3600,
        )
    )
    policy = personality.update_policy
    assert policy.evidence_window_events == 4
    assert policy.max_delta == 0.02
    assert policy.cooldown_seconds == 3600
    assert BehavioralPolicy().directness == 0.5


def test_numeric_foundation_values_reject_non_finite_numbers() -> None:
    with pytest.raises(FoundationValidationError):
        Personality(openness=nan)
