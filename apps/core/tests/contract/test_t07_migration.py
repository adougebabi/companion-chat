from fluctlight_core.memory import schema as memory_schema
from fluctlight_core.transport.api import EXPECTED_REVISION


def test_t07_advances_the_linear_readiness_revision() -> None:
    assert EXPECTED_REVISION in {
        "0006_t07_memory_relationships",
        "0007_t08_life_world_autonomy",
        "0008_t09_moments_media",
        "0009_t12_vector_column",
        "0010_t12_event_failures",
        "0011_t12_consumer_heads",
        "0012_t12_consumer_effects",
        "0013_direct_conversation",
        "0014_foundation_reason",
        "0015_actor_groups",
        "0016_media_intent_conversation",
        "0017_media_intent_moment",
        "0018_foundation_v2_life_profile",
        "0020_media_provider_job",
    }


def test_t07_declares_pgvector_and_fts_authority_columns() -> None:
    assert memory_schema.memory_embeddings.c.embedding_vector.type.get_col_spec() == "vector"
    assert any(
        index.name == "ix_memories_search_document" for index in memory_schema.memories.indexes
    )
