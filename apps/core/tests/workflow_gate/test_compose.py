from __future__ import annotations

import subprocess
from pathlib import Path

import pytest


@pytest.mark.compose
def test_compose_gate_has_postgres_api_worker_and_three_queues() -> None:
    compose = Path(__file__).parents[4] / "infra" / "compose" / "dbos-gate.compose.yml"
    text = compose.read_text()
    result = subprocess.run(
        ["docker", "compose", "-f", str(compose), "ps", "--services", "--status", "running"],
        capture_output=True,
        text=True,
        timeout=10,
        check=False,
    )
    diagnostics = f"{result.stdout}\n{result.stderr}".lower()
    if result.returncode != 0 and ("daemon" in diagnostics or "connect" in diagnostics):
        pytest.skip("Docker daemon is unavailable")
    assert result.returncode == 0, result.stderr
    assert set(result.stdout.splitlines()) >= {"postgres", "api", "worker"}
    for queue in ("interaction", "lifecycle", "media"):
        assert queue in text
