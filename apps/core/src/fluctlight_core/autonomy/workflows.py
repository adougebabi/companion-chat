"""Durable Temporal adapter for frozen autonomy actions."""

from __future__ import annotations

import logging
from datetime import timedelta
from typing import Any

from temporalio import activity, workflow

_autonomy_service: Any | None = None
_autonomy_executor: Any | None = None
logger = logging.getLogger(__name__)


def configure_autonomy_service(service: Any, executor: Any) -> None:
    global _autonomy_service, _autonomy_executor
    _autonomy_service = service
    _autonomy_executor = executor


@activity.defn(name="process_autonomy_action")
async def process_autonomy_action(payload: dict[str, Any]) -> dict[str, str]:
    if _autonomy_service is None or _autonomy_executor is None:
        raise RuntimeError("autonomy action activity is not configured")
    action_id = str(payload["action_id"])
    logger.warning("autonomy.activity.start action_id=%s", action_id)
    action = await _autonomy_service.execute(action_id, _autonomy_executor)
    logger.warning(
        "autonomy.activity.finished action_id=%s action_type=%s status=%s",
        action_id,
        action.action_type,
        action.status.value,
    )
    return {"action_id": action.id, "status": action.status.value}


@workflow.defn(name="AutonomyActionWorkflow")
class AutonomyActionWorkflow:
    @workflow.run
    async def run(self, payload: dict[str, Any]) -> dict[str, str]:
        action_id = str(payload.get("action_id", "")).strip()
        if not action_id:
            raise ValueError("autonomy workflow requires action_id")
        return await workflow.execute_activity(
            process_autonomy_action,
            payload,
            start_to_close_timeout=timedelta(minutes=5),
        )
