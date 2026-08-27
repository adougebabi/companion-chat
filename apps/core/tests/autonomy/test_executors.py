import asyncio
from typing import Any, cast

import pytest
from fluctlight_core.autonomy.executors import AutonomyExecutor
from fluctlight_core.life_world.contracts import ActionStatus, FrozenAutonomousAction


class EmptyService:
    pass


class MediaPrompt:
    def __init__(self) -> None:
        self.calls: list[dict[str, Any]] = []

    async def generate_media_prompt(self, **kwargs: Any) -> str:
        self.calls.append(kwargs)
        return "final image prompt"


class Media:
    def __init__(self) -> None:
        self.intent = None

    async def request_generation(self, intent: Any) -> None:
        self.intent = intent


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


class Moments:
    def __init__(self) -> None:
        self.created = None

    async def create(self, moment: Any) -> Any:
        self.created = moment
        return moment


def executor(
    conversations: Conversations,
    media: Any = None,
    media_prompt: Any = None,
    moments: Any = None,
) -> AutonomyExecutor:
    return AutonomyExecutor(
        conversations=cast(Any, conversations),
        memory=cast(Any, EmptyService()),
        relationships=cast(Any, EmptyService()),
        life_world=cast(Any, EmptyService()),
        media=cast(Any, media or EmptyService()),
        moments=cast(Any, moments or EmptyService()),
        media_prompt=cast(Any, media_prompt or MediaPrompt()),
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


def test_media_action_uses_media_prompt_and_persists_the_target_conversation() -> None:
    media = Media()
    media_prompt = MediaPrompt()
    result = asyncio.run(
        executor(Conversations(), media=media, media_prompt=media_prompt).execute(
            FrozenAutonomousAction(
                id="action-media",
                fluctlight_id="fluctlight-1",
                action_type="media_request",
                payload={
                    "conversation_id": "conversation-1",
                    "media_request": {"subject": "一张当前状态的自然照片"},
                },
                policy_snapshot={},
                expected_revisions={},
                workflow_id="workflow-media",
                provider_request_id="provider-media",
            )
        )
    )

    assert result.status is ActionStatus.COMPLETED
    assert media_prompt.calls == [
        {
            "media_request": {"subject": "一张当前状态的自然照片"},
            "correlation_id": "media-prompt:action-media",
        }
    ]
    assert media.intent is not None
    assert media.intent.prompt == "final image prompt"
    assert media.intent.conversation_id == "conversation-1"
    assert media.intent.mime_type == "image/png"
    assert media.intent.workflow_id == "media_workflow_action-media"


def test_moment_action_persists_realized_text_with_shared_visibility() -> None:
    moments = Moments()
    media = Media()
    media_prompt = MediaPrompt()
    result = asyncio.run(
        executor(Conversations(), moments=moments, media=media, media_prompt=media_prompt).execute(
            FrozenAutonomousAction(
                id="action-moment",
                fluctlight_id="fluctlight-1",
                action_type="moment",
                payload={
                    "text": "今天整理了几张很喜欢的照片",
                    "moment_media_request": {
                        "subject": "整理照片后的自然桌面一角",
                        "scene": "家中书桌",
                    },
                },
                policy_snapshot={},
                expected_revisions={},
                workflow_id="workflow-moment",
                provider_request_id="provider-moment",
            )
        )
    )

    assert result.status is ActionStatus.COMPLETED
    assert moments.created is not None
    assert moments.created.text == "今天整理了几张很喜欢的照片"
    assert moments.created.visibility.value == "participants"
    assert moments.created.media_asset_ids == ()
    assert media_prompt.calls == [
        {
            "media_request": {
                "subject": "整理照片后的自然桌面一角",
                "scene": "家中书桌",
            },
            "correlation_id": "media-prompt:action-moment",
        }
    ]
    assert media.intent is not None
    assert media.intent.kind.value == "image"
    assert media.intent.moment_id == "moment_action-moment"
    assert media.intent.conversation_id is None
