from __future__ import annotations

import asyncio

import pytest
from fluctlight_core.diagnostics.contracts import DiagnosticEvent, DiagnosticModelRun
from fluctlight_core.providers.contracts import ModelRole
from fluctlight_core.providers.runtime import (
    ConfiguredProviderRuntime,
    InitializationAnalysisError,
)
from fluctlight_core.providers.service import ProviderEndpoint, RoleAssignment


class _DiagnosticsRecorder:
    def __init__(self) -> None:
        self.events: list[DiagnosticEvent] = []
        self.runs: list[DiagnosticModelRun] = []

    async def emit_event(self, event: DiagnosticEvent) -> str:
        self.events.append(event)
        return "event-1"

    async def emit_model_run(self, run: DiagnosticModelRun) -> str:
        self.runs.append(run)
        return "run-1"


class _InvalidJsonAdapter:
    async def complete_structured(self, *_args: object, **_kwargs: object) -> dict[str, object]:
        raise RuntimeError("structured Provider completion was not valid JSON")


def _runtime(diagnostics: _DiagnosticsRecorder) -> ConfiguredProviderRuntime:
    runtime = ConfiguredProviderRuntime.__new__(ConfiguredProviderRuntime)
    runtime._diagnostics = diagnostics  # type: ignore[assignment]
    runtime._adapter = _InvalidJsonAdapter()  # type: ignore[assignment]
    return runtime


def test_initialization_invalid_json_is_recorded_as_a_failed_model_run() -> None:
    async def verify() -> None:
        diagnostics = _DiagnosticsRecorder()
        runtime = _runtime(diagnostics)

        async def resolve(
            _role: ModelRole,
        ) -> tuple[RoleAssignment, ProviderEndpoint, None]:
            return (
                RoleAssignment(ModelRole.INITIALIZATION, "endpoint-1", "model-1", 100, 10),
                ProviderEndpoint(
                    "endpoint-1",
                    "openai-compatible",
                    "http://provider",
                    "provider:key",
                ),
                None,
            )

        runtime._resolve = resolve  # type: ignore[assignment]
        with pytest.raises(InitializationAnalysisError) as raised:
            await runtime.analyze_initialization("一个想要认识世界的人")

        assert raised.value.code == "initialization_response_invalid_json"
        assert raised.value.status_code == 503
        assert len(diagnostics.runs) == 1
        run = diagnostics.runs[0]
        assert run.status == "failed"
        assert run.error_code == "initialization_response_invalid_json"
        assert "behavioral_policy" in str(run.prompt["messages"])
        assert "tone/voice/cadence" in str(run.prompt["messages"])

    asyncio.run(verify())


def test_initialization_role_unconfigured_is_recorded_as_a_diagnostic_event() -> None:
    async def verify() -> None:
        diagnostics = _DiagnosticsRecorder()
        runtime = _runtime(diagnostics)

        async def resolve(
            _role: ModelRole,
        ) -> tuple[RoleAssignment, ProviderEndpoint, None]:
            raise RuntimeError("Provider role initialization is not configured")

        runtime._resolve = resolve  # type: ignore[assignment]
        with pytest.raises(InitializationAnalysisError) as raised:
            await runtime.analyze_initialization("一个想要认识世界的人")

        assert raised.value.code == "initialization_role_unconfigured"
        assert raised.value.status_code == 422
        assert str(raised.value.details["correlation_id"]).startswith("initialization:")
        assert len(diagnostics.events) == 1
        assert diagnostics.events[0].payload["error_code"] == "initialization_role_unconfigured"

    asyncio.run(verify())
