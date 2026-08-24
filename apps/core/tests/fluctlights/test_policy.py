from datetime import UTC, datetime

import pytest
from fluctlight_core.fluctlights.contracts import (
    BehavioralPolicy,
    FluctlightSnapshot,
    FluctlightStatus,
    FoundationRevisionRequest,
    Identity,
    InitializationMode,
    Personality,
    PersonalityUpdatePolicy,
    RevisionSource,
)
from fluctlight_core.fluctlights.policy import (
    RevisionConflictError,
    RevisionGovernanceError,
    apply_changes,
    validate_revision,
)


def snapshot(*, revision: int = 0, updated_at: datetime | None = None) -> FluctlightSnapshot:
    moment = updated_at or datetime(2026, 8, 24, tzinfo=UTC)
    return FluctlightSnapshot(
        id="fluctlight-1",
        initialization_mode=InitializationMode.BLANK_SLATE,
        status=FluctlightStatus.ACTIVE,
        identity=Identity(id="fluctlight-1"),
        personality=Personality(
            update_policy=PersonalityUpdatePolicy(
                evidence_window_events=2,
                max_delta=0.1,
                cooldown_seconds=0,
            )
        ),
        behavioral_policy=BehavioralPolicy(),
        current_revision=revision,
        created_at=moment,
        updated_at=moment,
    )


def request(
    *,
    source: RevisionSource,
    changes: dict[str, object],
    evidence: tuple[str, ...] = ("event:1", "event:2"),
    expected_revision: int = 0,
) -> FoundationRevisionRequest:
    return FoundationRevisionRequest(
        fluctlight_id="fluctlight-1",
        actor_id="actor-1",
        source=source,
        changes=changes,
        evidence_refs=evidence,
        expected_revision=expected_revision,
        idempotency_key=f"revision-{expected_revision}-{len(changes)}-{source.value}",
    )


def test_identity_changes_require_human_governance() -> None:
    with pytest.raises(RevisionGovernanceError):
        validate_revision(
            snapshot(),
            request(source=RevisionSource.REFLECTION, changes={"name": "Mira"}),
        )

    validate_revision(
        snapshot(),
        request(source=RevisionSource.HUMAN, changes={"name": "Mira"}),
    )
    updated = apply_changes(
        snapshot(), {"name": "Mira"}, revision=1, now=datetime(2026, 8, 24, tzinfo=UTC)
    )
    assert updated.identity.name == "Mira"


def test_personality_requires_evidence_window_and_max_delta() -> None:
    with pytest.raises(RevisionGovernanceError, match="evidence window"):
        validate_revision(
            snapshot(),
            request(
                source=RevisionSource.REFLECTION,
                changes={"curiosity": 0.55},
                evidence=("event:1",),
            ),
        )
    with pytest.raises(RevisionGovernanceError, match="maximum delta"):
        validate_revision(
            snapshot(),
            request(
                source=RevisionSource.REFLECTION,
                changes={"curiosity": 0.7},
            ),
        )
    validate_revision(
        snapshot(),
        request(source=RevisionSource.REFLECTION, changes={"curiosity": 0.55}),
    )
    with pytest.raises(RevisionGovernanceError):
        validate_revision(
            snapshot(),
            request(source=RevisionSource.LIVED_FACT, changes={"curiosity": 0.55}),
        )
    low_confidence = FoundationRevisionRequest(
        fluctlight_id="fluctlight-1",
        actor_id="actor-1",
        source=RevisionSource.REFLECTION,
        changes={"curiosity": 0.55},
        evidence_refs=("event:1", "event:2"),
        expected_revision=0,
        idempotency_key="low-confidence",
        confidence=0.1,
    )
    with pytest.raises(RevisionGovernanceError):
        validate_revision(snapshot(), low_confidence)


def test_stale_foundation_revision_is_rejected_before_mutation() -> None:
    with pytest.raises(RevisionConflictError):
        validate_revision(
            snapshot(revision=2),
            request(
                source=RevisionSource.HUMAN,
                changes={"name": "Mira"},
                expected_revision=1,
            ),
        )
