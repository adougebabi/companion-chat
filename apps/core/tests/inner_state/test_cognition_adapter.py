import asyncio
from dataclasses import dataclass
from typing import Any, cast

from fluctlight_core.inner_state.service import CognitionStateApplier


@dataclass
class Snapshot:
    revision: int


@dataclass
class Current:
    revision: int


@dataclass
class Transition:
    current: Current


class FakeInnerStateService:
    def __init__(self) -> None:
        self.calls: list[tuple[str, object]] = []

    async def read(self, fluctlight_id: str, *, tx: object) -> Snapshot:
        self.calls.append(("read", tx))
        return Snapshot(revision=7)

    async def apply_assessment(
        self,
        fluctlight_id: str,
        assessment: object,
        *,
        expected_revision: int,
        tx: object,
    ) -> Transition:
        self.calls.append(("apply", (expected_revision, tx)))
        return Transition(current=Current(revision=8))


def test_cognition_state_applier_uses_persisted_revision_and_returns_transition_revision() -> None:
    service = FakeInnerStateService()
    transaction = object()
    revision = asyncio.run(
        CognitionStateApplier(cast(Any, service)).apply_assessment(
            "fluctlight-1", cast(Any, object()), tx=cast(Any, transaction)
        )
    )

    assert revision == 8
    assert service.calls == [("read", transaction), ("apply", (7, transaction))]
