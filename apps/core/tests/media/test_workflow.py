import asyncio

from fluctlight_core.media.contracts import MediaIntent, MediaKind
from fluctlight_core.media.service import MediaWorkflowAdapter


class Provider:
    def __init__(self) -> None:
        self.cancelled = False

    async def submit(self, intent: MediaIntent) -> str:
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
    result = asyncio.run(
        MediaWorkflowAdapter().run(
            intent, Provider(), heartbeat=lambda request_id: beats.append(request_id)
        )
    )
    assert result.completed is True
    assert beats == ["provider-1"]
