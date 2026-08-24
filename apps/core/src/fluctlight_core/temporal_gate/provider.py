"""Idempotent fake h3 provider for heartbeat and recovery tests."""

from __future__ import annotations

import threading

from .models import ProviderResult


class CooperativeCancellation(Exception):
    """The provider observed Temporal's cooperative cancellation request."""


class ProviderTimeout(TimeoutError):
    """The fake provider exceeded the activity deadline."""


class FakeH3Provider:
    """A fake external system keyed by the stable Provider request ID."""

    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._results: dict[str, ProviderResult] = {}
        self.submit_count: dict[str, int] = {}
        self.heartbeat_count: dict[str, int] = {}

    def submit(self, request_id: str) -> ProviderResult:
        with self._lock:
            self.submit_count[request_id] = self.submit_count.get(request_id, 0) + 1
            existing = self._results.get(request_id)
            if existing is not None:
                return existing
            result = ProviderResult(request_id, f"fake-h3-output:{request_id}")
            self._results[request_id] = result
            return result

    def heartbeat(self, request_id: str) -> None:
        with self._lock:
            self.heartbeat_count[request_id] = self.heartbeat_count.get(request_id, 0) + 1

    def lookup(self, request_id: str) -> ProviderResult | None:
        with self._lock:
            return self._results.get(request_id)


_WORKER_PROVIDER = FakeH3Provider()


def worker_provider() -> FakeH3Provider:
    return _WORKER_PROVIDER
