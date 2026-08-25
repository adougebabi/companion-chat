"""Durable Temporal adapter for committed media intents."""

from __future__ import annotations

from typing import Any

from temporalio import workflow


@workflow.defn(name="MediaGenerationWorkflow")
class MediaGenerationWorkflow:
    @workflow.run
    async def run(self, payload: dict[str, Any]) -> dict[str, str]:
        intent_id = str(payload.get("intent_id", "")).strip()
        if not intent_id:
            raise ValueError("media workflow requires intent_id")
        return {"intent_id": intent_id, "status": "accepted"}
