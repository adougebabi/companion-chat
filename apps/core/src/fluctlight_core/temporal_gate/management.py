"""Audited adapter over official Temporal client management operations."""

from __future__ import annotations

from typing import Any

from .diagnostics import Diagnostics
from .ids import audit_id, stable_id
from .models import ManagementAudit, RepairCommand


class ManagementOperationError(RuntimeError):
    pass


class TemporalManagementClient:
    """Keep authorization/audit at the application command boundary."""

    def __init__(self, client: Any, diagnostics: Diagnostics | None = None) -> None:
        self.client = client
        self.diagnostics = diagnostics or Diagnostics()
        self.audits: list[ManagementAudit] = []

    def _authorize(
        self,
        action: str,
        execution_id: str,
        actor: str,
        details: dict[str, Any] | None = None,
    ) -> None:
        authorized = actor.startswith("owner:")
        audit = ManagementAudit(
            action,
            execution_id,
            actor,
            authorized,
            audit_id(actor, action, execution_id),
            details or {},
        )
        self.audits.append(audit)
        self.diagnostics.emit(
            "management_audit",
            intent_id="management",
            workflow_id=execution_id,
            correlation_id=stable_id("corr", execution_id),
            details={
                "action": action,
                "actor": actor,
                "authorized": authorized,
                **(details or {}),
            },
        )
        if not authorized:
            raise PermissionError("workflow management requires owner authorization")

    async def list(self, actor: str, query: str = "", **kwargs: Any) -> list[Any]:
        self._authorize("list", "*", actor, {"query": query, **kwargs})
        return [item async for item in self.client.list_workflows(query=query)]

    async def get(self, execution_id: str, actor: str) -> Any:
        self._authorize("get", execution_id, actor)
        return self.client.get_workflow_handle(execution_id)

    async def query(self, execution_id: str, actor: str) -> Any:
        self._authorize("query", execution_id, actor)
        handle = self.client.get_workflow_handle(execution_id)
        from .temporal_workflows import GateWorkflow

        return await handle.query(GateWorkflow.status)

    async def pause(self, execution_id: str, actor: str) -> None:
        self._authorize("pause", execution_id, actor)
        from .temporal_workflows import GateWorkflow

        await self.client.get_workflow_handle(execution_id).signal(GateWorkflow.pause)

    async def resume(self, execution_id: str, actor: str) -> None:
        self._authorize("resume", execution_id, actor)
        from .temporal_workflows import GateWorkflow

        await self.client.get_workflow_handle(execution_id).signal(GateWorkflow.resume)

    async def repair(self, execution_id: str, command: RepairCommand, actor: str) -> Any:
        self._authorize("repair", execution_id, actor, {"reason": command.reason})
        from .temporal_workflows import GateWorkflow

        return await self.client.get_workflow_handle(execution_id).execute_update(
            GateWorkflow.repair,
            {"reason": command.reason, "expected_version": command.expected_version},
        )

    async def cancel(self, execution_id: str, actor: str) -> None:
        self._authorize("cancel", execution_id, actor)
        await self.client.get_workflow_handle(execution_id).cancel()

    async def terminate(self, execution_id: str, actor: str, reason: str) -> None:
        self._authorize("terminate", execution_id, actor, {"reason": reason})
        await self.client.get_workflow_handle(execution_id).terminate(reason)

    async def restart(self, execution_id: str, actor: str) -> Any:
        self._authorize("restart", execution_id, actor)
        return self.client.get_workflow_handle(execution_id)

    async def reset(self, execution_id: str, actor: str, history_point: int) -> Any:
        self._authorize("reset", execution_id, actor, {"history_point": history_point})
        if history_point < 1:
            raise ValueError("history_point must be a positive Event History point")
        from temporalio.api.common.v1.message_pb2 import WorkflowExecution
        from temporalio.api.enums.v1.reset_pb2 import ResetReapplyType
        from temporalio.api.workflowservice.v1 import ResetWorkflowExecutionRequest

        await self.client.workflow_service.reset_workflow_execution(
            ResetWorkflowExecutionRequest(
                namespace=self.client.namespace,
                workflow_execution=WorkflowExecution(workflow_id=execution_id, run_id=""),
                reason=f"owner reset from event {history_point}",
                reset_reapply_type=ResetReapplyType.RESET_REAPPLY_TYPE_UNSPECIFIED,
                request_id=stable_id("reset", f"{execution_id}:{history_point}"),
                workflow_task_finish_event_id=history_point,
            )
        )
        return self.client.get_workflow_handle(execution_id)


ManagementClient = TemporalManagementClient
