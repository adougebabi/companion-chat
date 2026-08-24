"""Minimal API process for the DBOS gate.

This process owns intent validation and workflow enqueueing. It never registers
DBOS queue listeners, so only the Worker consumes background work.
"""

from __future__ import annotations

import asyncio
import os
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field

from .dbos_runtime import DBOSGateClient, database_url
from .ids import stable_id, workflow_id
from .models import GateInput
from .store import PostgresGateStore


class IntentRequest(BaseModel):
    intent_key: str = Field(min_length=1, max_length=256)
    queue: str = "media"
    sleep_seconds: float = Field(default=0.0, ge=0.0)
    h3_duration_seconds: float = Field(default=0.0, ge=0.0, le=900.0)
    heartbeat_interval_seconds: float = Field(default=5.0, gt=0.0)
    timeout_seconds: float = Field(default=900.0, gt=0.0)
    decision_version: str = "gate-v1"


def create_app() -> FastAPI:
    client: DBOSGateClient | None = None
    store = PostgresGateStore(database_url())

    @asynccontextmanager
    async def lifespan(_: FastAPI):
        nonlocal client
        await asyncio.to_thread(store.initialize)
        client = await asyncio.to_thread(DBOSGateClient)
        yield
        if client is not None:
            client.close()

    app = FastAPI(title="Fluctlight DBOS Runtime Gate", version="0.1.0", lifespan=lifespan)

    @app.get("/healthz")
    def health() -> dict[str, str]:
        return {"status": "ok", "role": "api"}

    @app.get("/readyz")
    def ready() -> dict[str, str]:
        return {"status": "ready", "role": "api"}

    @app.post("/gate/intents", status_code=202)
    def start_intent(payload: IntentRequest) -> dict[str, str]:
        if payload.queue not in {"interaction", "lifecycle", "media"}:
            raise HTTPException(
                status_code=422,
                detail="queue must be interaction, lifecycle, or media",
            )
        request = GateInput(**payload.model_dump())
        try:
            if client is None:
                raise RuntimeError("API lifespan has not initialized DBOS client")
            intent_id = stable_id("intent", request.intent_key)
            store.commit_intent(intent_id, workflow_id(intent_id), request)
            handle = client.enqueue(request)
            return {
                "intent_key": request.intent_key,
                "workflow_id": str(getattr(handle, "workflow_id", "")),
            }
        except Exception as exc:
            raise HTTPException(status_code=503, detail="DBOS workflow enqueue failed") from exc

    return app


app = create_app()


def main() -> None:
    import uvicorn

    uvicorn.run(
        app,
        host=os.environ.get("GATE_API_HOST", "0.0.0.0"),
        port=int(os.environ.get("GATE_API_PORT", "8080")),
        log_level=os.environ.get("DBOS_LOG_LEVEL", "info").lower(),
    )


if __name__ == "__main__":
    main()
