import asyncio
from typing import Any, cast

from fluctlight_core.fluctlights.contracts import BehavioralPolicy, InitializationMode, Personality
from fluctlight_core.fluctlights.creation import CreationLifecycleService
from fluctlight_core.fluctlights.service import FluctlightService


class _Analyzer:
    async def analyze_initialization(self, _description: str) -> dict[str, object]:
        return {
            "foundation": {
                "identity": {"name": "测试"},
                "personality": Personality(openness=0.8).as_payload(),
                "behavioral_policy": BehavioralPolicy(directness=0.7).as_payload(),
            }
        }


class _Fluctlights:
    def __init__(self) -> None:
        self.created = None

    async def get(self, _fluctlight_id: str):
        from fluctlight_core.fluctlights.service import FluctlightNotFoundError

        raise FluctlightNotFoundError(_fluctlight_id)

    async def create(self, command):
        self.created = command
        # This is the same persistence serialization that previously crashed.
        command.personality.as_payload()
        return type(
            "Snapshot",
            (),
            {
                "id": command.id,
                "initialization_mode": command.initialization_mode,
                "identity": command.identity,
                "personality": command.personality,
                "behavioral_policy": command.behavioral_policy,
            },
        )()


def test_creation_preview_json_round_trips_personality_update_policy_for_activation() -> None:
    async def verify() -> None:
        fluctlights = _Fluctlights()
        service = CreationLifecycleService(
            cast(FluctlightService, fluctlights), cast(Any, _Analyzer())
        )
        preview = await service.analyze_description("测试描述")

        created = await service.activate(
            actor_id="human-owner",
            request_id="request-1",
            initialization_mode=InitializationMode.LLM_DEFINED,
            identity=preview.identity,
            personality=preview.personality,
            behavioral_policy=preview.behavioral_policy,
        )

        assert created.id.startswith("fluctlight_")
        assert fluctlights.created is not None
        assert fluctlights.created.personality.update_policy.max_delta == 0.05

    asyncio.run(verify())
