"""Deterministic gate runtime and failure/recovery state machine."""

from __future__ import annotations

import threading
from dataclasses import replace
from typing import Callable

from .diagnostics import Diagnostics
from .ids import correlation_id, provider_request_id, stable_id, workflow_id
from .models import (
    GateInput,
    GateResult,
    StepStatus,
    WorkflowRecord,
    WorkflowStatus,
)
from .provider import CooperativeCancellation, FakeH3Provider, ProviderTimeout
from .queues import LocalQueueLimits
from .store import GateStore, InMemoryGateStore


class GateInvariantError(RuntimeError):
    """Raised when a workflow tries to cross an uncommitted boundary."""


class CrashInjected(RuntimeError):
    """Represents process death at a named durable boundary."""


class GateRuntime:
    """Run the DBOS gate scenarios against deterministic gate persistence.

    The state machine mirrors the production boundary: the intent is committed
    before a workflow is enqueued, provider effects are keyed independently,
    and the final result is inserted once after recovery.
    """

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
        self._requests: dict[str, GateInput] = {}
        self._sleep_due: dict[str, float] = {}

    def commit_intent(self, request: GateInput) -> tuple[str, str]:
        intent_id = stable_id("intent", request.intent_key)
        wf_id = workflow_id(intent_id)
        if not self.store.commit_intent(intent_id, wf_id, request):
            existing = self.store.get_workflow(wf_id)
            if existing is None:
                raise GateInvariantError("intent exists without a workflow record")
            persisted_request = self.store.get_intent(intent_id)
            if persisted_request is not None:
                self._requests[wf_id] = persisted_request
            return intent_id, wf_id
        self._requests[wf_id] = request
        self.store.put_workflow(
            WorkflowRecord(
                intent_id=intent_id,
                workflow_id=wf_id,
                queue=request.queue,
                status=WorkflowStatus.QUEUED,
                provider_request_id=provider_request_id(intent_id),
                step_status={"durable_sleep": StepStatus.PENDING, "fake_h3": StepStatus.PENDING},
                metadata={"decision_version": request.decision_version},
            )
        )
        self.diagnostics.emit(
            "intent_committed",
            intent_id=intent_id,
            workflow_id=wf_id,
            provider_request_id=provider_request_id(intent_id),
            correlation_id=correlation_id(intent_id),
            details={"queue": request.queue, "decision_version": request.decision_version},
        )
        return intent_id, wf_id

    def start(self, request: GateInput, *, execute: bool = True) -> GateResult | WorkflowRecord:
        intent_id, wf_id = self.commit_intent(request)
        record = self.store.get_workflow(wf_id)
        if record is None:
            raise GateInvariantError("committed intent has no workflow record")
        if not execute:
            return record
        if record.result is not None:
            return record.result
        return self.execute(wf_id)

    def execute(self, wf_id: str) -> GateResult | WorkflowRecord:
        with self._lock:
            record = self._require(wf_id)
            request = self._requests.get(wf_id)
            if request is None:
                raise GateInvariantError("workflow input is unavailable after restart")
            if record.result is not None:
                return record.result
            if record.status == WorkflowStatus.PAUSED:
                return record
            if record.status == WorkflowStatus.SLEEPING:
                due = self._sleep_deadline(record)
                if self._clock is not None and self._clock() < due:
                    return record
            record = replace(record, status=WorkflowStatus.RUNNING)
            self.store.put_workflow(record)
            self.diagnostics.emit(
                "workflow_started",
                intent_id=record.intent_id,
                workflow_id=wf_id,
                provider_request_id=record.provider_request_id,
                correlation_id=correlation_id(record.intent_id),
            )

            try:
                with self.queue_limits.acquire(request.queue):
                    return self._run(record, request)
            except CrashInjected as exc:
                record = replace(record, status=WorkflowStatus.CRASHED, error=str(exc))
                self.store.put_workflow(record)
                self.diagnostics.emit(
                    "worker_crashed",
                    intent_id=record.intent_id,
                    workflow_id=wf_id,
                    provider_request_id=record.provider_request_id,
                    correlation_id=correlation_id(record.intent_id),
                    details={"error": str(exc)},
                )
                return record
            except CooperativeCancellation as exc:
                return self._settle(record, WorkflowStatus.CANCELED, str(exc))
            except ProviderTimeout as exc:
                return self._settle(record, WorkflowStatus.FAILED, str(exc))

    def _run(self, record: WorkflowRecord, request: GateInput) -> GateResult | WorkflowRecord:
        if request.sleep_seconds > 0 and record.checkpoint is None:
            if self._clock is None:
                # Unit tests never wait; production DBOS uses DBOS.sleep below.
                due = request.sleep_seconds
            else:
                due = self._clock() + request.sleep_seconds
            self._sleep_due[record.workflow_id] = due
            record = replace(
                record,
                status=WorkflowStatus.SLEEPING,
                checkpoint="durable_sleep",
                metadata={**record.metadata, "sleep_due": due},
            )
            self.store.put_workflow(record)
            self.diagnostics.emit(
                "durable_sleep_scheduled",
                intent_id=record.intent_id,
                workflow_id=record.workflow_id,
                step_id="durable_sleep",
                provider_request_id=record.provider_request_id,
                correlation_id=correlation_id(record.intent_id),
                details={"seconds": request.sleep_seconds},
            )
            return record

        request_id = record.provider_request_id
        if request_id is None:
            raise GateInvariantError("provider request ID must be committed before execution")
        provider_result = self.store.get_provider_result(record.workflow_id)
        recovered_provider_result = provider_result is not None
        if provider_result is None:
            provider_result = self.provider.lookup(request_id)
            recovered_provider_result = provider_result is not None
        if provider_result is None:
            if request.failure.cancel:
                raise CooperativeCancellation("cooperative cancellation observed")
            if request.failure.timeout:
                raise ProviderTimeout("fake h3 timeout exceeded")
            provider_result = self._run_fake_h3(record, request)
            if request.failure.provider_success_before_checkpoint:
                # The fake provider has succeeded, but the process dies before
                # the durable result checkpoint. Recovery looks up the stable
                # request ID and records the same effect once.
                raise CrashInjected("provider succeeded before checkpoint")
            self.store.put_provider_result(record.workflow_id, provider_result)
            record = replace(record, provider_result=provider_result, checkpoint="provider_result")
            self.store.put_workflow(record)
        else:
            self.store.put_provider_result(record.workflow_id, provider_result)
            self.diagnostics.emit(
                "provider_result_recovered",
                intent_id=record.intent_id,
                workflow_id=record.workflow_id,
                step_id="fake_h3",
                provider_request_id=request_id,
                correlation_id=correlation_id(record.intent_id),
                details={"effect_count": provider_result.effect_count},
            )

        if (
            not recovered_provider_result
            and (
                request.failure.crash_after_provider_checkpoint
                or request.failure.crash_before_result_commit
            )
        ):
            raise CrashInjected("process died before final result commit")
        return self._settle(record, WorkflowStatus.SUCCEEDED, output=provider_result.output)

    def _run_fake_h3(self, record: WorkflowRecord, request: GateInput):
        request_id = record.provider_request_id
        if request_id is None:
            raise GateInvariantError("fake h3 requires stable provider request ID")
        elapsed = 0.0
        interval = max(request.heartbeat_interval_seconds, 0.001)
        duration = max(request.h3_duration_seconds, 0.0)
        while elapsed < duration:
            if request.failure.cancel:
                raise CooperativeCancellation("cooperative cancellation observed")
            if request.failure.timeout or elapsed >= request.timeout_seconds:
                raise ProviderTimeout("fake h3 timeout exceeded")
            self.provider.heartbeat(request_id)
            self.diagnostics.emit(
                "provider_heartbeat",
                intent_id=record.intent_id,
                workflow_id=record.workflow_id,
                step_id="fake_h3",
                provider_request_id=request_id,
                attempt=1,
                correlation_id=correlation_id(record.intent_id),
                details={"elapsed_seconds": elapsed},
            )
            elapsed += interval
        result = self.provider.submit(request_id)
        self.diagnostics.emit(
            "provider_succeeded",
            intent_id=record.intent_id,
            workflow_id=record.workflow_id,
            step_id="fake_h3",
            provider_request_id=request_id,
            attempt=1,
            correlation_id=correlation_id(record.intent_id),
            details={"effect_count": result.effect_count},
        )
        return result

    def _settle(
        self,
        record: WorkflowRecord,
        status: WorkflowStatus,
        error: str | None = None,
        output: str | None = None,
    ) -> GateResult:
        result_id = stable_id("result", record.workflow_id)
        result = GateResult(
            intent_id=record.intent_id,
            workflow_id=record.workflow_id,
            provider_request_id=record.provider_request_id or "",
            result_id=result_id,
            status=status,
            output=output,
            recovery_count=record.recovery_count,
        )
        result = self.store.put_result_once(record.workflow_id, result)
        updated = replace(record, status=status, result=result, error=error)
        self.store.put_workflow(updated)
        self.diagnostics.emit(
            "result_committed",
            intent_id=record.intent_id,
            workflow_id=record.workflow_id,
            provider_request_id=record.provider_request_id,
            result_id=result.result_id,
            correlation_id=correlation_id(record.intent_id),
            details={"status": status.value, "error": error},
        )
        return result

    def recover(self, wf_id: str) -> GateResult | WorkflowRecord:
        with self._lock:
            record = self._require(wf_id)
            if record.result is not None:
                return record.result
            if record.status == WorkflowStatus.SLEEPING and self._clock is not None:
                due = self._sleep_deadline(record)
                if self._clock() < due:
                    return record
            updated = replace(
                record,
                status=WorkflowStatus.RUNNING,
                recovery_count=record.recovery_count + 1,
            )
            self.store.put_workflow(updated)
            self.diagnostics.emit(
                "workflow_recovery_started",
                intent_id=record.intent_id,
                workflow_id=wf_id,
                provider_request_id=record.provider_request_id,
                correlation_id=correlation_id(record.intent_id),
                details={"recovery_count": updated.recovery_count},
            )
            request = self._requests.get(wf_id) or self.store.get_intent(record.intent_id)
            if request is None:
                raise GateInvariantError("workflow input is unavailable after restart")
            self._requests[wf_id] = request
            return self._run(updated, request)

    def pause(self, wf_id: str) -> WorkflowRecord:
        record = self._require(wf_id)
        updated = replace(record, status=WorkflowStatus.PAUSED)
        self.store.put_workflow(updated)
        return updated

    def resume(self, wf_id: str) -> WorkflowRecord:
        record = self._require(wf_id)
        updated = replace(record, status=WorkflowStatus.QUEUED)
        self.store.put_workflow(updated)
        return updated

    def cancel(self, wf_id: str) -> GateResult | WorkflowRecord:
        record = self._require(wf_id)
        return self._settle(record, WorkflowStatus.CANCELED, "administrative cancellation")

    def _require(self, wf_id: str) -> WorkflowRecord:
        record = self.store.get_workflow(wf_id)
        if record is None:
            raise KeyError(wf_id)
        return record

    def _sleep_deadline(self, record: WorkflowRecord) -> float:
        persisted = record.metadata.get("sleep_due")
        if persisted is not None:
            return float(persisted)
        return self._sleep_due.get(record.workflow_id, 0)
