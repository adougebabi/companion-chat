import pytest
from fluctlight_core.relationships.contracts import (
    RelationshipTrend,
    RelationshipUpdate,
)


def test_directed_relationship_requires_evidence_and_bounded_metrics() -> None:
    update = RelationshipUpdate(
        owner_fluctlight_id="fl-1",
        target_actor_id="human-1",
        metrics={"trust": 0.8, "affection": 0.4},
        evidence_refs=("event-1",),
        actor_id="human-1",
        expected_revision=0,
        trend=RelationshipTrend.IMPROVING,
    )
    assert update.metrics["trust"] == 0.8
    with pytest.raises(ValueError):
        RelationshipUpdate("fl-1", "human-1", {"trust": 1.5}, ("event-1",), "human-1", 0)
