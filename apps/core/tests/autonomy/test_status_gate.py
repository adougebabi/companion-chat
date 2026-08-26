import asyncio
from typing import cast

from fluctlight_core.autonomy.service import AutonomyService
from fluctlight_core.platform.persistence import UnitOfWorkFactory


async def _status(value: str) -> str:
    return value


def test_fluctlight_status_gate_allows_only_active_autonomy() -> None:
    async def verify() -> None:
        active_service = AutonomyService(
            cast(UnitOfWorkFactory, None), status_resolver=lambda _: _status("active")
        )
        paused_service = AutonomyService(
            cast(UnitOfWorkFactory, None), status_resolver=lambda _: _status("paused")
        )
        retired_service = AutonomyService(
            cast(UnitOfWorkFactory, None), status_resolver=lambda _: _status("retired")
        )
        assert await active_service._activity_allowed("fluctlight-1") == (True, "active")
        assert await paused_service._activity_allowed("fluctlight-1") == (
            False,
            "fluctlight_paused",
        )
        assert await retired_service._activity_allowed("fluctlight-1") == (
            False,
            "fluctlight_retired",
        )

    asyncio.run(verify())
