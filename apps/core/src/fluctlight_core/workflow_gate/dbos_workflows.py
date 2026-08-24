"""Official DBOS decorators used by the Compose worker."""

from __future__ import annotations

import asyncio
from dataclasses import asdict

try:
    from dbos import DBOS, SetWorkflowTimeout
except ImportError:  # pragma: no cover - local unit tests may omit DBOS
    DBOS = None  # type: ignore[assignment,misc]
    SetWorkflowTimeout = None  # type: ignore[assignment,misc]


def _workflow_decorator():
    return DBOS.workflow() if DBOS is not None else (lambda function: function)


def _step_decorator(**kwargs):
    return DBOS.step(**kwargs) if DBOS is not None else (lambda function: function)


@_step_decorator(retries_allowed=True, interval_seconds=1.0, max_attempts=3, preemptible=True)
async def fake_h3_step(request_id: str, duration_seconds: float = 0.0) -> dict[str, str | float]:
    """The production-shaped step; the deterministic provider is tested separately."""

    if duration_seconds > 0:
        await asyncio.sleep(duration_seconds)
    return {"request_id": request_id, "duration_seconds": duration_seconds, "status": "submitted"}


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
        )
    else:
        with SetWorkflowTimeout(timeout):
            result = await DBOS.run_step_async(
                None,
                fake_h3_step,
                str(payload["provider_request_id"]),
                float(payload.get("h3_duration_seconds", 0.0)),
            )
    return {
        "payload": asdict(payload) if hasattr(payload, "__dataclass_fields__") else payload,
        "step": result,
    }
