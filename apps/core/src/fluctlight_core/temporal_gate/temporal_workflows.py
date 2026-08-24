"""Temporal workflow and Activity definitions used by the real gate Worker."""

from __future__ import annotations

import asyncio
import os
import time
from dataclasses import asdict
from datetime import timedelta
from typing import Any

from temporalio import activity, workflow
from temporalio.common import RetryPolicy
from temporalio.exceptions import CancelledError as TemporalCancelledError

with workflow.unsafe.imports_passed_through():
    from .ids import correlation_id, intent_id, provider_request_id, result_id, workflow_id
    from .models import GateInput, GateResult, WorkflowStatus
    from .provider import worker_provider
    from .store import PostgresGateStore


@activity.defn(name="fake_h3")
async def fake_h3_activity(payload: dict[str, Any]) -> dict[str, Any]:
    """Heartbeat a bounded fake h3 job and reuse an existing Provider result."""

    request_id = str(payload["provider_request_id"])
    provider = worker_provider()
    existing = provider.lookup(request_id)
    if existing is not None:
        activity.heartbeat({"request_id": request_id, "recovered": True})
        return {
            "request_id": existing.request_id,
            "output": existing.output,
            "effect_count": existing.effect_count,
            "recovered": True,
        }

    duration = float(payload.get("h3_duration_seconds", 0.0))
    interval = max(float(payload.get("heartbeat_interval_seconds", 5.0)), 0.01)
    timeout = float(payload.get("timeout_seconds", 900.0))
    started = time.monotonic()
    elapsed = 0.0
    while elapsed < duration:
        if activity.is_cancelled() or payload.get("cancel", False):
            raise asyncio.CancelledError("fake h3 cancellation observed")
        if payload.get("timeout", False) or elapsed >= timeout:
            raise TimeoutError("fake h3 timeout exceeded")
        activity.heartbeat({"request_id": request_id, "elapsed_seconds": elapsed})
        await asyncio.sleep(min(interval, max(duration - elapsed, 0.0)))
        elapsed = time.monotonic() - started

    result = provider.submit(request_id)
    if payload.get("provider_success_before_checkpoint", False):
        # The first attempt dies after the external effect. A retry looks up
        # the stable request ID above and returns the same result exactly once.
        raise RuntimeError("provider succeeded before Activity checkpoint")
    activity.heartbeat({"request_id": request_id, "completed": True})
    return {
        "request_id": result.request_id,
        "output": result.output,
        "effect_count": result.effect_count,
        "recovered": False,
    }


@activity.defn(name="persist_gate_result")
async def persist_gate_result_activity(payload: dict[str, Any]) -> dict[str, Any]:
    """Persist one final result in the application boundary after completion."""

    database_url = os.environ.get("GATE_DATABASE_URL")
    if not database_url:
        raise RuntimeError("GATE_DATABASE_URL is required for final-result persistence")
    result = GateResult(
        intent_id=str(payload["intent_id"]),
        workflow_id=str(payload["workflow_id"]),
        provider_request_id=str(payload["provider_request_id"]),
        result_id=str(payload["result_id"]),
        status=WorkflowStatus.SUCCEEDED,
        output=str(payload.get("output", "")),
    )
    persisted = await asyncio.to_thread(
        PostgresGateStore(database_url).put_result_once,
        result.workflow_id,
        result,
    )
    return asdict(persisted)


