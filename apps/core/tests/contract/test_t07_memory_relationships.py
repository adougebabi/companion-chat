from fluctlight_core.memory import schema as memory_schema
from fluctlight_core.platform.persistence import metadata
from fluctlight_core.relationships import schema as relationship_schema


def test_t07_tables_share_public_metadata_and_revision_keys() -> None:
    expected = {
        "memories",
        "memory_revisions",
        "memory_embeddings",
        "relationships",
        "relationship_revisions",
        "relationship_governance",
    }
    assert {f"public.{name}" for name in expected} <= set(metadata.tables)
    assert any(
        constraint.name == "memory_revision_number"
        for constraint in memory_schema.memory_revisions.constraints
    )
    assert any(
        constraint.name == "relationship_directed_pair"
        for constraint in relationship_schema.relationships.constraints
    )
