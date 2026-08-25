from datetime import UTC, datetime

import pytest
from fluctlight_core.conversations.contracts import (
    ConversationTurn,
    MessageDraft,
    MessageKind,
    ParticipantRole,
)


def test_conversation_turn_and_message_draft_are_typed_and_bounded() -> None:
    turn = ConversationTurn(
        conversation_id="conversation-1",
        actor_id="human-1",
        text="hello",
        attachment_refs=("asset:1",),
        idempotency_key="turn-1",
        turn_id="turn-1",
        correlation_id="corr-1",
    )
    draft = MessageDraft("fluctlight-1", "reply", MessageKind.ASSISTANT)
    assert turn.attachment_refs == ("asset:1",)
    assert draft.kind is MessageKind.ASSISTANT
    assert ParticipantRole.OWNER.value == "owner"
    with pytest.raises(ValueError):
        ConversationTurn("conversation-1", "human-1", "")


def test_turn_defaults_are_timezone_safe() -> None:
    turn = ConversationTurn("conversation-1", "human-1", "hello")
    assert turn.turn_id.startswith("turn_")
    assert datetime.now(UTC).tzinfo is not None
