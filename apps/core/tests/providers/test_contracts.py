from fluctlight_core.providers.contracts import ModelRole


def test_model_roles_are_explicit_and_complete() -> None:
    assert {role.value for role in ModelRole} == {
        "initialization",
        "cognitive_assessment",
        "action_realization",
        "reflection",
        "embedding",
        "media_prompt",
    }
