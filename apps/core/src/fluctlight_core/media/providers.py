"""Pluggable MediaProvider adapters; this module currently registers ComfyUI only."""

from __future__ import annotations

import copy
from collections.abc import Mapping
from dataclasses import dataclass
from typing import Any, Protocol, cast

import httpx

from .contracts import MediaIntent, MediaProvider


class MediaProviderConfigurationError(ValueError):
    pass


@dataclass(frozen=True, slots=True)
class DownloadedMedia:
    content: bytes
    content_type: str


class DownloadableMediaProvider(MediaProvider, Protocol):
    async def download(self, output: Mapping[str, Any]) -> DownloadedMedia: ...


class MediaProviderRegistry:
    """Resolve configured provider plugins without coupling MediaService to transport."""

    def __init__(self, providers: Mapping[str, type[Any]]) -> None:
        self._providers = dict(providers)

    def from_config(self, provider_id: str, config: object) -> DownloadableMediaProvider:
        provider = self._providers.get(provider_id)
        if provider is None:
            raise MediaProviderConfigurationError(f"media provider {provider_id} is unavailable")
        from_config = getattr(provider, "from_config", None)
        if from_config is None:
            raise MediaProviderConfigurationError(
                f"media provider {provider_id} has no config factory"
            )
        return cast(DownloadableMediaProvider, from_config(config))


@dataclass(slots=True)
class ComfyUiPlugin:
    """ComfyUI API-format workflow plugin with one explicit prompt placeholder."""

    base_url: str
    workflow: Mapping[str, Any]

    @classmethod
    def from_config(cls, value: object) -> ComfyUiPlugin:
        if not isinstance(value, Mapping):
            raise MediaProviderConfigurationError("media.comfyui must be an object")
        base_url = value.get("baseUrl")
        workflow = value.get("workflow")
        if not isinstance(base_url, str) or not base_url.startswith(("http://", "https://")):
            raise MediaProviderConfigurationError("media.comfyui.baseUrl must be an HTTP URL")
        if not isinstance(workflow, Mapping) or not workflow:
            raise MediaProviderConfigurationError(
                "media.comfyui.workflow must be a non-empty object"
            )
        cls._replace_prompt(workflow, "__validation__")
        return cls(base_url.rstrip("/"), copy.deepcopy(dict(workflow)))

    async def submit(self, intent: MediaIntent) -> str:
        workflow = self._replace_prompt(self.workflow, intent.prompt)
        async with httpx.AsyncClient(timeout=30.0) as client:
            response = await client.post(
                f"{self.base_url}/prompt",
                json={"prompt": workflow, "prompt_id": intent.provider_request_id},
            )
            response.raise_for_status()
            payload = response.json()
        prompt_id = payload.get("prompt_id") if isinstance(payload, dict) else None
        if prompt_id != intent.provider_request_id:
            raise RuntimeError("ComfyUI did not accept the stable provider request ID")
        return prompt_id

    async def poll(self, provider_request_id: str) -> Mapping[str, Any] | None:
        async with httpx.AsyncClient(timeout=30.0) as client:
            response = await client.get(f"{self.base_url}/history/{provider_request_id}")
            if response.status_code == 404:
                return None
            response.raise_for_status()
            payload = response.json()
        history = payload.get(provider_request_id) if isinstance(payload, dict) else None
        if not isinstance(history, Mapping):
            return None
        status = history.get("status")
        status_text = status.get("status_str") if isinstance(status, Mapping) else None
        if status_text in {"error", "failed", "cancelled"}:
            return {"status": "failed", "error": str(status_text)}
        output = self._first_output(history.get("outputs"))
        if output is None:
            return {"status": "pending"}
        return {"status": "completed", "output": output}

    async def cancel(self, provider_request_id: str) -> None:
        async with httpx.AsyncClient(timeout=30.0) as client:
            queue = await client.post(
                f"{self.base_url}/queue", json={"delete": [provider_request_id]}
            )
            queue.raise_for_status()
            interrupt = await client.post(f"{self.base_url}/interrupt")
            interrupt.raise_for_status()

    async def download(self, output: Mapping[str, Any]) -> DownloadedMedia:
        filename = output.get("filename")
        if not isinstance(filename, str) or not filename:
            raise RuntimeError("ComfyUI output has no filename")
        params = {
            "filename": filename,
            "subfolder": str(output.get("subfolder", "")),
            "type": str(output.get("type", "output")),
        }
        async with httpx.AsyncClient(timeout=60.0, follow_redirects=True) as client:
            response = await client.get(f"{self.base_url}/view", params=params)
            response.raise_for_status()
        content_type = response.headers.get("content-type", "application/octet-stream").split(
            ";", 1
        )[0]
        if not response.content:
            raise RuntimeError("ComfyUI output was empty")
        return DownloadedMedia(response.content, content_type)

    @staticmethod
    def _replace_prompt(workflow: Mapping[str, Any], prompt: str) -> dict[str, Any]:
        copied = copy.deepcopy(dict(workflow))
        matches: list[tuple[dict[str, Any], str]] = []

        def visit(value: object) -> None:
            if isinstance(value, dict):
                for key, child in value.items():
                    if child == "{{prompt}}":
                        matches.append((value, key))
                    else:
                        visit(child)
            elif isinstance(value, list):
                for child in value:
                    visit(child)

        visit(copied)
        if len(matches) != 1:
            raise MediaProviderConfigurationError(
                "ComfyUI workflow must contain exactly one {{prompt}} placeholder"
            )
        container, key = matches[0]
        container[key] = prompt
        return copied

    @staticmethod
    def _first_output(outputs: object) -> dict[str, Any] | None:
        if not isinstance(outputs, Mapping):
            return None
        for node_output in outputs.values():
            if not isinstance(node_output, Mapping):
                continue
            for key in ("images", "videos", "audio"):
                files = node_output.get(key)
                if isinstance(files, list) and files and isinstance(files[0], Mapping):
                    return dict(files[0])
        return None


DEFAULT_MEDIA_PROVIDERS = MediaProviderRegistry({"comfyui": ComfyUiPlugin})
