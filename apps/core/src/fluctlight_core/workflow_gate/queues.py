"""Independent queue policies for the three DBOS logical queues."""

from __future__ import annotations

import threading
import time
from collections.abc import Iterator
from contextlib import contextmanager

from .models import QueuePolicy

QUEUE_POLICIES: dict[str, QueuePolicy] = {
    "interaction": QueuePolicy("interaction", concurrency=2, rate_limit_per_second=4.0),
    "lifecycle": QueuePolicy("lifecycle", concurrency=1, rate_limit_per_second=1.0),
    "media": QueuePolicy("media", concurrency=1, rate_limit_per_second=0.5),
}


class QueuePolicyError(ValueError):
    pass


class LocalQueueLimits:
    """Small deterministic policy harness used by contract tests."""

    def __init__(self, policies: dict[str, QueuePolicy] | None = None) -> None:
        self.policies = policies or QUEUE_POLICIES
        self._semaphores = {
            name: threading.BoundedSemaphore(policy.concurrency)
            for name, policy in self.policies.items()
        }
        self._last_dispatch: dict[str, float] = {}
        self._lock = threading.Lock()

    @contextmanager
    def acquire(self, queue: str) -> Iterator[None]:
        if queue not in self.policies:
            raise QueuePolicyError(f"unknown queue: {queue}")
        policy = self.policies[queue]
        semaphore = self._semaphores[queue]
        semaphore.acquire()
        try:
            with self._lock:
                previous = self._last_dispatch.get(queue)
                minimum_gap = 1.0 / policy.rate_limit_per_second
                if previous is not None:
                    elapsed = time.monotonic() - previous
                    if elapsed < minimum_gap:
                        time.sleep(minimum_gap - elapsed)
                self._last_dispatch[queue] = time.monotonic()
            yield
        finally:
            semaphore.release()


def build_dbos_queues():
    """Build official DBOS Queue objects without starting consumers."""

    try:
        from dbos import Queue
    except ImportError as exc:  # pragma: no cover - deployment diagnostic
        raise RuntimeError("DBOS is required to construct worker queues") from exc
    return {
        name: Queue(
            name,
            concurrency=policy.concurrency,
            limiter={
                "limit": max(1, int(policy.rate_limit_per_second)),
                "period": max(1.0, 1.0 / policy.rate_limit_per_second),
            },
        )
        for name, policy in QUEUE_POLICIES.items()
    }