@workflow.defn(name="GateWorkflow")
class GateWorkflow:
    """Minimal durable workflow exercising Temporal's core gate primitives."""

    def __init__(self) -> None:
        self._paused = False
        self._status = WorkflowStatus.QUEUED.value
        self._latest_heartbeat: dict[str, Any] = {}
        self._pending_signals: list[str] = []
        self._repair_version = "gate-v1"
        self._recovery_count = 0

    @workflow.run
    async def run(self, payload: dict[str, Any]) -> dict[str, Any]:
        request = GateInput.from_dict(payload)
        committed_intent = intent_id(request.intent_key)
        execution_id = workflow_id(committed_intent)
        provider_id = provider_request_id(committed_intent)
        iteration = int(payload.get("continue_iteration", 0))
        self._repair_version = request.decision_version
        self._status = WorkflowStatus.RUNNING.value

        if request.sleep_seconds:
            self._status = WorkflowStatus.SLEEPING.value
            await workflow.sleep(timedelta(seconds=request.sleep_seconds))

        await workflow.wait_condition(lambda: not self._paused)
        self._status = WorkflowStatus.RUNNING.value
        try:
            activity_payload = {
                "provider_request_id": provider_id,
                "h3_duration_seconds": request.h3_duration_seconds,
                "heartbeat_interval_seconds": request.heartbeat_interval_seconds,
                "timeout_seconds": request.timeout_seconds,
                "cancel": request.failure.cancel,
                "timeout": request.failure.timeout,
                "provider_success_before_checkpoint": (
                    request.failure.provider_success_before_checkpoint
                ),
            }
            result = await workflow.execute_activity(
                fake_h3_activity,
                activity_payload,
                start_to_close_timeout=timedelta(seconds=request.timeout_seconds),
                heartbeat_timeout=timedelta(
                    seconds=max(request.heartbeat_interval_seconds * 2, 1.0)
                ),
                retry_policy=RetryPolicy(maximum_attempts=3),
                activity_id=provider_id,
            )
        except (asyncio.CancelledError, TemporalCancelledError):
            self._status = WorkflowStatus.CANCELED.value
            return self._result(
                execution_id, committed_intent, provider_id, WorkflowStatus.CANCELED, None
            )
        except Exception as exc:
            self._status = WorkflowStatus.FAILED.value
            return self._result(
                execution_id, committed_intent, provider_id, WorkflowStatus.FAILED, str(exc)
            )

        self._status = WorkflowStatus.SUCCEEDED.value
        self._latest_heartbeat = {"provider_request_id": provider_id, "output": result["output"]}
        persisted_result = await workflow.execute_activity(
            persist_gate_result_activity,
            {
                "intent_id": committed_intent,
                "workflow_id": execution_id,
                "provider_request_id": provider_id,
                "result_id": result_id(execution_id),
                "output": result["output"],
            },
            start_to_close_timeout=timedelta(seconds=10),
            retry_policy=RetryPolicy(maximum_attempts=3),
            activity_id=f"{provider_id}:result",
        )
        if request.continue_after and iteration < request.continue_after:
            self._pending_signals = list(self._pending_signals)
            workflow.continue_as_new(
                {
                    **request.as_dict(),
                    "continue_iteration": iteration + 1,
                    "pending_signals": self._pending_signals,
                }
            )
        return self._result(
            execution_id,
            committed_intent,
            provider_id,
            WorkflowStatus.SUCCEEDED,
            str(result["output"]),
            persisted_result,
        )

    @workflow.signal
    async def pause(self) -> None:
        self._pending_signals.append("pause")
        self._paused = True
        self._status = WorkflowStatus.PAUSED.value

    @workflow.signal
    async def resume(self) -> None:
        self._pending_signals.append("resume")
        self._paused = False
        self._status = WorkflowStatus.RUNNING.value

    @workflow.query
    def status(self) -> dict[str, Any]:
        return {
            "status": self._status,
            "paused": self._paused,
            "pending_signals": list(self._pending_signals),
            "latest_heartbeat": dict(self._latest_heartbeat),
            "repair_version": self._repair_version,
            "recovery_count": self._recovery_count,
        }

    @workflow.update
    async def repair(self, command: dict[str, Any]) -> dict[str, Any]:
        reason = str(command.get("reason", "")).strip()
        expected = command.get("expected_version")
        if not reason:
            raise ValueError("repair requires an audit reason")
        if expected is not None and expected != self._repair_version:
            raise ValueError("repair version conflict")
        self._repair_version = str(command.get("version", self._repair_version))
        return {
            "accepted": True,
            "reason": reason,
            "acknowledged_version": self._repair_version,
        }

    def _result(
        self,
        execution_id: str,
        committed_intent: str,
        provider_id: str,
        status: WorkflowStatus,
        output: str | None,
        persisted_result: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        return {
            "intent_id": committed_intent,
            "workflow_id": execution_id,
            "provider_request_id": provider_id,
            "result_id": result_id(execution_id),
            "status": status.value,
            "output": output,
            "persisted_result": persisted_result,
            "recovery_count": self._recovery_count,
            "correlation_id": correlation_id(committed_intent),
        }
