"""Temporal-only workflow adapter with owner-authorized management operations."""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from typing import Any, Protocol

from temporalio.api.common.v1.message_pb2 import WorkflowExecution
from temporalio.api.enums.v1.reset_pb2 import ResetReapplyType
from temporalio.api.workflowservice.v1 import ResetWorkflowExecutionRequest
from temporalio.common import WorkflowIDReusePolicy
from temporalio.exceptions import WorkflowAlreadyStartedError

TASK_QUEUES = ("interaction", "lifecycle", "media")


class ManagementAuthorizer(Protocol):
    def can_manage_workflows(self, actor_id: str) -> bool: ...


class ManagementAuditSink(Protocol):
    async def record(
        self,
        *,
        action: str,
        workflow_id: str,
        actor_id: str,
        authorized: bool,
        details: dict[str, Any],
    ) -> None: ...


@dataclass(frozen=True, slots=True)
class RestartSpec:
    workflow: Any
    args: tuple[Any, ...]
    task_queue: str


class TemporalRuntime:
    """Application adapter; domain modules provide committed intents and workflow definitions."""

    def __init__(
        self,
        client: Any,
        authorizer: ManagementAuthorizer,
        audit: ManagementAuditSink,
        restart_specs: Callable[[str], Awaitable[RestartSpec]] | None = None,
    ) -> None:
        self.client = client
        self.authorizer = authorizer
        self.audit = audit
        self.restart_specs = restart_specs

    async def _authorize(
        self, action: str, workflow_id: str, actor_id: str, details: dict[str, Any] | None = None
    ) -> None:
        authorized = self.authorizer.can_manage_workflows(actor_id)
        await self.audit.record(
            action=action,
            workflow_id=workflow_id,
            actor_id=actor_id,
            authorized=authorized,
            details=details or {},
        )
        if not authorized:
            raise PermissionError("workflow management requires Owner authorization")

    async def list(self, *, actor_id: str, query: str = "") -> list[Any]:
        await self._authorize("list", "*", actor_id, {"query": query})
        return [item async for item in self.client.list_workflows(query=query)]

    async def get(self, *, actor_id: str, workflow_id: str) -> Any:
        await self._authorize("get", workflow_id, actor_id)
        return self.client.get_workflow_handle(workflow_id)

    async def history(self, *, actor_id: str, workflow_id: str) -> Any:
        await self._authorize("history", workflow_id, actor_id)
        return await self.client.get_workflow_handle(workflow_id).fetch_history()

    async def query(self, *, actor_id: str, workflow_id: str, query: Any) -> Any:
        await self._authorize("query", workflow_id, actor_id)
        return await self.client.get_workflow_handle(workflow_id).query(query)

    async def signal(self, *, actor_id: str, workflow_id: str, signal: Any) -> None:
        await self._authorize("signal", workflow_id, actor_id, {"signal": str(signal)})
        await self.client.get_workflow_handle(workflow_id).signal(signal)

    async def update(self, *, actor_id: str, workflow_id: str, update: Any, arg: Any) -> Any:
        await self._authorize("update", workflow_id, actor_id)
        return await self.client.get_workflow_handle(workflow_id).execute_update(update, arg)

    async def cancel(self, *, actor_id: str, workflow_id: str) -> None:
        await self._authorize("cancel", workflow_id, actor_id)
        await self.client.get_workflow_handle(workflow_id).cancel()

    async def terminate(self, *, actor_id: str, workflow_id: str, reason: str) -> None:
        await self._authorize("terminate", workflow_id, actor_id, {"reason": reason})
        await self.client.get_workflow_handle(workflow_id).terminate(reason)

    async def restart(self, *, actor_id: str, workflow_id: str) -> Any:
        await self._authorize("restart", workflow_id, actor_id)
        if self.restart_specs is None:
            raise RuntimeError("restart requires a committed workflow intent")
        spec = await self.restart_specs(workflow_id)
        if spec.task_queue not in TASK_QUEUES:
            raise ValueError("restart intent uses an unknown task queue")
        try:
            return await self.client.start_workflow(
                spec.workflow,
                *spec.args,
                id=workflow_id,
                task_queue=spec.task_queue,
                id_reuse_policy=WorkflowIDReusePolicy.ALLOW_DUPLICATE,
            )
        except WorkflowAlreadyStartedError:
            return self.client.get_workflow_handle(workflow_id)

    async def reset(self, *, actor_id: str, workflow_id: str, history_point: int) -> Any:
        if history_point < 1:
            raise ValueError("history_point must be a positive workflow task finish event ID")
        await self._authorize("reset", workflow_id, actor_id, {"history_point": history_point})
        await self.client.workflow_service.reset_workflow_execution(
            ResetWorkflowExecutionRequest(
                namespace=self.client.namespace,
                workflow_execution=WorkflowExecution(workflow_id=workflow_id, run_id=""),
                reason=f"Owner reset from history event {history_point}",
                reset_reapply_type=ResetReapplyType.RESET_REAPPLY_TYPE_UNSPECIFIED,
                request_id=f"reset-{workflow_id}-{history_point}",
                workflow_task_finish_event_id=history_point,
            )
        )
        return self.client.get_workflow_handle(workflow_id)
