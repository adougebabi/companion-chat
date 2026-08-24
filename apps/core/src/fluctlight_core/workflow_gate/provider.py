"""Idempotent fake h3 provider used by the long-task gate."""

from __future__ import annotations

import threading

from .models import ProviderResult


class CooperativeCancellation(Exception):
    """Raised when a fake provider observes an authorized cancellation."""


class ProviderTimeout(TimeoutError):
    """Raised when a fake provider reaches its workflow deadline."""


class FakeH3Provider:
    """A fake external system with stable request-key idempotency."""

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
            result = ProviderResult(
                request_id=request_id,
                output=f"fake-h3-output:{request_id}",
                effect_count=1,
            )
            self._results[request_id] = result
            return result

    def heartbeat(self, request_id: str) -> None:
        with self._lock:
            self.heartbeat_count[request_id] = self.heartbeat_count.get(request_id, 0) + 1

    def lookup(self, request_id: str) -> ProviderResult | None:
        with self._lock:
            return self._results.get(request_id)
