from __future__ import annotations

from unittest.mock import patch

from dbos import DBOS
from fluctlight_core.workflow_gate.dbos_runtime import (
    DBOSGateClient,
    _queue_options,
    make_dbos_config,
)
from fluctlight_core.workflow_gate.models import GateInput


class FakeClient:
    def __init__(self, **kwargs) -> None:
        self.init_kwargs = kwargs
        self.registered: list[tuple[str, dict]] = []

    def register_queue(self, name: str, **kwargs):
        self.registered.append((name, kwargs))
        return name

    def destroy(self) -> None:
        return None


def test_api_client_uses_client_supported_queue_conflict_policy() -> None:
    with patch("dbos.DBOSClient", FakeClient):
        client = DBOSGateClient("postgresql://unused")

    assert {name for name, _ in client._queues.items()} == set(_queue_options())
    assert client._client.init_kwargs["application_name"] == "fluctlight-gate"
    assert all(
        options["on_conflict"] == "always_update"
        for _, options in client._client.registered
    )


def test_dbos_application_version_is_explicit(monkeypatch) -> None:
    monkeypatch.setenv("DBOS_APPLICATION_VERSION", "t01-test-v2")
    assert make_dbos_config()["application_version"] == "t01-test-v2"


def test_enqueue_uses_the_same_stable_ids_as_the_gate_runtime() -> None:
    class EnqueueClient(FakeClient):
        def enqueue(self, options, payload):
            self.enqueue_options = options
            self.enqueue_payload = payload
            return options

    with patch("dbos.DBOSClient", EnqueueClient):
        client = DBOSGateClient("postgresql://unused")

    client.enqueue(GateInput(intent_key="api-id-consistency"))
    assert client._client.enqueue_options["workflow_id"].startswith("wf_")
    assert client._client.enqueue_options["deduplication_id"].startswith("intent_")
    assert client._client.enqueue_options["workflow_timeout"] == 900.0
    assert client._client.enqueue_payload["intent_id"].startswith("intent_")
    assert client._client.enqueue_payload["workflow_id"].startswith("wf_")
    assert client._client.enqueue_payload["provider_request_id"].startswith("provider_")


def test_dbos_workflow_module_is_importable_with_preemptible_async_step() -> None:
    from fluctlight_core.workflow_gate.dbos_workflows import fake_h3_step, gate_workflow

    assert callable(fake_h3_step)
    assert callable(gate_workflow)


def test_dbos_worker_and_client_smoke_with_sqlite(tmp_path) -> None:
    """Exercise the real DBOS decorators and queue wiring without Docker."""

    from fluctlight_core.workflow_gate.dbos_workflows import gate_workflow

    del gate_workflow
    database_url = f"sqlite:///{tmp_path / 'dbos.sqlite'}"
    dbos = DBOS(
        config={
            "name": "fluctlight-gate",
            "system_database_url": database_url,
            "application_database_url": database_url,
            "enable_otlp": False,
        }
    )
    client = None
    try:
        dbos.launch()
        for name, options in _queue_options().items():
            dbos.register_queue(name, **options)
        client = DBOSGateClient(database_url)
        handle = client.enqueue(
            GateInput(intent_key="sqlite-smoke", timeout_seconds=2.0),
        )
        result = handle.get_result(polling_interval_sec=0.05)
        assert result["step"]["request_id"].startswith("provider_")
        assert result["persisted_result"]["status"] == "skipped"
    finally:
        if client is not None:
            client.close()
        dbos.destroy()
