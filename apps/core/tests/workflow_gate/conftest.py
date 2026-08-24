from __future__ import annotations

import io

import pytest

from fluctlight_core.workflow_gate.diagnostics import Diagnostics
from fluctlight_core.workflow_gate.runtime import GateRuntime


@pytest.fixture
def diagnostics() -> Diagnostics:
    return Diagnostics(io.StringIO())


@pytest.fixture
def runtime(diagnostics: Diagnostics) -> GateRuntime:
    return GateRuntime(diagnostics=diagnostics)
