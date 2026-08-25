"""Durable Temporal adapter for frozen autonomy actions."""

from __future__ import annotations

from typing import Any

from temporalio import workflow


@workflow.defn(name="AutonomyActionWorkflow")
class AutonomyActionWorkflow:
    @workflow.run
    async def run(self, payload: dict[str, Any]) -> dict[str, str]:
        action_id = str(payload.get("action_id", "")).strip()
        if not action_id:
            raise ValueError("autonomy workflow requires action_id")
        return {"action_id": action_id, "status": "accepted"}
