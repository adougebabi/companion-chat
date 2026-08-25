"""Durable Temporal adapter for committed media intents."""

from __future__ import annotations

import asyncio
from datetime import timedelta
from typing import Any, cast

from temporalio import activity, workflow

# These imports are only used by the Activity. Passing them through keeps HTTP
# and persistence dependencies out of Temporal's deterministic workflow reload.
with workflow.unsafe.imports_passed_through():
    from .providers import DEFAULT_MEDIA_PROVIDERS, DownloadableMediaProvider
    from .service import MediaService, MediaWorkflowAdapter

_media_service: MediaService | None = None
_settings_service: Any | None = None


def configure_media_service(media_service: MediaService, settings_service: Any) -> None:
    global _media_service, _settings_service
    _media_service = media_service
    _settings_service = settings_service


@activity.defn(name="process_media_generation")
async def process_media_generation(payload: dict[str, Any]) -> dict[str, str]:
    if _media_service is None or _settings_service is None:
        raise RuntimeError("media generation activity is not configured")
    intent_id = str(payload["intent_id"])
    intent = await _media_service.get_intent(intent_id)
    await _media_service.mark_intent_running(intent.id)
    config = await _settings_service.runtime_value("media.comfyui")
    provider = cast(
        DownloadableMediaProvider, DEFAULT_MEDIA_PROVIDERS.from_config("comfyui", config)
    )
    try:
        result = await MediaWorkflowAdapter().run(
            intent,
            provider,
            heartbeat=lambda request_id: activity.heartbeat({"provider_request_id": request_id}),
            max_polls=900,
            poll_interval_seconds=1.0,
        )
        if not result.completed:
            await _media_service.settle_provider_failure(intent.id)
            return {"intent_id": intent.id, "status": "failed"}
        output = result.output.get("output")
        if not isinstance(output, dict):
            raise RuntimeError("media provider completed without an output descriptor")
        downloaded = await provider.download(output)
        asset = await _media_service.settle_provider_output(
            intent, content=downloaded.content, content_type=downloaded.content_type
        )
        return {"intent_id": intent.id, "asset_id": asset.id, "status": "ready"}
    except asyncio.CancelledError:
        await provider.cancel(intent.provider_request_id)
        await _media_service.settle_provider_failure(intent.id, cancelled=True)
        raise
    except Exception:
        await _media_service.settle_provider_failure(intent.id)
        raise


@workflow.defn(name="MediaGenerationWorkflow")
class MediaGenerationWorkflow:
    @workflow.run
    async def run(self, payload: dict[str, Any]) -> dict[str, str]:
        intent_id = str(payload.get("intent_id", "")).strip()
        if not intent_id:
            raise ValueError("media workflow requires intent_id")
        return await workflow.execute_activity(
            process_media_generation,
            payload,
            start_to_close_timeout=timedelta(minutes=15),
            heartbeat_timeout=timedelta(seconds=30),
        )
