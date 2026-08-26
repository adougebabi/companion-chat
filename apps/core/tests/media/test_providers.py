import asyncio
import json

import httpx
import pytest
from fluctlight_core.media.contracts import MediaIntent, MediaKind
from fluctlight_core.media.providers import (
    ComfyUiPlugin,
    MediaProviderConfigurationError,
    MediaProviderRequestError,
)


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


def test_comfyui_plugin_submits_workflow_and_uses_comfyui_prompt_id() -> None:
    calls: list[tuple[str, str]] = []

    def handler(request: httpx.Request) -> httpx.Response:
        calls.append((request.method, request.url.path))
        if request.method == "POST" and request.url.path == "/prompt":
            body = json.loads(request.content)
            assert "prompt_id" not in body
            assert body["prompt"]["3"]["inputs"]["text"] == "frozen prompt"
            return httpx.Response(200, json={"prompt_id": "comfyui-1"})
        if request.method == "GET" and request.url.path == "/history/comfyui-1":
            return httpx.Response(
                200,
                json={
                    "comfyui-1": {
                        "outputs": {
                            "9": {
                                "images": [
                                    {"filename": "output.png", "subfolder": "", "type": "output"}
                                ]
                            }
                        }
                    }
                },
            )
        if request.method == "GET" and request.url.path == "/view":
            return httpx.Response(
                200, content=b"image-bytes", headers={"content-type": "image/png"}
            )
        if request.method == "POST" and request.url.path in {"/queue", "/interrupt"}:
            return httpx.Response(200, json={})
        return httpx.Response(404)

    plugin = ComfyUiPlugin(
        "http://comfyui:8188",
        {"3": {"inputs": {"text": "{{prompt}}"}}},
        transport=httpx.MockTransport(handler),
    )
    intent = MediaIntent(
        "intent-1",
        "fl-1",
        MediaKind.IMAGE,
        "image/png",
        "frozen prompt",
        "provider-1",
        "workflow-1",
    )

    assert asyncio.run(plugin.submit(intent)) == "comfyui-1"
    outcome = asyncio.run(plugin.poll("comfyui-1"))
    assert outcome is not None and outcome["status"] == "completed"
    downloaded = asyncio.run(plugin.download(outcome["output"]))
    asyncio.run(plugin.cancel("provider-1"))

    assert downloaded.content == b"image-bytes"
    assert downloaded.content_type == "image/png"
    assert calls == [
        ("POST", "/prompt"),
        ("GET", "/history/comfyui-1"),
        ("GET", "/view"),
        ("POST", "/queue"),
        ("POST", "/interrupt"),
    ]


def test_comfyui_plugin_exposes_a_bounded_validation_error() -> None:
    plugin = ComfyUiPlugin(
        "http://comfyui:8188",
        {"3": {"inputs": {"text": "{{prompt}}"}}},
        transport=httpx.MockTransport(
            lambda request: httpx.Response(400, text="invalid node input: resolution")
        ),
    )
    intent = MediaIntent(
        "intent-invalid",
        "fl-1",
        MediaKind.IMAGE,
        "image/png",
        "prompt",
        "provider-invalid",
        "workflow-invalid",
    )

    with pytest.raises(MediaProviderRequestError, match="HTTP 400") as error:
        asyncio.run(plugin.submit(intent))

    assert error.value.status_code == 400
    assert error.value.detail == "invalid node input: resolution"
