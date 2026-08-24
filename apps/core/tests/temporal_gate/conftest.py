from __future__ import annotations

import io

import pytest
from fluctlight_core.temporal_gate.diagnostics import Diagnostics
from fluctlight_core.temporal_gate.models import QUEUES, QueuePolicy
from fluctlight_core.temporal_gate.queues import LocalQueueLimits
from fluctlight_core.temporal_gate.runtime import TemporalGateRuntime


@pytest.fixture
def diagnostics() -> Diagnostics:
    return Diagnostics(io.StringIO())


@pytest.fixture
def runtime(diagnostics: Diagnostics) -> TemporalGateRuntime:
    # Keep unit tests independent from wall-clock rate-limit sleeps.
    policies = {
        queue: QueuePolicy(queue, concurrency=2, rate_limit_per_second=1_000.0) for queue in QUEUES
    }
    return TemporalGateRuntime(
        diagnostics=diagnostics,
        queue_limits=LocalQueueLimits(policies),
    )
