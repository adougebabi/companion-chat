import asyncio
from typing import cast

import pytest
from fluctlight_core.fluctlights.creation import CreationError, CreationLifecycleService
from fluctlight_core.fluctlights.service import FluctlightService


class _InvalidTimezoneAnalyzer:
    async def analyze_initialization(self, _description: str) -> dict[str, object]:
        return {
            "foundation": {
                "identity": {"timezone": "UTC+9"},
                "personality": {},
                "behavioral_policy": {},
            }
        }


def test_creation_analysis_rejects_non_iana_timezones() -> None:
    async def verify() -> None:
        service = CreationLifecycleService(
            cast(FluctlightService, object()),
            _InvalidTimezoneAnalyzer(),
        )
        with pytest.raises(CreationError) as raised:
            await service.analyze_description("测试描述")
        assert raised.value.code == "initialization_foundation_invalid"

    asyncio.run(verify())
