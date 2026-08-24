"""Thin, audited wrapper over official DBOS management operations."""

from __future__ import annotations

from collections.abc import Callable
from typing import Any

from .diagnostics import Diagnostics
from .ids import stable_id
from .models import ManagementAudit


class ManagementOperationError(RuntimeError):
    pass


class ManagementClient:
    """Translate canonical gate operations to a DBOS client or test double."""

    def __init__(self, client: Any, diagnostics: Diagnostics | None = None) -> None:
        self.client = client
        self.diagnostics = diagnostics or Diagnostics()
        self.audits: list[ManagementAudit] = []

    def _call(self, action: str, workflow_id: str, actor: str, *args: Any, **kwargs: Any) -> Any:
        authorized = actor.startswith("owner:")
        audit = ManagementAudit(
            action=action,
            workflow_id=workflow_id,
            actor=actor,
            authorized=authorized,
            audit_id=stable_id("audit", f"{actor}:{action}:{workflow_id}"),
            details=kwargs,
        )
        self.audits.append(audit)
        self.diagnostics.emit(
            "management_audit",
            intent_id="management",
            workflow_id=workflow_id,
            correlation_id=stable_id("corr", workflow_id),
            details={"action": action, "actor": actor, "authorized": authorized},
        )
        if not authorized:
            raise PermissionError("workflow management requires owner authorization")
        method = self._method(action)
        if action == "fork-from-step":
            step_id = kwargs.pop("step_id")
            try:
                return method(workflow_id, step_id, *args, **kwargs)
            except TypeError:
                # The fixture double uses a keyword-only step_id; DBOS uses
                # the positional start_step parameter.
                return method(workflow_id, step_id=step_id, *args, **kwargs)
        if action == "restart" and getattr(method, "__name__", "") == "fork_workflow":
            return method(workflow_id, 0, *args, **kwargs)
        return method(workflow_id, *args, **kwargs)

    def _method(self, action: str) -> Callable[..., Any]:
        names = {
            "list": ("list_workflows",),
            "get": ("retrieve_workflow", "get_workflow", "get_workflow_status"),
            "pause": ("pause_workflow", "pause"),
            "resume": ("resume_workflow", "resume"),
            "cancel": ("cancel_workflow", "cancel"),
            "restart": ("restart_workflow", "restart", "fork_workflow"),
            "fork-from-step": ("fork_workflow",),
        }
        for name in names[action]:
            method = getattr(self.client, name, None)
            if callable(method):
                return method
        raise ManagementOperationError(f"DBOS client does not expose canonical operation: {action}")

    def list(self, actor: str, **filters: Any) -> Any:
        # list_workflows receives filters rather than a workflow ID.
        authorized = actor.startswith("owner:")
        audit = ManagementAudit(
            action="list",
            workflow_id="*",
            actor=actor,
            authorized=authorized,
            audit_id=stable_id("audit", f"{actor}:list:*"),
            details=filters,
        )
        self.audits.append(audit)
        self.diagnostics.emit(
            "management_audit",
            intent_id="management",
            workflow_id="*",
            correlation_id=stable_id("corr", "*"),
            details={
                "action": "list",
                "actor": actor,
                "authorized": authorized,
                "filters": filters,
            },
        )
        if not authorized:
            raise PermissionError("workflow management requires owner authorization")
        return self._method("list")(**filters)

    def get(self, workflow_id: str, actor: str) -> Any:
        return self._call("get", workflow_id, actor)

    def pause(self, workflow_id: str, actor: str) -> Any:
        return self._call("pause", workflow_id, actor)

    def resume(self, workflow_id: str, actor: str) -> Any:
        return self._call("resume", workflow_id, actor)

    def cancel(self, workflow_id: str, actor: str) -> Any:
        return self._call("cancel", workflow_id, actor)

    def restart(self, workflow_id: str, actor: str) -> Any:
        return self._call("restart", workflow_id, actor)

    def fork_from_step(self, workflow_id: str, step_id: int, actor: str) -> Any:
        return self._call("fork-from-step", workflow_id, actor, step_id=step_id)
