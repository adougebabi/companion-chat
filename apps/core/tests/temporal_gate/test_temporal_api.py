from __future__ import annotations

import asyncio
from collections.abc import Awaitable
from typing import Any

import pytest
from fastapi import HTTPException
from fluctlight_core.temporal_gate.api_entrypoint import IntentRequest, create_app
from fluctlight_core.temporal_gate.store import InMemoryGateStore
from fluctlight_core.temporal_gate.temporal_workflows import (
    GateWorkflow,
    fake_h3_activity,
    persist_gate_result_activity,
)
from fluctlight_core.temporal_gate.worker_entrypoint import deployment_config


class FakeClient:
    def __init__(self) -> None:
        self.started: list[dict[str, object]] = []

    async def start_workflow(self, workflow, payload, *, id: str, task_queue: str) -> None:
        self.started.append(
            {"workflow": workflow, "payload": payload, "id": id, "task_queue": task_queue}
        )


def test_temporal_definitions_expose_run_signal_query_update_and_activity() -> None:
    definition = getattr(GateWorkflow, "__temporal_workflow_definition")

    assert definition.name == "GateWorkflow"
    assert definition.run_fn is GateWorkflow.run
    assert set(definition.signals) == {"pause", "resume"}
    assert set(definition.queries) == {"status"}
    assert set(definition.updates) == {"repair"}
    activity_definition = getattr(fake_h3_activity, "__temporal_activity_definition")
    assert activity_definition.name == "fake_h3"
    assert activity_definition.is_async is True
    result_definition = getattr(persist_gate_result_activity, "__temporal_activity_definition")
    assert result_definition.name == "persist_gate_result"


def test_worker_uses_official_deployment_versioning_config(monkeypatch) -> None:
    monkeypatch.setenv("TEMPORAL_WORKER_DEPLOYMENT_NAME", "gate")
    monkeypatch.setenv("TEMPORAL_WORKER_DEPLOYMENT_VERSION", "gate-v2")
    config = deployment_config()
    assert config.use_worker_versioning is True
    assert config.version.deployment_name == "gate"
    assert config.version.build_id == "gate-v2"


def test_workflow_handlers_are_callable_without_a_live_server() -> None:
    workflow = GateWorkflow()

    assert workflow.status()["status"] == "queued"
    run(workflow.pause())
    assert workflow.status()["paused"] is True
    run(workflow.resume())
    repair = run(workflow.repair({"reason": "owner repair", "version": "gate-v2"}))

    assert workflow.status()["paused"] is False
    assert repair == {
        "accepted": True,
        "reason": "owner repair",
        "acknowledged_version": "gate-v2",
    }


def test_api_routes_can_be_invoked_without_starting_temporal() -> None:
    temporal_client = FakeClient()
    app = create_app(temporal_client=temporal_client, store=InMemoryGateStore())
    endpoints: dict[str, Any] = {
        getattr(route, "path"): getattr(route, "endpoint")
        for route in app.routes
        if hasattr(route, "endpoint")
    }

    assert endpoints["/healthz"]() == {"status": "ok", "role": "api", "runtime": "temporal"}
    assert endpoints["/readyz"]() == {"status": "ready", "role": "api", "runtime": "temporal"}

    response = run(
        endpoints["/gate/intents"](IntentRequest(intent_key="api-contract", queue="interaction"))
    )
    assert response["workflow_id"] == temporal_client.started[0]["id"]
    assert response["task_queue"] == "interaction"
    assert temporal_client.started[0]["workflow"] is GateWorkflow.run

    with pytest.raises(HTTPException) as error:
        run(endpoints["/gate/intents"](IntentRequest(intent_key="bad-api-queue", queue="unknown")))
    assert error.value.status_code == 422


def run(awaitable: Awaitable[Any]) -> Any:
    async def wait() -> Any:
        return await awaitable

    return asyncio.run(wait())
