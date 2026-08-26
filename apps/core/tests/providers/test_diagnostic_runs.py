import asyncio
from typing import Any

from fluctlight_core.providers.contracts import ModelRole
from fluctlight_core.providers.runtime import ConfiguredProviderRuntime
from fluctlight_core.providers.service import ProviderEndpoint, RoleAssignment


class Recorder:
    def __init__(self) -> None:
        self.runs: list[Any] = []

    async def emit_model_run(self, run) -> str:
        self.runs.append(run)
        return "run-1"


def test_provider_runtime_records_redacted_model_run_through_diagnostics_port() -> None:
    async def verify() -> None:
        runtime = ConfiguredProviderRuntime.__new__(ConfiguredProviderRuntime)
        recorder = Recorder()
        setattr(runtime, "_diagnostics", recorder)
        await runtime._record_model_run(
            assignment=RoleAssignment(ModelRole.INITIALIZATION, "endpoint-1", "model-1", 100, 10),
            endpoint=ProviderEndpoint("endpoint-1", "openai-compatible", "https://provider", "key"),
            prompt={"messages": [{"content": "draft", "api_key": "secret"}]},
            response={"foundation": {}, "token": "secret"},
            correlation_id="creation-1",
        )
        assert len(recorder.runs) == 1
        run = recorder.runs[0]
        assert run.role == "initialization"
        assert run.prompt["messages"][0]["api_key"] == "[REDACTED]"
        assert run.response["token"] == "[REDACTED]"

    asyncio.run(verify())
