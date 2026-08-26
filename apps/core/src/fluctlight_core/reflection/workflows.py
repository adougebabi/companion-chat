"""Temporal trigger for the watermark-governed reflection lifecycle."""

from __future__ import annotations

from datetime import timedelta
from typing import Any

from temporalio import activity, workflow

_cognition_service: Any | None = None


def configure_reflection_service(service: Any) -> None:
    global _cognition_service
    _cognition_service = service


@activity.defn(name="run_reflection")
async def run_reflection(payload: dict[str, Any]) -> dict[str, str]:
    if _cognition_service is None:
        raise RuntimeError("reflection activity is not configured")
    proposal = await _cognition_service.run_current_reflection(
        str(payload["fluctlight_id"]),
        correlation_id=str(payload["correlation_id"]),
    )
    return {"status": "completed" if proposal is not None else "no_op"}


@workflow.defn(name="ReflectionWorkflow")
class ReflectionWorkflow:
    """A durable debounce; state/window CAS remains owned by Cognition."""

    @workflow.run
    async def run(self, payload: dict[str, Any]) -> dict[str, str]:
        if not str(payload.get("intent_id", "")).strip():
            raise ValueError("reflection workflow requires intent_id")
        await workflow.sleep(timedelta(minutes=5))
        return await workflow.execute_activity(
            run_reflection,
            payload,
            start_to_close_timeout=timedelta(minutes=5),
        )
