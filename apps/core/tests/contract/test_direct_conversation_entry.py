from fluctlight_core.conversations import schema as conversation_schema
from fluctlight_core.transport.conversations import (
    ConversationCreateRequest,
    ConversationTurnRequest,
)
from pydantic import ValidationError


def test_direct_conversation_mapping_has_one_authoritative_owner_fluctlight_pair() -> None:
    mapping = conversation_schema.direct_conversations
    assert mapping.c.owner_actor_id.primary_key
    assert mapping.c.fluctlight_actor_id.primary_key
    assert mapping.c.conversation_id.unique


def test_browser_entry_requests_cannot_omit_the_selected_fluctlight() -> None:
    try:
        ConversationCreateRequest()
    except ValidationError:
        pass
    else:
        raise AssertionError("an empty conversation request must be rejected")

    try:
        ConversationTurnRequest(text="hello", idempotency_key="turn-1")
    except ValidationError:
        pass
    else:
        raise AssertionError("a turn without a Fluctlight target must be rejected")
