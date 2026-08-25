import asyncio
from typing import Any, cast

import pytest
from fluctlight_core.autonomy.executors import AutonomyExecutor
from fluctlight_core.life_world.contracts import ActionStatus, FrozenAutonomousAction


class EmptyService:
    pass


class Conversations:
    def __init__(self) -> None:
        self.calls: list[dict[str, Any]] = []

    async def append_message(self, conversation_id: str, draft: Any, *, actor_id: str) -> None:
        self.calls.append(
            {
                "conversation_id": conversation_id,
                "author": draft.author_actor_id,
                "text": draft.text,
                "idempotency_key": draft.idempotency_key,
                "actor_id": actor_id,
            }
        )


def executor(conversations: Conversations) -> AutonomyExecutor:
    return AutonomyExecutor(
        conversations=cast(Any, conversations),
        memory=cast(Any, EmptyService()),
        relationships=cast(Any, EmptyService()),
        life_world=cast(Any, EmptyService()),
        media=cast(Any, EmptyService()),
        moments=cast(Any, EmptyService()),
    )


def action(payload: dict[str, object]) -> FrozenAutonomousAction:
    return FrozenAutonomousAction(
        id="action-1",
        fluctlight_id="fluctlight-1",
        action_type="proactive_message",
        payload=payload,
        policy_snapshot={},
        expected_revisions={},
        workflow_id="workflow-1",
        provider_request_id="provider-1",
    )


def test_proactive_action_requires_explicit_conversation_and_reuses_action_idempotency() -> None:
    conversations = Conversations()
    result = asyncio.run(
        executor(conversations).execute(
            action({"conversation_id": "conversation-1", "text": "A deliberate message"})
        )
    )

    assert result.status is ActionStatus.COMPLETED
    assert conversations.calls == [
        {
            "conversation_id": "conversation-1",
            "author": "fluctlight-1",
            "text": "A deliberate message",
            "idempotency_key": "autonomy:action-1:message",
            "actor_id": "fluctlight-1",
        }
    ]


def test_proactive_action_without_conversation_is_rejected() -> None:
    with pytest.raises(ValueError, match="conversation_id"):
        asyncio.run(executor(Conversations()).execute(action({"text": "no target"})))
