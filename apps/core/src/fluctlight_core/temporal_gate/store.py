"""Gate-only intent/result persistence.

Temporal owns execution history. This store contains only the committed intent
and the one-result external-effect boundary needed to prove idempotency.
"""

from __future__ import annotations

import json
import threading
from dataclasses import replace
from datetime import UTC, datetime
from typing import Protocol

from .models import GateInput, GateResult, ProviderResult, WorkflowRecord


class GateStore(Protocol):
    def commit_intent(self, intent_id: str, workflow_id: str, request: GateInput) -> bool: ...

    def intent_exists(self, intent_id: str) -> bool: ...

    def get_intent(self, intent_id: str) -> GateInput | None: ...

    def get_workflow(self, workflow_id: str) -> WorkflowRecord | None: ...

    def put_workflow(self, record: WorkflowRecord) -> None: ...

    def put_provider_result(self, workflow_id: str, result: ProviderResult) -> None: ...

    def get_provider_result(self, workflow_id: str) -> ProviderResult | None: ...

    def put_result_once(self, workflow_id: str, result: GateResult) -> GateResult: ...

    def list_workflows(self) -> list[WorkflowRecord]: ...


class InMemoryGateStore:
    """Thread-safe fixture persistence with transaction-like critical sections."""

    def __init__(self) -> None:
        self._lock = threading.RLock()
        self.intents: dict[str, tuple[str, GateInput]] = {}
        self.workflows: dict[str, WorkflowRecord] = {}
        self.provider_results: dict[str, ProviderResult] = {}
        self.results: dict[str, GateResult] = {}

    def commit_intent(self, intent_id: str, workflow_id: str, request: GateInput) -> bool:
        with self._lock:
            if intent_id in self.intents:
                return False
            self.intents[intent_id] = (workflow_id, request)
            return True

    def intent_exists(self, intent_id: str) -> bool:
        with self._lock:
            return intent_id in self.intents

    def get_intent(self, intent_id: str) -> GateInput | None:
        with self._lock:
            value = self.intents.get(intent_id)
            return value[1] if value else None

    def get_workflow(self, workflow_id: str) -> WorkflowRecord | None:
        with self._lock:
            return self.workflows.get(workflow_id)

    def put_workflow(self, record: WorkflowRecord) -> None:
        with self._lock:
            self.workflows[record.workflow_id] = record

    def put_provider_result(self, workflow_id: str, result: ProviderResult) -> None:
        with self._lock:
            existing = self.provider_results.setdefault(workflow_id, result)
            current = self.workflows[workflow_id]
            self.workflows[workflow_id] = replace(
                current,
                provider_result=existing,
                checkpoint="provider_result",
            )

    def get_provider_result(self, workflow_id: str) -> ProviderResult | None:
        with self._lock:
            return self.provider_results.get(workflow_id)

    def put_result_once(self, workflow_id: str, result: GateResult) -> GateResult:
        with self._lock:
            existing = self.results.get(workflow_id)
            if existing is not None:
                return existing
            self.results[workflow_id] = result
            current = self.workflows[workflow_id]
            self.workflows[workflow_id] = replace(current, result=result, status=result.status)
            return result

    def list_workflows(self) -> list[WorkflowRecord]:
        with self._lock:
            return list(self.workflows.values())


class PostgresGateStore:
    """Small PostgreSQL boundary used by the Compose API process."""

    def __init__(self, database_url: str) -> None:
        self.database_url = database_url

    def _connect(self):
        try:
            import psycopg
        except ImportError as exc:  # pragma: no cover
            raise RuntimeError("psycopg is required for the Temporal gate store") from exc
        return psycopg.connect(self.database_url)

    def initialize(self) -> None:
        with self._connect() as connection:
            connection.execute(
                """
                CREATE TABLE IF NOT EXISTS gate_intents (
                    intent_id TEXT PRIMARY KEY,
                    workflow_id TEXT NOT NULL UNIQUE,
                    intent_json JSONB NOT NULL,
                    committed_at TIMESTAMPTZ NOT NULL
                );
                CREATE TABLE IF NOT EXISTS gate_results (
                    workflow_id TEXT PRIMARY KEY,
                    result_json JSONB NOT NULL,
                    committed_at TIMESTAMPTZ NOT NULL
                );
                """
            )

    def commit_intent(self, intent_id: str, workflow_id: str, request: GateInput) -> bool:
        with self._connect() as connection:
            cursor = connection.execute(
                """
                INSERT INTO gate_intents(intent_id, workflow_id, intent_json, committed_at)
                VALUES (%s, %s, %s, %s)
                ON CONFLICT (intent_id) DO NOTHING
                RETURNING intent_id
                """,
                (intent_id, workflow_id, json.dumps(request.as_dict()), datetime.now(UTC)),
            )
            return cursor.fetchone() is not None

    def intent_exists(self, intent_id: str) -> bool:
        with self._connect() as connection:
            return (
                connection.execute(
                    "SELECT 1 FROM gate_intents WHERE intent_id = %s", (intent_id,)
                ).fetchone()
                is not None
            )

    def get_intent(self, intent_id: str) -> GateInput | None:
        with self._connect() as connection:
            row = connection.execute(
                "SELECT intent_json FROM gate_intents WHERE intent_id = %s", (intent_id,)
            ).fetchone()
        if row is None:
            return None
        payload = row[0] if isinstance(row[0], dict) else json.loads(row[0])
        return GateInput.from_dict(payload)

    def get_workflow(self, workflow_id: str) -> WorkflowRecord | None:
        del workflow_id
        return None

    def put_workflow(self, record: WorkflowRecord) -> None:
        del record

    def put_provider_result(self, workflow_id: str, result: ProviderResult) -> None:
        del workflow_id, result

    def get_provider_result(self, workflow_id: str) -> ProviderResult | None:
        del workflow_id
        return None

    def put_result_once(self, workflow_id: str, result: GateResult) -> GateResult:
        with self._connect() as connection:
            cursor = connection.execute(
                """
                INSERT INTO gate_results(workflow_id, result_json, committed_at)
                VALUES (%s, %s, %s)
                ON CONFLICT (workflow_id) DO NOTHING
                RETURNING result_json
                """,
                (workflow_id, json.dumps(result.as_dict()), datetime.now(UTC)),
            )
            row = cursor.fetchone()
            if row is None:
                row = connection.execute(
                    "SELECT result_json FROM gate_results WHERE workflow_id = %s", (workflow_id,)
                ).fetchone()
        if row is None:
            return result
        payload = row[0] if isinstance(row[0], dict) else json.loads(row[0])
        return GateResult(
            intent_id=payload["intent_id"],
            workflow_id=payload["workflow_id"],
            provider_request_id=payload["provider_request_id"],
            result_id=payload["result_id"],
            status=payload["status"],
            output=payload.get("output"),
            recovery_count=payload.get("recovery_count", 0),
        )

    def list_workflows(self) -> list[WorkflowRecord]:
        return []
