"""Minimal API process: commit intent, start and manage Temporal workflows."""

from __future__ import annotations

import asyncio
import os
from contextlib import asynccontextmanager
from typing import Any

from fastapi import FastAPI, Header, HTTPException, Query
from pydantic import BaseModel, Field

from .ids import intent_id, workflow_id
from .management import TemporalManagementClient
from .models import GateInput, RepairCommand
from .store import GateStore, PostgresGateStore
from .temporal_workflows import GateWorkflow


class IntentRequest(BaseModel):
    intent_key: str = Field(min_length=1, max_length=256)
    queue: str = "media"
    sleep_seconds: float = Field(default=0.0, ge=0.0)
    h3_duration_seconds: float = Field(default=0.0, ge=0.0, le=900.0)
    heartbeat_interval_seconds: float = Field(default=5.0, gt=0.0)
    timeout_seconds: float = Field(default=900.0, gt=0.0)
    decision_version: str = "gate-v1"
    continue_after: int = Field(default=0, ge=0)


class RepairRequest(BaseModel):
    reason: str = Field(min_length=1, max_length=512)
    expected_version: str | None = None


def _database_url() -> str:
    return os.environ.get(
        "GATE_DATABASE_URL",
        "postgresql://temporal:temporal@postgres:5432/temporal",
    )


def create_app(*, temporal_client: Any | None = None, store: GateStore | None = None) -> FastAPI:
    gate_store = store or PostgresGateStore(_database_url())
    client = temporal_client
    owns_client = temporal_client is None

    @asynccontextmanager
    async def lifespan(_: FastAPI):
        nonlocal client
        if hasattr(gate_store, "initialize"):
            await asyncio.to_thread(gate_store.initialize)
        if client is None:
            from temporalio.client import Client

            client = await Client.connect(
                os.environ.get("TEMPORAL_ADDRESS", "temporal:7233"),
                namespace=os.environ.get("TEMPORAL_NAMESPACE", "default"),
            )
        yield
        if owns_client and client is not None:
            close = getattr(client, "close", None)
            if close is not None:
                await close()

    app = FastAPI(title="Fluctlight Temporal Runtime Gate", version="0.1.0", lifespan=lifespan)

    def management() -> TemporalManagementClient:
        if client is None:
            raise HTTPException(status_code=503, detail="Temporal client is not ready")
        return TemporalManagementClient(client)

    @app.get("/healthz")
    def health() -> dict[str, str]:
        return {"status": "ok", "role": "api", "runtime": "temporal"}

    @app.get("/readyz")
    def ready() -> dict[str, str]:
        if client is None:
            raise HTTPException(status_code=503, detail="Temporal client is not ready")
        return {"status": "ready", "role": "api", "runtime": "temporal"}

    @app.post("/gate/intents", status_code=202)
    async def start_intent(payload: IntentRequest) -> dict[str, str]:
        try:
            request = GateInput(**payload.model_dump())
        except ValueError as exc:
            raise HTTPException(status_code=422, detail=str(exc)) from exc
        if client is None:
            raise HTTPException(status_code=503, detail="Temporal client is not ready")
        committed_intent = intent_id(request.intent_key)
        execution_id = workflow_id(committed_intent)
        committed = await asyncio.to_thread(
            gate_store.commit_intent,
            committed_intent,
            execution_id,
            request,
        )
        if committed:
            try:
                await client.start_workflow(
                    GateWorkflow.run,
                    request.as_dict(),
                    id=execution_id,
                    task_queue=request.queue,
                )
            except Exception as exc:
                # A duplicate stable ID reuses the existing Temporal execution.
                if (
                    "already started" not in str(exc).lower()
                    and "already exists" not in str(exc).lower()
                ):
                    raise HTTPException(
                        status_code=503, detail="Temporal workflow start failed"
                    ) from exc
        return {
            "intent_id": committed_intent,
            "intent_key": request.intent_key,
            "workflow_id": execution_id,
            "task_queue": request.queue,
        }

    @app.get("/gate/workflows")
    async def list_workflows(
        actor: str = Header(default="owner:local", alias="x-gate-actor"),
        query: str = Query(default=""),
    ) -> list[dict[str, Any]]:
        try:
            executions = await management().list(actor, query=query)
            return [
                dict(execution.to_json_dict())
                if hasattr(execution, "to_json_dict")
                else {"workflow": str(execution)}
                for execution in executions
            ]
        except PermissionError as exc:
            raise HTTPException(status_code=403, detail=str(exc)) from exc

    @app.get("/gate/workflows/{execution_id}/status")
    async def workflow_status(
        execution_id: str, actor: str = Header(default="owner:local", alias="x-gate-actor")
    ) -> Any:
        try:
            return await management().query(execution_id, actor)
        except PermissionError as exc:
            raise HTTPException(status_code=403, detail=str(exc)) from exc

    @app.post("/gate/workflows/{execution_id}/pause", status_code=202)
    async def pause(
        execution_id: str, actor: str = Header(default="owner:local", alias="x-gate-actor")
    ) -> dict[str, str]:
        try:
            await management().pause(execution_id, actor)
            return {"workflow_id": execution_id, "operation": "pause"}
        except PermissionError as exc:
            raise HTTPException(status_code=403, detail=str(exc)) from exc

    @app.post("/gate/workflows/{execution_id}/resume", status_code=202)
    async def resume(
        execution_id: str, actor: str = Header(default="owner:local", alias="x-gate-actor")
    ) -> dict[str, str]:
        try:
            await management().resume(execution_id, actor)
            return {"workflow_id": execution_id, "operation": "resume"}
        except PermissionError as exc:
            raise HTTPException(status_code=403, detail=str(exc)) from exc

    @app.post("/gate/workflows/{execution_id}/repair")
    async def repair(
        execution_id: str,
        payload: RepairRequest,
        actor: str = Header(default="owner:local", alias="x-gate-actor"),
    ) -> Any:
        try:
            return await management().repair(
                execution_id,
                RepairCommand(payload.reason, payload.expected_version),
                actor,
            )
        except PermissionError as exc:
            raise HTTPException(status_code=403, detail=str(exc)) from exc

    @app.post("/gate/workflows/{execution_id}/cancel", status_code=202)
    async def cancel(
        execution_id: str, actor: str = Header(default="owner:local", alias="x-gate-actor")
    ) -> dict[str, str]:
        try:
            await management().cancel(execution_id, actor)
            return {"workflow_id": execution_id, "operation": "cancel"}
        except PermissionError as exc:
            raise HTTPException(status_code=403, detail=str(exc)) from exc

    return app


app = create_app()


def main() -> None:
    import uvicorn

    uvicorn.run(
        app,
        host=os.environ.get("GATE_API_HOST", "0.0.0.0"),
        port=int(os.environ.get("GATE_API_PORT", "8080")),
        log_level=os.environ.get("GATE_LOG_LEVEL", "info").lower(),
    )


if __name__ == "__main__":
    main()
