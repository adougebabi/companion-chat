"""Temporal trigger for the watermark-governed reflection lifecycle."""

from __future__ import annotations

import logging
from datetime import timedelta
from typing import Any

from temporalio import activity, workflow

_cognition_service: Any | None = None
logger = logging.getLogger(__name__)


def configure_reflection_service(service: Any) -> None:
    global _cognition_service
    _cognition_service = service


@activity.defn(name="run_reflection")
async def run_reflection(payload: dict[str, Any]) -> dict[str, str]:
    if _cognition_service is None:
        raise RuntimeError("reflection activity is not configured")
    fluctlight_id = str(payload["fluctlight_id"])
    logger.warning("reflection.activity.start fluctlight_id=%s", fluctlight_id)
    proposal = await _cognition_service.run_current_reflection(
        fluctlight_id,
        correlation_id=str(payload["correlation_id"]),
    )
    logger.warning(
        "reflection.activity.finished fluctlight_id=%s status=%s",
        fluctlight_id,
        "completed" if proposal is not None else "no_op",
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
