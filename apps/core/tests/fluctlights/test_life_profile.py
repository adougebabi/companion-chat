import pytest
from fluctlight_core.fluctlights.contracts import FoundationProvenance, LifeProfile


def test_life_profile_keeps_structured_long_lived_context() -> None:
    profile = LifeProfile(
        appearance={"hair": "short"},
        social_background={"family": "close"},
        preferences={"likes": ["photography"]},
        life_habits=({"description": "直播前发布预告"},),
        recurring_commitments=({"title": "周五直播", "local_start_time": "20:00"},),
        relationship_seeds=({"target": "owner", "role": "长期观众"},),
        character_constraints=({"description": "不编造直播数据"},),
    )

    assert profile.as_payload()["recurring_commitments"][0]["title"] == "周五直播"


def test_foundation_provenance_accepts_only_explicit_initialization_sources() -> None:
    provenance = FoundationProvenance(
        {"identity.occupation": "user_explicit", "life_profile.appearance": "model_generated"}
    )
    assert provenance.as_payload()["field_sources"]["identity.occupation"] == "user_explicit"
    with pytest.raises(ValueError):
        FoundationProvenance({"identity.notes": "unknown"})
