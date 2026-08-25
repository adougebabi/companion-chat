"""Temporal adapter for committed cognitive intents."""

from __future__ import annotations

from datetime import timedelta
from typing import TYPE_CHECKING, Any

from temporalio import activity, workflow

if TYPE_CHECKING:
    from .service import CognitionService

_cognition_service: Any | None = None


def configure_cognition_service(service: CognitionService) -> None:
    global _cognition_service
    _cognition_service = service


@activity.defn(name="process_cognition")
async def process_cognition(payload: dict[str, Any]) -> dict[str, str]:
    if _cognition_service is None:
        raise RuntimeError("cognition activity is not configured")
    result = await _cognition_service.process_next(
        str(payload["fluctlight_id"]), worker_id=str(payload.get("worker_id", "temporal"))
    )
    return {
        "status": result.status.value if result else "pending",
        "event_id": str(payload.get("event_id", "")),
    }


@workflow.defn(name="CognitionProcessingWorkflow")
class CognitionProcessingWorkflow:
    """Keep durable workflow identity separate from cognition domain facts."""

    def __init__(self) -> None:
        self._paused = False
        self._status = "queued"
        self._version = "cognition-v1"

    @workflow.run
    async def run(self, payload: dict[str, Any]) -> dict[str, str]:
        intent_id = str(payload.get("intent_id", "")).strip()
        if not intent_id:
            raise ValueError("cognition workflow requires intent_id")
        self._version = str(payload.get("version", self._version))
        self._status = "running"
        await workflow.wait_condition(lambda: not self._paused)
        if payload.get("fluctlight_id"):
            await workflow.execute_activity(
                process_cognition,
                payload,
                start_to_close_timeout=timedelta(minutes=5),
            )
        self._status = "completed"
        return {"intent_id": intent_id, "status": self._status}

    @workflow.signal
    async def pause(self) -> None:
        self._paused = True
        self._status = "paused"

    @workflow.signal
    async def resume(self) -> None:
        self._paused = False
        self._status = "running"

    @workflow.query
    def status(self) -> dict[str, str | bool]:
        return {"status": self._status, "paused": self._paused, "version": self._version}
