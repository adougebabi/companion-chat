import asyncio

from fluctlight_core.media.contracts import MediaIntent, MediaKind
from fluctlight_core.media.service import MediaWorkflowAdapter


class Provider:
    def __init__(self) -> None:
        self.cancelled = False
        self.submit_count = 0

    async def submit(self, intent: MediaIntent) -> str:
        self.submit_count += 1
        return intent.provider_request_id

    async def poll(self, provider_request_id: str):
        return {"status": "completed", "provider_request_id": provider_request_id}

    async def cancel(self, provider_request_id: str) -> None:
        self.cancelled = True


def test_media_workflow_reuses_stable_provider_id_and_heartbeats() -> None:
    intent = MediaIntent(
        "intent-1", "fl-1", MediaKind.IMAGE, "image/png", "prompt", "provider-1", "workflow-1"
    )
    beats: list[str] = []
    provider = Provider()
    result = asyncio.run(
        MediaWorkflowAdapter().run(
            intent, provider, heartbeat=lambda request_id: beats.append(request_id)
        )
    )
    assert result.completed is True
    assert beats == ["provider-1"]


def test_media_workflow_retry_polls_persisted_provider_job_without_resubmitting() -> None:
    intent = MediaIntent(
        "intent-2", "fl-1", MediaKind.IMAGE, "image/png", "prompt", "provider-2", "workflow-2"
    )
    provider = Provider()
    first = asyncio.run(MediaWorkflowAdapter().run(intent, provider))
    second = asyncio.run(
        MediaWorkflowAdapter().run(
            intent,
            provider,
            provider_request_id=first.provider_request_id,
        )
    )

    assert first.completed is True
    assert second.completed is True
    assert provider.submit_count == 1
