"""DBOS process wiring for the API and Worker entrypoints."""

from __future__ import annotations

import os
from dataclasses import asdict
from typing import Any

from . import dbos_workflows as _dbos_workflows
from .ids import provider_request_id, stable_id, workflow_id
from .models import GateInput


def database_url() -> str:
    return os.environ.get("DBOS_SYSTEM_DATABASE_URL", "postgresql://dbos:dbos@postgres:5432/dbos")


def make_dbos_config() -> dict[str, Any]:
    url = database_url()
    config: dict[str, Any] = {
        "name": os.environ.get("DBOS_APP_NAME", "fluctlight-gate"),
        "system_database_url": url,
        "application_database_url": os.environ.get("DBOS_APPLICATION_DATABASE_URL", url),
        "log_level": os.environ.get("DBOS_LOG_LEVEL", "INFO"),
        "enable_otlp": False,
    }
    application_version = os.environ.get("DBOS_APPLICATION_VERSION")
    if application_version:
        config["application_version"] = application_version
    return config


def launch_worker():
    try:
        from dbos import DBOS
    except ImportError as exc:  # pragma: no cover - deployment diagnostic
        raise RuntimeError("DBOS dependency is unavailable; cannot start the gate Worker") from exc
    dbos = DBOS(config=make_dbos_config())
    dbos.launch()
    for name, policy in _queue_options().items():
        dbos.register_queue(name, **policy)
    return dbos


class DBOSGateClient:
    """API-side enqueue client; it never registers a queue consumer."""

    def __init__(self, system_database_url: str | None = None) -> None:
        try:
            from dbos import DBOSClient
        except ImportError as exc:  # pragma: no cover - deployment diagnostic
            raise RuntimeError("DBOS dependency is unavailable; cannot start the gate API") from exc
        self._client = DBOSClient(
            system_database_url=system_database_url or database_url(),
            application_name=os.environ.get("DBOS_APP_NAME", "fluctlight-gate"),
        )
        self._queues = {
            name: self._client.register_queue(name, on_conflict="always_update", **options)
            for name, options in _queue_options().items()
        }

    def enqueue(self, request: GateInput):
        intent_id = stable_id("intent", request.intent_key)
        stable_workflow_id = workflow_id(intent_id)
        request_id = provider_request_id(intent_id)
        payload = asdict(request) | {
            "intent_id": intent_id,
            "workflow_id": stable_workflow_id,
            "provider_request_id": request_id,
        }
        from dbos import EnqueueOptions

        options: EnqueueOptions = {
            "workflow_name": "gate_workflow",
            "queue_name": request.queue,
            "workflow_id": stable_workflow_id,
            "deduplication_id": intent_id,
            "duplication_policy": "return-existing",
            "workflow_timeout": request.timeout_seconds,
        }
        return self._client.enqueue(options, payload)

    def close(self) -> None:
        destroy = getattr(self._client, "destroy", None)
        if callable(destroy):
            destroy()


def _queue_options() -> dict[str, dict[str, object]]:
    from .queues import QUEUE_POLICIES

    return {
        name: {
            "concurrency": policy.concurrency,
            "limiter": {
                "limit": max(1, int(policy.rate_limit_per_second)),
                "period": max(1.0, 1.0 / policy.rate_limit_per_second),
            },
        }
        for name, policy in QUEUE_POLICIES.items()
    }
