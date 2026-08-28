"""Durable Temporal adapter for committed media intents."""

from __future__ import annotations

import asyncio
from datetime import timedelta
from typing import Any, cast

from temporalio import activity, workflow

# These imports are only used by the Activity. Passing them through keeps HTTP
# and persistence dependencies out of Temporal's deterministic workflow reload.
with workflow.unsafe.imports_passed_through():
    from fluctlight_core.conversations.contracts import MessageDraft, MessageKind
    from fluctlight_core.conversations.service import ConversationService
    from fluctlight_core.diagnostics.contracts import DiagnosticEvent, DiagnosticSeverity
    from fluctlight_core.diagnostics.service import DiagnosticsService
    from fluctlight_core.moments.service import MomentsService

    from .contracts import MediaReference
    from .providers import DEFAULT_MEDIA_PROVIDERS, DownloadableMediaProvider
    from .service import MediaService, MediaWorkflowAdapter

_media_service: MediaService | None = None
_settings_service: Any | None = None
_conversation_service: ConversationService | None = None
_diagnostics: DiagnosticsService | None = None
_moments_service: MomentsService | None = None


def configure_media_service(
    media_service: MediaService,
    settings_service: Any,
    conversation_service: ConversationService,
    diagnostics: DiagnosticsService,
    moments_service: MomentsService,
) -> None:
    global _conversation_service, _diagnostics, _media_service, _moments_service, _settings_service
    _media_service = media_service
    _settings_service = settings_service
    _conversation_service = conversation_service
    _diagnostics = diagnostics
    _moments_service = moments_service


@activity.defn(name="process_media_generation")
async def process_media_generation(payload: dict[str, Any]) -> dict[str, str]:
    if _conversation_service is None or _media_service is None or _settings_service is None:
        raise RuntimeError("media generation activity is not configured")
    intent_id = str(payload["intent_id"])
    intent = await _media_service.get_intent(intent_id)
    asset = await _media_service.get_asset(f"asset_{intent.id}")
    if asset is not None and asset.status.value == "ready":
        await _publish_asset(intent, asset)
        return {"intent_id": intent.id, "asset_id": asset.id, "status": "ready"}
    if intent.status.value in {"failed", "cancelled"}:
        return {"intent_id": intent.id, "status": intent.status.value}
    claimed_running = await _media_service.mark_intent_running(intent.id)
    if not claimed_running:
        refreshed = await _media_service.get_intent(intent.id)
        if refreshed.provider_job_id is None:
            return {"intent_id": intent.id, "status": refreshed.status.value}
        intent = refreshed
    provider: DownloadableMediaProvider | None = None
    provider_job_id = intent.provider_job_id
    try:
        config = await _settings_service.runtime_value("media.comfyui")
        provider = cast(
            DownloadableMediaProvider, DEFAULT_MEDIA_PROVIDERS.from_config("comfyui", config)
        )
        if provider_job_id is None:
            submitted_job_id = await provider.submit(intent)
            provider_job_id = await _media_service.record_provider_submission(
                intent.id, submitted_job_id
            )
        result = await MediaWorkflowAdapter().run(
            intent,
            provider,
            provider_request_id=provider_job_id,
            heartbeat=lambda request_id: activity.heartbeat({"provider_request_id": request_id}),
            max_polls=900,
            poll_interval_seconds=1.0,
        )
        if not result.completed:
            status = str(result.output.get("status", "failed"))
            if status in {"deferred", "pending"}:
                await _media_service.settle_provider_failure(intent.id)
                return {"intent_id": intent.id, "status": "failed"}
            if status == "cancelled":
                await _media_service.settle_provider_failure(intent.id, cancelled=True)
                return {"intent_id": intent.id, "status": "cancelled"}
            await _media_service.settle_provider_failure(intent.id)
            return {"intent_id": intent.id, "status": "failed"}
        output = result.output.get("output")
        if not isinstance(output, dict):
            raise RuntimeError("media provider completed without an output descriptor")
        downloaded = await provider.download(output)
        asset = await _media_service.settle_provider_output(
            intent, content=downloaded.content, content_type=downloaded.content_type
        )
        await _publish_asset(intent, asset)
        return {"intent_id": intent.id, "asset_id": asset.id, "status": "ready"}
    except asyncio.CancelledError:
        if provider is not None and provider_job_id is not None:
            await provider.cancel(provider_job_id)
        await _media_service.settle_provider_failure(intent.id, cancelled=True)
        raise
    except Exception as exc:
        await _media_service.settle_provider_failure(intent.id)
        if _diagnostics is not None:
            await _diagnostics.emit_event(
                DiagnosticEvent(
                    event_type="media.comfyui.failed",
                    severity=DiagnosticSeverity.ERROR,
                    fluctlight_id=intent.owner_fluctlight_id,
                    correlation_id=intent.workflow_id,
                    payload={
                        "intent_id": intent.id,
                        "error_code": f"media_provider_{type(exc).__name__}".lower(),
                        **(
                            {
                                "http_status": exc.status_code,
                                "provider_detail": exc.detail,
                            }
                            if hasattr(exc, "status_code") and hasattr(exc, "detail")
                            else {}
                        ),
                    },
                )
            )
        raise


async def _publish_asset(intent: Any, asset: Any) -> None:
    if _conversation_service is None or _media_service is None:
        raise RuntimeError("media publication services are not configured")
    if intent.conversation_id is not None:
        await _conversation_service.append_message(
            intent.conversation_id,
            MessageDraft(
                author_actor_id=intent.owner_fluctlight_id,
                text="图片已生成。",
                kind=MessageKind.MEDIA_REFERENCE,
                attachment_refs=(asset.id,),
                idempotency_key=f"media:{intent.id}:conversation-result",
            ),
            actor_id=intent.owner_fluctlight_id,
        )
    if intent.moment_id is not None:
        if _moments_service is None:
            raise RuntimeError("Moments service is not configured")
        await _media_service.attach(
            MediaReference(
                id=f"media_reference_{intent.id}",
                asset_id=asset.id,
                owner_fluctlight_id=intent.owner_fluctlight_id,
                target_type="moment",
                target_id=intent.moment_id,
            ),
            actor_id=intent.owner_fluctlight_id,
        )
        await _moments_service.attach_media_asset(intent.moment_id, asset.id)


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
