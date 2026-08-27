import asyncio
from typing import Any, cast

import pytest
from fluctlight_core.fluctlights.contracts import BehavioralPolicy, InitializationMode, Personality
from fluctlight_core.fluctlights.creation import CreationError, CreationLifecycleService
from fluctlight_core.fluctlights.service import FluctlightService


class _Analyzer:
    async def analyze_initialization(self, _description: str) -> dict[str, object]:
        personality = Personality(openness=0.8).as_payload()
        personality.pop("update_policy")
        life_profile = {
            "appearance": {"description": "短发，常穿宽松卫衣"},
            "social_background": {"summary": "独立生活的摄影爱好者"},
            "preferences": {"interests": ["街头摄影"]},
            "life_habits": [{"description": "每天记录一个画面"}],
            "recurring_commitments": [{"title": "周末摄影练习"}],
            "relationship_seeds": [{"target": "owner", "role": "熟人"}],
            "character_constraints": [{"description": "重视真实记录"}],
        }
        sources = {
            **{
                f"identity.{name}": "model_generated"
                for name in (
                    "name",
                    "age",
                    "gender",
                    "occupation",
                    "residence",
                    "timezone",
                    "birthday",
                    "background",
                    "biography",
                    "core_values",
                    "worldview",
                    "notes",
                )
            },
            **{f"personality.{name}": "model_generated" for name in personality},
            **{
                f"behavioral_policy.{name}": "model_generated"
                for name in BehavioralPolicy().as_payload()
            },
            **{f"life_profile.{name}": "model_generated" for name in life_profile},
        }
        sources["identity.name"] = "user_explicit"
        return {
            "foundation": {
                "identity": {"name": "测试", "timezone": "UTC+8"},
                "personality": personality,
                "behavioral_policy": BehavioralPolicy(directness=0.7).as_payload(),
                "life_profile": life_profile,
                "provenance": {"field_sources": sources},
                "initial_goals": [
                    {"description": "完成一组街头摄影练习", "importance": 0.8, "urgency": 0.4}
                ],
                "initial_intentions": [
                    {
                        "goal_index": 0,
                        "action": "整理今天想记录的街头画面",
                        "confidence": 0.7,
                        "expiration_hours": 24,
                    }
                ],
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
            life_profile=preview.life_profile,
            foundation_provenance=preview.foundation_provenance,
            initial_goals=preview.initial_goals,
            initial_intentions=preview.initial_intentions,
        )

        assert created.id.startswith("fluctlight_")
        assert fluctlights.created is not None
        assert fluctlights.created.personality.update_policy.max_delta == 0.05
        assert fluctlights.created.identity.timezone == "Asia/Shanghai"
        assert preview.initial_goals[0]["description"] == "完成一组街头摄影练习"
        assert preview.life_profile["life_habits"][0]["description"] == "每天记录一个画面"
        assert preview.foundation_provenance["field_sources"]["identity.name"] == "user_explicit"

    asyncio.run(verify())


def test_creation_analysis_rejects_an_incomplete_model_expression_profile() -> None:
    class IncompleteAnalyzer:
        async def analyze_initialization(self, _description: str) -> dict[str, object]:
            personality = Personality().as_payload()
            personality.pop("update_policy")
            return {
                "foundation": {
                    "identity": {"name": "测试", "notes": "语气温和而克制"},
                    "personality": personality,
                    "behavioral_policy": {"response_style": "简洁"},
                    "life_profile": {
                        "appearance": {},
                        "social_background": {},
                        "preferences": {},
                        "life_habits": [],
                        "recurring_commitments": [],
                        "relationship_seeds": [],
                        "character_constraints": [],
                    },
                    "provenance": {"field_sources": {}},
                    "initial_goals": [
                        {"description": "完成摄影练习", "importance": 0.8, "urgency": 0.4}
                    ],
                    "initial_intentions": [
                        {
                            "goal_index": 0,
                            "action": "整理画面灵感",
                            "confidence": 0.7,
                            "expiration_hours": 24,
                        }
                    ],
                }
            }

    async def verify() -> None:
        service = CreationLifecycleService(
            cast(FluctlightService, _Fluctlights()), cast(Any, IncompleteAnalyzer())
        )
        with pytest.raises(CreationError) as raised:
            await service.analyze_description("语气温和而克制，平时简短表达")
        assert raised.value.code == "initialization_foundation_invalid"

    asyncio.run(verify())
