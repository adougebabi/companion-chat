"""Official DBOS decorators used by the Compose worker."""

from __future__ import annotations

import asyncio
import json
import os
from dataclasses import asdict

try:
    from dbos import DBOS, SetWorkflowTimeout
except ImportError:  # pragma: no cover - local unit tests may omit DBOS
    DBOS = None  # type: ignore[assignment,misc]
    SetWorkflowTimeout = None  # type: ignore[assignment,misc]

from .models import GateResult, WorkflowStatus
from .ids import stable_id
from .store import PostgresGateStore


def _workflow_decorator():
    return DBOS.workflow() if DBOS is not None else (lambda function: function)


def _step_decorator(**kwargs):
    return DBOS.step(**kwargs) if DBOS is not None else (lambda function: function)


@_step_decorator(retries_allowed=True, interval_seconds=1.0, max_attempts=3, preemptible=True)
async def fake_h3_step(
    request_id: str,
    duration_seconds: float = 0.0,
    heartbeat_interval_seconds: float = 5.0,
    workflow_id: str = "unknown",
) -> dict[str, str | float]:
    """The production-shaped step; the deterministic provider is tested separately."""

    elapsed = 0.0
    interval = max(heartbeat_interval_seconds, 0.05)
    while elapsed < duration_seconds:
        await asyncio.sleep(min(interval, duration_seconds - elapsed))
        elapsed += min(interval, duration_seconds - elapsed)
        print(
            json.dumps(
                {
                    "event": "provider_heartbeat",
                    "workflow_id": workflow_id,
                    "provider_request_id": request_id,
                    "elapsed_seconds": elapsed,
                },
                sort_keys=True,
            ),
            flush=True,
        )
    return {"request_id": request_id, "duration_seconds": duration_seconds, "status": "submitted"}


@_step_decorator(retries_allowed=True, interval_seconds=1.0, max_attempts=3)
def persist_gate_result(
    intent_id: str,
    workflow_id: str,
    provider_request_id: str,
    output: str,
) -> dict[str, object]:
    """Persist the final fixture result once, outside DBOS history by ID."""

    database_url = os.environ.get("DBOS_APPLICATION_DATABASE_URL", "")
    if not database_url.startswith(("postgres://", "postgresql://")):
        return {"status": "skipped", "result_id": stable_id("result", workflow_id)}
    result = GateResult(
        intent_id=intent_id,
        workflow_id=workflow_id,
        provider_request_id=provider_request_id,
        result_id=stable_id("result", workflow_id),
        status=WorkflowStatus.SUCCEEDED,
        output=output,
    )
    persisted = PostgresGateStore(database_url).put_result_once(workflow_id, result)
    return asdict(persisted)


@_workflow_decorator()
async def gate_workflow(payload: dict[str, object]) -> dict[str, object]:
    """Minimal DBOS workflow proving durable sleep + durable external step."""

    if DBOS is None:  # pragma: no cover - only a helpful direct-call fallback
        return payload
    sleep_seconds = float(payload.get("sleep_seconds", 0.0))
    if sleep_seconds > 0:
        await DBOS.sleep_async(sleep_seconds)
    timeout = float(payload.get("timeout_seconds", 900.0))
    if SetWorkflowTimeout is None:  # pragma: no cover
        result = await DBOS.run_step_async(
            None,
            fake_h3_step,
            str(payload["provider_request_id"]),
            float(payload.get("h3_duration_seconds", 0.0)),
            float(payload.get("heartbeat_interval_seconds", 5.0)),
            str(payload.get("workflow_id", "unknown")),
        )
    else:
        with SetWorkflowTimeout(timeout):
            result = await DBOS.run_step_async(
                None,
                fake_h3_step,
                str(payload["provider_request_id"]),
                float(payload.get("h3_duration_seconds", 0.0)),
                float(payload.get("heartbeat_interval_seconds", 5.0)),
                str(payload.get("workflow_id", "unknown")),
            )
    persisted_result = await DBOS.run_step_async(
        None,
        persist_gate_result,
        str(payload.get("intent_id", payload.get("intent_key", "unknown"))),
        str(payload.get("workflow_id", "unknown")),
        str(payload["provider_request_id"]),
        str(result["status"]),
    )
    return {
        "payload": asdict(payload) if hasattr(payload, "__dataclass_fields__") else payload,
        "step": result,
        "persisted_result": persisted_result,
    }
