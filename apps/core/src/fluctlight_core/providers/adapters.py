"""Protocol-normalizing Provider adapters with no domain-policy behavior."""

from __future__ import annotations

import asyncio
import codecs
import json
from collections.abc import AsyncIterator, Callable
from dataclasses import dataclass
from math import isfinite
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

import httpx

from fluctlight_core.settings.crypto import SecretValue

from .contracts import CapabilityReport, ModelRole
from .service import ProviderEndpoint, RoleAssignment


@dataclass(frozen=True, slots=True)
class HttpResult:
    status: int
    body: bytes


HttpRequest = Callable[[str, str, dict[str, str], bytes, float], HttpResult]


def _request(
    method: str, url: str, headers: dict[str, str], body: bytes, timeout: float
) -> HttpResult:
    try:
        request = Request(url, data=body or None, headers=headers, method=method)
        with urlopen(request, timeout=timeout) as response:  # noqa: S310 - configured Owner endpoint
            return HttpResult(response.status, response.read())
    except (HTTPError, URLError, OSError) as exc:
        return HttpResult(getattr(exc, "code", 599), b"")


class OpenAICompatibleAdapter:
    """Small transport adapter used only for explicit role capability probes."""

    def __init__(self, request: HttpRequest = _request) -> None:
        self._request = request

    async def preflight(
        self,
        assignment: RoleAssignment,
        endpoint: ProviderEndpoint,
        secret: SecretValue,
    ) -> CapabilityReport:
        if endpoint.kind not in {"openai", "openai-compatible"}:
            return CapabilityReport(assignment.role, False, detail="unsupported provider kind")
        base_url = endpoint.base_url.rstrip("/")
        headers = {
            "authorization": f"Bearer {secret.reveal_for_provider()}",
            "content-type": "application/json",
        }
        timeout = float(assignment.timeout_seconds)
        models = await self._call("GET", f"{base_url}/models", headers, {}, timeout)
        if models is None or assignment.model_id not in {
            item.get("id") for item in models.get("data", []) if isinstance(item, dict)
        }:
            return CapabilityReport(
                assignment.role, False, detail="configured model is unavailable"
            )
        if assignment.role is ModelRole.EMBEDDING:
            embedding_payload: dict[str, object] = {
                "model": assignment.model_id,
                "input": "fluctlight preflight",
            }
            embedding_result = await self._call(
                "POST", f"{base_url}/embeddings", headers, embedding_payload, timeout
            )
            data = embedding_result.get("data") if isinstance(embedding_result, dict) else None
            vector = (
                data[0].get("embedding")
                if isinstance(data, list) and data and isinstance(data[0], dict)
                else None
            )
            if (
                not isinstance(vector, list)
                or not vector
                or any(
                    not isinstance(value, int | float) or not isfinite(value) for value in vector
                )
            ):
                return CapabilityReport(
                    assignment.role, False, detail="embedding capability is unavailable"
                )
            return CapabilityReport(
                assignment.role, True, capability_version=f"dimensions:{len(vector)}"
            )
        if assignment.role is ModelRole.ACTION_REALIZATION:
            stream_messages: list[dict[str, object]] = [{"role": "user", "content": "ping"}]
            try:
                chunks = [
                    chunk
                    async for chunk in self.stream_realization_chunks(
                        assignment,
                        endpoint,
                        secret,
                        messages=stream_messages,
                    )
                ]
                available = bool("".join(chunks).strip())
            except (RuntimeError, UnicodeError, httpx.HTTPError):
                available = False
            return CapabilityReport(
                assignment.role, available, detail=None if available else "streaming unavailable"
            )
        structured_payload: dict[str, object] = {
            "model": assignment.model_id,
            "messages": [{"role": "user", "content": "Return an empty JSON object."}],
            "response_format": {"type": "json_object"},
            "stream": False,
        }
        structured_result = await self._call(
            "POST", f"{base_url}/chat/completions", headers, structured_payload, timeout
        )
        available = False
        if isinstance(structured_result, dict):
            choices = structured_result.get("choices", [])
            content = (
                choices[0].get("message", {}).get("content")
                if isinstance(choices, list)
                and choices
                and isinstance(choices[0], dict)
                and isinstance(choices[0].get("message"), dict)
                else None
            )
            if isinstance(content, str):
                try:
                    parsed = json.loads(content)
                except json.JSONDecodeError:
                    parsed = None
                available = isinstance(parsed, dict)
        return CapabilityReport(
            assignment.role,
            available,
            detail=None if available else "structured output unavailable",
        )

    async def complete_structured(
        self,
        assignment: RoleAssignment,
        endpoint: ProviderEndpoint,
        secret: SecretValue,
        *,
        messages: list[dict[str, object]],
        schema_version: str,
        request_id: str | None = None,
    ) -> dict:
        if assignment.role not in {
            ModelRole.INITIALIZATION,
            ModelRole.COGNITIVE_ASSESSMENT,
            ModelRole.REFLECTION,
            ModelRole.MEDIA_PROMPT,
        }:
            raise ValueError("role does not support structured completion")
        payload = {
            "model": assignment.model_id,
            "messages": messages,
            "response_format": {"type": "json_object"},
            "stream": False,
            "metadata": {"schema_version": schema_version},
        }
        result = await self._call(
            "POST",
            f"{endpoint.base_url.rstrip('/')}/chat/completions",
            self._headers(secret, request_id),
            payload,
            float(assignment.timeout_seconds),
        )
        if not result:
            raise RuntimeError("structured Provider completion failed")
        content = result.get("choices", [{}])[0].get("message", {}).get("content")
        if not isinstance(content, str):
            raise RuntimeError("structured Provider completion had no JSON content")
        try:
            parsed = json.loads(content)
        except json.JSONDecodeError as exc:
            raise RuntimeError("structured Provider completion was not valid JSON") from exc
        if not isinstance(parsed, dict):
            raise RuntimeError("structured Provider completion must be an object")
        return parsed

    async def stream_realization(
        self,
        assignment: RoleAssignment,
        endpoint: ProviderEndpoint,
        secret: SecretValue,
        *,
        messages: list[dict[str, object]],
        request_id: str | None = None,
    ) -> str:
        if assignment.role is not ModelRole.ACTION_REALIZATION:
            raise ValueError("role does not support realization")
        content = "".join(
            [
                chunk
                async for chunk in self.stream_realization_chunks(
                    assignment,
                    endpoint,
                    secret,
                    messages=messages,
                    request_id=request_id,
                )
            ]
        )
        if not content.strip():
            raise RuntimeError("realization Provider completion was empty")
        return content

    async def stream_realization_chunks(
        self,
        assignment: RoleAssignment,
        endpoint: ProviderEndpoint,
        secret: SecretValue,
        *,
        messages: list[dict[str, object]],
        request_id: str | None = None,
    ) -> AsyncIterator[str]:
        """Yield SSE content incrementally and close the HTTP response on cancellation."""

        if assignment.role is not ModelRole.ACTION_REALIZATION:
            raise ValueError("role does not support realization")
        payload = {"model": assignment.model_id, "messages": messages, "stream": True}
        headers = self._headers(secret, request_id)
        if self._request is not _request:
            result = await self._call_raw(
                "POST",
                f"{endpoint.base_url.rstrip('/')}/chat/completions",
                headers,
                payload,
                float(assignment.timeout_seconds),
            )
            if result is None:
                raise RuntimeError("realization Provider completion failed")
            content = self._stream_content(result.body)
            if content:
                yield content
            return

        decoder = codecs.getincrementaldecoder("utf-8")()
        buffer = ""
        saw_done = False
        saw_data = False
        async with httpx.AsyncClient(timeout=float(assignment.timeout_seconds)) as client:
            async with client.stream(
                "POST",
                f"{endpoint.base_url.rstrip('/')}/chat/completions",
                headers=headers,
                json=payload,
            ) as response:
                response.raise_for_status()
                async for raw in response.aiter_bytes():
                    buffer += decoder.decode(raw)
                    while "\n" in buffer:
                        line, buffer = buffer.split("\n", 1)
                        value = line.strip()
                        if not value or not value.startswith("data:"):
                            continue
                        saw_data = True
                        data = value[5:].strip()
                        if data == "[DONE]":
                            saw_done = True
                            continue
                        try:
                            event = json.loads(data)
                        except json.JSONDecodeError as exc:
                            raise RuntimeError(
                                "realization Provider stream was not valid JSON"
                            ) from exc
                        choices = event.get("choices", []) if isinstance(event, dict) else []
                        if not isinstance(choices, list) or not choices:
                            continue
                        delta = choices[0].get("delta", {}) if isinstance(choices[0], dict) else {}
                        if isinstance(delta, dict) and isinstance(delta.get("content"), str):
                            yield delta["content"]
                buffer += decoder.decode(b"", final=True)
                if buffer.strip():
                    value = buffer.strip()
                    if value.startswith("data:") and value[5:].strip() != "[DONE]":
                        event = json.loads(value[5:].strip())
                        choices = event.get("choices", []) if isinstance(event, dict) else []
                        if choices and isinstance(choices[0], dict):
                            delta = choices[0].get("delta", {})
                            if isinstance(delta, dict) and isinstance(delta.get("content"), str):
                                yield delta["content"]
                if not saw_data or not saw_done:
                    raise RuntimeError("realization Provider stream was incomplete")

    async def embed(
        self,
        assignment: RoleAssignment,
        endpoint: ProviderEndpoint,
        secret: SecretValue,
        *,
        text: str,
        request_id: str | None = None,
    ) -> tuple[float, ...]:
        if assignment.role is not ModelRole.EMBEDDING:
            raise ValueError("role does not support embeddings")
        result = await self._call(
            "POST",
            f"{endpoint.base_url.rstrip('/')}/embeddings",
            self._headers(secret, request_id),
            {"model": assignment.model_id, "input": text},
            float(assignment.timeout_seconds),
        )
        data = result.get("data") if isinstance(result, dict) else None
        vector = (
            data[0].get("embedding")
            if isinstance(data, list) and data and isinstance(data[0], dict)
            else None
        )
        if (
            not isinstance(vector, list)
            or not vector
            or any(not isinstance(value, int | float) for value in vector)
        ):
            raise RuntimeError("embedding Provider completion was invalid")
        return tuple(float(value) for value in vector)

    @staticmethod
    def _headers(secret: SecretValue, request_id: str | None) -> dict[str, str]:
        headers = {
            "authorization": f"Bearer {secret.reveal_for_provider()}",
            "content-type": "application/json",
        }
        if request_id:
            headers["idempotency-key"] = request_id
            headers["x-fluctlight-provider-request-id"] = request_id
        return headers

    @staticmethod
    def _stream_content(body: bytes) -> str:
        try:
            parsed = json.loads(body)
        except json.JSONDecodeError:
            parsed = None
        if isinstance(parsed, dict):
            choices = parsed.get("choices", [])
            if isinstance(choices, list) and choices:
                choice = choices[0]
                if isinstance(choice, dict):
                    message = choice.get("message", {})
                    if isinstance(message, dict) and isinstance(message.get("content"), str):
                        return message["content"]
                    delta = choice.get("delta", {})
                    if isinstance(delta, dict) and isinstance(delta.get("content"), str):
                        return delta["content"]
        chunks: list[str] = []
        for line in body.decode("utf-8", errors="strict").splitlines():
            line = line.strip()
            if not line.startswith("data:"):
                continue
            value = line[5:].strip()
            if value == "[DONE]":
                break
            try:
                event = json.loads(value)
            except json.JSONDecodeError as exc:
                raise RuntimeError("realization Provider stream was not valid JSON") from exc
            choices = event.get("choices", []) if isinstance(event, dict) else []
            if not isinstance(choices, list) or not choices:
                continue
            delta = choices[0].get("delta", {}) if isinstance(choices[0], dict) else {}
            if isinstance(delta, dict) and isinstance(delta.get("content"), str):
                chunks.append(delta["content"])
        return "".join(chunks)

    async def _call(
        self, method: str, url: str, headers: dict[str, str], payload: object, timeout: float
    ) -> dict | None:
        response = await self._call_raw(method, url, headers, payload, timeout)
        if response is None or not 200 <= response.status < 300:
            return None
        try:
            parsed = json.loads(response.body)
        except json.JSONDecodeError:
            return None
        return parsed if isinstance(parsed, dict) else None

    async def _call_raw(
        self, method: str, url: str, headers: dict[str, str], payload: object, timeout: float
    ) -> HttpResult | None:
        encoded = b"" if method == "GET" else json.dumps(payload).encode("utf-8")
        result = await asyncio.to_thread(self._request, method, url, headers, encoded, timeout)
        return result if 200 <= result.status < 300 else None
