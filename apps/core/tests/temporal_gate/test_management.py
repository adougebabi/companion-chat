from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator, Coroutine
from typing import Any

import pytest
from fluctlight_core.temporal_gate.diagnostics import Diagnostics
from fluctlight_core.temporal_gate.management import TemporalManagementClient
from fluctlight_core.temporal_gate.models import RepairCommand


class FakeWorkflowHandle:
    def __init__(self, execution_id: str) -> None:
        self.execution_id = execution_id
        self.calls: list[tuple[str, object]] = []

    async def query(self, query_type: object) -> dict[str, object]:
        self.calls.append(("query", query_type))
        return {"workflow_id": self.execution_id, "status": "running"}

    async def signal(self, signal_type: object) -> None:
        self.calls.append(("signal", signal_type))

    async def execute_update(
        self, update_type: object, payload: dict[str, object]
    ) -> dict[str, object]:
        self.calls.append(("update", (update_type, payload)))
        return {"accepted": True, **payload}

    async def cancel(self) -> None:
        self.calls.append(("cancel", None))

    async def terminate(self, reason: str) -> None:
        self.calls.append(("terminate", reason))


class FakeWorkflowService:
    def __init__(self) -> None:
        self.requests: list[Any] = []

    async def reset_workflow_execution(self, request: Any) -> None:
        self.requests.append(request)


class FakeTemporalClient:
    namespace = "default"

    def __init__(self) -> None:
        self.handles: dict[str, FakeWorkflowHandle] = {}
        self.workflow_service = FakeWorkflowService()
        self.list_queries: list[str] = []

    def list_workflows(self, **filters: object) -> AsyncIterator[dict[str, str]]:
        query = str(filters["query"])
        self.list_queries.append(query)

        async def executions() -> AsyncIterator[dict[str, str]]:
            yield {"workflow_id": "wf-1"}

        return executions()

    def get_workflow_handle(self, execution_id: str) -> FakeWorkflowHandle:
        return self.handles.setdefault(execution_id, FakeWorkflowHandle(execution_id))


def run(coro: Coroutine[Any, Any, Any]) -> Any:
    return asyncio.run(coro)


def test_canonical_management_operations_call_official_client_and_audit() -> None:
    client = FakeTemporalClient()
    management = TemporalManagementClient(client, Diagnostics())

    assert run(management.list("owner:local", query="status:running")) == [{"workflow_id": "wf-1"}]
    assert management.audits[-1].action == "list"
    assert management.audits[-1].details["query"] == "status:running"

    handle = run(management.get("wf-1", "owner:local"))
    assert handle is client.handles["wf-1"]
    assert run(management.query("wf-1", "owner:local")) == {
        "workflow_id": "wf-1",
        "status": "running",
    }
    run(management.pause("wf-1", "owner:local"))
    run(management.resume("wf-1", "owner:local"))
    repair = run(
        management.repair(
            "wf-1",
            RepairCommand("owner-approved repair", expected_version="gate-v1"),
            "owner:local",
        )
    )
    run(management.cancel("wf-1", "owner:local"))
    run(management.terminate("wf-1", "owner:local", "emergency policy"))
    assert repair["accepted"] is True
    assert [call[0] for call in client.handles["wf-1"].calls] == [
        "query",
        "signal",
        "signal",
        "update",
        "cancel",
        "terminate",
    ]


def test_restart_and_reset_are_audited_and_reset_uses_positive_history_event() -> None:
    client = FakeTemporalClient()
    management = TemporalManagementClient(client, Diagnostics())

    restarted = run(management.restart("wf-1", "owner:local"))
    assert restarted is client.handles["wf-1"]
    reset_handle = run(management.reset("wf-1", "owner:local", history_point=7))

    assert reset_handle is client.handles["wf-1"]
    request = client.workflow_service.requests[0]
    assert request.workflow_execution.workflow_id == "wf-1"
    assert request.workflow_task_finish_event_id == 7
    assert request.reason == "owner reset from event 7"
    assert [audit.action for audit in management.audits] == ["restart", "reset"]

    with pytest.raises(ValueError, match="positive"):
        run(management.reset("wf-1", "owner:local", history_point=0))


def test_management_requires_owner_authorization_before_client_calls() -> None:
    client = FakeTemporalClient()
    management = TemporalManagementClient(client, Diagnostics())

    with pytest.raises(PermissionError, match="owner authorization"):
        run(management.pause("wf-1", "human:untrusted"))

    audit = management.audits[-1]
    assert audit.authorized is False
    assert audit.actor == "human:untrusted"
    assert client.handles == {}
