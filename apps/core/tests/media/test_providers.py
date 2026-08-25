import pytest
from fluctlight_core.media.providers import ComfyUiPlugin, MediaProviderConfigurationError


def test_comfyui_plugin_replaces_exactly_one_configured_prompt_placeholder() -> None:
    plugin = ComfyUiPlugin.from_config(
        {
            "baseUrl": "http://comfyui:8188",
            "workflow": {
                "3": {
                    "class_type": "CLIPTextEncode",
                    "inputs": {"text": "{{prompt}}"},
                }
            },
        }
    )

    workflow = plugin._replace_prompt(plugin.workflow, "A frozen media prompt")
    assert workflow["3"]["inputs"]["text"] == "A frozen media prompt"
    assert plugin.workflow["3"]["inputs"]["text"] == "{{prompt}}"


@pytest.mark.parametrize(
    "workflow",
    [
        {"3": {"inputs": {"text": "no prompt placeholder"}}},
        {"3": {"inputs": {"first": "{{prompt}}", "second": "{{prompt}}"}}},
    ],
)
def test_comfyui_plugin_rejects_ambiguous_prompt_configuration(workflow: object) -> None:
    with pytest.raises(MediaProviderConfigurationError, match="exactly one"):
        ComfyUiPlugin.from_config({"baseUrl": "http://comfyui:8188", "workflow": workflow})
