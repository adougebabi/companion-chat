from __future__ import annotations

import pytest

from fluctlight_core.workflow_gate.diagnostics import Diagnostics
from fluctlight_core.workflow_gate.management import ManagementClient


class FakeDBOSClient:
    def __init__(self) -> None:
        self.calls: list[tuple[str, tuple, dict]] = []

    def list_workflows(self, **filters):
        self.calls.append(("list", (), filters))
        return ["wf"]

    def get_workflow(self, workflow_id):
        self.calls.append(("get", (workflow_id,), {}))
        return workflow_id

    def pause_workflow(self, workflow_id):
        self.calls.append(("pause", (workflow_id,), {}))

    def resume_workflow(self, workflow_id):
        self.calls.append(("resume", (workflow_id,), {}))

    def cancel_workflow(self, workflow_id):
        self.calls.append(("cancel", (workflow_id,), {}))

    def restart_workflow(self, workflow_id):
        self.calls.append(("restart", (workflow_id,), {}))

    def fork_workflow(self, workflow_id, *, step_id):
        self.calls.append(("fork", (workflow_id,), {"step_id": step_id}))


def test_canonical_management_operations_are_audited() -> None:
    client = FakeDBOSClient()
    management = ManagementClient(client, Diagnostics())
    assert management.list("owner:local") == ["wf"]
    management.get("wf-1", "owner:local")
    management.pause("wf-1", "owner:local")
    management.resume("wf-1", "owner:local")
    management.cancel("wf-1", "owner:local")
    management.restart("wf-1", "owner:local")
    management.fork_from_step("wf-1", 2, "owner:local")
    assert [audit.action for audit in management.audits] == [
        "list",
        "get",
        "pause",
        "resume",
        "cancel",
        "restart",
        "fork-from-step",
    ]


def test_management_rejects_unauthorized_actor() -> None:
    management = ManagementClient(FakeDBOSClient(), Diagnostics())
    with pytest.raises(PermissionError):
        management.pause("wf-1", "human:untrusted")
