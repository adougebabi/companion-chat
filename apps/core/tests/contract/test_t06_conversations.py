from fluctlight_core.conversations import schema as conversation_schema
from fluctlight_core.platform.persistence import metadata


def test_t06_conversation_tables_share_public_metadata_and_order_keys() -> None:
    expected = {
        "conversations",
        "conversation_participants",
        "conversation_heads",
        "conversation_messages",
        "conversation_read_positions",
        "fluctlight_direct_conversations",
    }
    assert {f"public.{name}" for name in expected} <= set(metadata.tables)
    assert any(
        constraint.name == "conversation_message_sequence"
        for constraint in conversation_schema.messages.constraints
    )
    assert any(
        constraint.name == "conversation_message_idempotency"
        for constraint in conversation_schema.messages.constraints
    )
    assert conversation_schema.direct_conversations.c.owner_actor_id.primary_key
    assert conversation_schema.direct_conversations.c.fluctlight_actor_id.primary_key
