import pytest
from fluctlight_core.memory.contracts import (
    EmbeddingResult,
    MemoryQuery,
    MemoryRecord,
    MemoryType,
    MemoryVisibility,
    cosine_similarity,
)


def memory() -> MemoryRecord:
    return MemoryRecord(
        id="memory-1",
        owner_fluctlight_id="fl-1",
        type=MemoryType.EPISODIC,
        content="A bounded event",
        actor_refs=("human-1",),
        conversation_id="conversation-1",
        event_refs=("event-1",),
        evidence_refs=("event-1",),
        confidence=0.9,
        importance=0.7,
        emotional_significance=0.4,
        visibility=MemoryVisibility.PRIVATE,
    )


def test_memory_requires_evidence_and_query_scope() -> None:
    assert memory().revision == 0
    query = MemoryQuery("fl-1", ("human-1",), query_text="event")
    assert query.owner_fluctlight_id == "fl-1"
    with pytest.raises(ValueError):
        MemoryRecord(
            id="memory-1",
            owner_fluctlight_id="fl-1",
            type=MemoryType.SEMANTIC,
            content="missing evidence",
            actor_refs=(),
            conversation_id=None,
            event_refs=(),
            evidence_refs=(),
            confidence=0.5,
            importance=0.5,
            emotional_significance=0.0,
            visibility=MemoryVisibility.PRIVATE,
        )
    with pytest.raises(ValueError, match="authorized_actor_ids"):
        MemoryQuery("fl-1", ())


def test_embedding_model_and_exact_vector_are_typed() -> None:
    result = EmbeddingResult("memory-1", 0, "model-v1", (1.0, 0.0))
    assert result.vector == (1.0, 0.0)
    assert cosine_similarity((1.0, 0.0), (1.0, 0.0)) == 1.0
