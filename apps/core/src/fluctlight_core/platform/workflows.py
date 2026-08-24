"""A Temporal management fixture used only to validate the platform boundary."""

from __future__ import annotations

from dataclasses import asdict, dataclass
from datetime import timedelta
from typing import Any

from temporalio import workflow


@dataclass(frozen=True, slots=True)
class PlatformWorkflowInput:
    intent_id: str
    delay_seconds: float = 0.0
    version: str = "platform-v1"

    def as_dict(self) -> dict[str, Any]:
        return asdict(self)


@workflow.defn(name="PlatformControlWorkflow")
class PlatformControlWorkflow:
    """Exercises durable controls without defining product workflow semantics."""

    def __init__(self) -> None:
        self._paused = False
        self._status = "queued"
        self._version = "platform-v1"

    @workflow.run
    async def run(self, payload: dict[str, Any]) -> dict[str, str]:
        input_value = PlatformWorkflowInput(**payload)
        self._version = input_value.version
        self._status = "running"
        await workflow.wait_condition(lambda: not self._paused)
        if input_value.delay_seconds:
            self._status = "sleeping"
            await workflow.sleep(timedelta(seconds=input_value.delay_seconds))
        self._status = "completed"
        return {"intent_id": input_value.intent_id, "status": self._status}

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

    @workflow.update
    async def repair(self, command: dict[str, str]) -> dict[str, str | bool]:
        reason = command.get("reason", "").strip()
        if not reason:
            raise ValueError("repair requires an audit reason")
        expected = command.get("expected_version")
        if expected and expected != self._version:
            raise ValueError("repair version conflict")
        self._version = command.get("version", self._version)
        return {"accepted": True, "reason": reason, "acknowledged_version": self._version}
