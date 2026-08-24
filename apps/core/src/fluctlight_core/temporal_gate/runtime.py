"""Deterministic local mirror of the Temporal gate contract.

This is intentionally a test fixture, not a second production runtime. The
Compose path uses Temporal Server and the SDK Worker in ``temporal_workflows``.
"""

from __future__ import annotations

import threading
from collections.abc import Callable
from dataclasses import replace

from .diagnostics import Diagnostics
from .ids import correlation_id, intent_id, provider_request_id, result_id, workflow_id
from .models import (
    GateInput,
    GateResult,
    RepairCommand,
    RepairResult,
    StepStatus,
    WorkflowRecord,
    WorkflowStatus,
)
from .provider import CooperativeCancellation, FakeH3Provider, ProviderTimeout
from .queues import LocalQueueLimits
from .store import GateStore, InMemoryGateStore


class GateInvariantError(RuntimeError):
    """Raised when a fixture crosses an uncommitted boundary."""


class CrashInjected(RuntimeError):
    """Represents process death at a named durable boundary."""


class TemporalGateRuntime:
    """Exercise stable identity, timer, recovery, and management invariants."""

    def __init__(
        self,
        *,
        store: GateStore | None = None,
        provider: FakeH3Provider | None = None,
        diagnostics: Diagnostics | None = None,
        queue_limits: LocalQueueLimits | None = None,
        clock: Callable[[], float] | None = None,
    ) -> None:
        self.store = store or InMemoryGateStore()
        self.provider = provider or FakeH3Provider()
        self.diagnostics = diagnostics or Diagnostics()
        self.queue_limits = queue_limits or LocalQueueLimits()
        self._clock = clock
        self._lock = threading.RLock()

    def commit_intent(self, request: GateInput) -> tuple[str, str]:
        committed_intent = intent_id(request.intent_key)
        execution_id = workflow_id(committed_intent)
        if not self.store.commit_intent(committed_intent, execution_id, request):
            if self.store.get_workflow(execution_id) is None:
                self.store.put_workflow(
                    WorkflowRecord(
                        committed_intent,
                        execution_id,
                        request.queue,
                        WorkflowStatus.QUEUED,
                        provider_request_id=provider_request_id(committed_intent),
                    )
                )
            return committed_intent, execution_id
        self.store.put_workflow(
            WorkflowRecord(
                committed_intent,
                execution_id,
                request.queue,
                WorkflowStatus.QUEUED,
                step_status={"durable_timer": StepStatus.PENDING, "fake_h3": StepStatus.PENDING},
                provider_request_id=provider_request_id(committed_intent),
                metadata={
                    "decision_version": request.decision_version,
                    "history_version": "gate-v1",
                },
            )
        )
        self._emit(
            "intent_committed",
            committed_intent,
            execution_id,
            provider_request_id=provider_request_id(committed_intent),
            details={"queue": request.queue, "decision_version": request.decision_version},
        )
        return committed_intent, execution_id

    def start(self, request: GateInput, *, execute: bool = True) -> GateResult | WorkflowRecord:
        _, execution_id = self.commit_intent(request)
        record = self._require(execution_id)
        if not execute or record.result is not None:
            return record.result or record
        return self.execute(execution_id)

    def execute(self, execution_id: str) -> GateResult | WorkflowRecord:
        with self._lock:
            record = self._require(execution_id)
            request = self.store.get_intent(record.intent_id)
            if request is None:
                raise GateInvariantError("committed intent is unavailable")
            if record.result is not None or record.status == WorkflowStatus.PAUSED:
                return record.result or record
            if record.status == WorkflowStatus.SLEEPING and self._not_due(record):
                return record
            record = replace(record, status=WorkflowStatus.RUNNING)
            self.store.put_workflow(record)
            self._emit(
                "workflow_started",
                record.intent_id,
                execution_id,
                provider_request_id=record.provider_request_id,
            )
            try:
                with self.queue_limits.acquire(request.queue):
                    return self._run(record, request)
            except CrashInjected as exc:
                crashed = replace(record, status=WorkflowStatus.CRASHED, error=str(exc))
                self.store.put_workflow(crashed)
                self._emit(
                    "worker_crashed", record.intent_id, execution_id, details={"error": str(exc)}
                )
                return crashed
            except CooperativeCancellation as exc:
                return self._settle(record, WorkflowStatus.CANCELED, error=str(exc))
            except ProviderTimeout as exc:
                return self._settle(record, WorkflowStatus.FAILED, error=str(exc))

    def _run(self, record: WorkflowRecord, request: GateInput) -> GateResult | WorkflowRecord:
        if request.sleep_seconds and record.checkpoint is None:
            due = (self._clock() if self._clock else 0.0) + request.sleep_seconds
            sleeping = replace(
                record,
                status=WorkflowStatus.SLEEPING,
                checkpoint="durable_timer",
                metadata={**record.metadata, "timer_due": due},
            )
            self.store.put_workflow(sleeping)
            self._emit(
                "durable_timer_scheduled",
                record.intent_id,
                record.workflow_id,
                step_id="durable_timer",
            )
            return sleeping

        request_id = record.provider_request_id
        if request_id is None:
            raise GateInvariantError("Provider ID must be committed before execution")
        provider_result = self.store.get_provider_result(
            record.workflow_id
        ) or self.provider.lookup(request_id)
        recovered = provider_result is not None
        if provider_result is None:
            if request.failure.cancel:
                raise CooperativeCancellation("cooperative cancellation observed")
            if request.failure.timeout:
                raise ProviderTimeout("fake h3 timeout exceeded")
            provider_result = self._fake_h3(record, request)
            if request.failure.provider_success_before_checkpoint:
                raise CrashInjected("provider succeeded before checkpoint")
            self.store.put_provider_result(record.workflow_id, provider_result)
        else:
            self.store.put_provider_result(record.workflow_id, provider_result)
            self._emit(
                "provider_result_recovered", record.intent_id, record.workflow_id, step_id="fake_h3"
            )
        if not recovered and (
            request.failure.crash_after_provider_checkpoint
            or request.failure.crash_before_result_commit
        ):
            raise CrashInjected("process died before final result commit")
        return self._settle(record, WorkflowStatus.SUCCEEDED, output=provider_result.output)

    def _fake_h3(self, record: WorkflowRecord, request: GateInput):
        elapsed = 0.0
        interval = max(request.heartbeat_interval_seconds, 0.001)
        while elapsed < request.h3_duration_seconds:
            if request.failure.cancel:
                raise CooperativeCancellation("cooperative cancellation observed")
            if request.failure.timeout or elapsed >= request.timeout_seconds:
                raise ProviderTimeout("fake h3 timeout exceeded")
            self.provider.heartbeat(record.provider_request_id or "")
            self._emit(
                "provider_heartbeat",
                record.intent_id,
                record.workflow_id,
                step_id="fake_h3",
                details={"elapsed_seconds": elapsed},
            )
            elapsed += interval
        result = self.provider.submit(record.provider_request_id or "")
        self._emit("provider_succeeded", record.intent_id, record.workflow_id, step_id="fake_h3")
        return result

    def _settle(
        self,
        record: WorkflowRecord,
        status: WorkflowStatus,
        *,
        error: str | None = None,
        output: str | None = None,
    ):
        result = GateResult(
            record.intent_id,
            record.workflow_id,
            record.provider_request_id or "",
            result_id(record.workflow_id),
            status,
            output,
            record.recovery_count,
        )
        result = self.store.put_result_once(record.workflow_id, result)
        self.store.put_workflow(replace(record, status=status, result=result, error=error))
        self._emit(
            "result_committed",
            record.intent_id,
            record.workflow_id,
            result_id=result.result_id,
            details={"status": status.value},
        )
        return result

    def recover(self, execution_id: str) -> GateResult | WorkflowRecord:
        record = self._require(execution_id)
        if record.result is not None or (
            record.status == WorkflowStatus.SLEEPING and self._not_due(record)
        ):
            return record.result or record
        restarted = replace(
            record, status=WorkflowStatus.RUNNING, recovery_count=record.recovery_count + 1
        )
        self.store.put_workflow(restarted)
        self._emit(
            "workflow_recovery_started",
            record.intent_id,
            execution_id,
            details={"recovery_count": restarted.recovery_count},
        )
        return self.execute(execution_id)

    def pause(self, execution_id: str) -> WorkflowRecord:
        return self._update_status(execution_id, WorkflowStatus.PAUSED)

    def resume(self, execution_id: str) -> WorkflowRecord:
        return self._update_status(execution_id, WorkflowStatus.QUEUED)

    def cancel(self, execution_id: str) -> GateResult | WorkflowRecord:
        record = self._require(execution_id)
        return record.result or self._settle(
            record, WorkflowStatus.CANCELED, error="administrative cancellation"
        )

    def terminate(self, execution_id: str) -> GateResult | WorkflowRecord:
        record = self._require(execution_id)
        return record.result or self._settle(
            record, WorkflowStatus.TERMINATED, error="emergency termination"
        )

    def repair(self, execution_id: str, command: RepairCommand) -> RepairResult:
        if not command.reason.strip():
            raise ValueError("repair requires an audit reason")
        record = self._require(execution_id)
        version = str(record.metadata.get("decision_version", "gate-v1"))
        if command.expected_version and command.expected_version != version:
            return RepairResult(False, execution_id, "version conflict", version)
        return RepairResult(True, execution_id, command.reason, version)

    def restart(self, execution_id: str) -> WorkflowRecord:
        record = self._require(execution_id)
        return self._update_status(
            execution_id, WorkflowStatus.QUEUED, record=replace(record, result=None, error=None)
        )

    def reset(self, execution_id: str, history_point: int) -> WorkflowRecord:
        if history_point < 0:
            raise ValueError("history_point must be non-negative")
        record = self._require(execution_id)
        updated = replace(
            record,
            status=WorkflowStatus.QUEUED,
            result=None,
            checkpoint=None,
            metadata={**record.metadata, "reset_from_event": history_point},
        )
        self.store.put_workflow(updated)
        return updated

    def continue_as_new(
        self, execution_id: str, pending_signals: tuple[str, ...] = ()
    ) -> WorkflowRecord:
        record = self._require(execution_id)
        updated = replace(
            record,
            status=WorkflowStatus.QUEUED,
            metadata={
                **record.metadata,
                "continue_as_new": True,
                "pending_signals": pending_signals,
            },
        )
        self.store.put_workflow(updated)
        return updated

    def query(self, execution_id: str) -> WorkflowRecord:
        return self._require(execution_id)

    def list(self) -> list[WorkflowRecord]:
        return self.store.list_workflows()

    def _not_due(self, record: WorkflowRecord) -> bool:
        if self._clock is None:
            return False
        return self._clock() < float(record.metadata.get("timer_due", 0.0))

    def _update_status(
        self, execution_id: str, status: WorkflowStatus, record: WorkflowRecord | None = None
    ) -> WorkflowRecord:
        current = record or self._require(execution_id)
        updated = replace(current, status=status)
        self.store.put_workflow(updated)
        return updated

    def _require(self, execution_id: str) -> WorkflowRecord:
        record = self.store.get_workflow(execution_id)
        if record is None:
            raise KeyError(execution_id)
        return record

    def _emit(self, event: str, intent: str, execution: str, **kwargs: object) -> None:
        self.diagnostics.emit(
            event,
            intent_id=intent,
            workflow_id=execution,
            correlation_id=correlation_id(intent),
            **kwargs,
        )


GateRuntime = TemporalGateRuntime
