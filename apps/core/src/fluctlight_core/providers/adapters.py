"""Protocol-normalizing Provider adapters with no domain-policy behavior."""

from __future__ import annotations

import asyncio
import codecs
import json
from collections.abc import AsyncIterator, Callable
from dataclasses import dataclass
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


@dataclass(frozen=True, slots=True)
class ToolCallResult:
    text: str
    arguments: dict[str, object]


def _nullable(kind: str) -> dict[str, object]:
    return {"type": [kind, "null"]}


def _object(
    properties: dict[str, object], required: list[str] | None = None, *, open_object: bool = False
) -> dict[str, object]:
    return {
        "type": "object",
        "properties": properties,
        "required": required if required is not None else list(properties),
        "additionalProperties": open_object,
    }


def _structured_schema(schema_version: str) -> dict[str, object]:
    """Return the wire schema for a structured role response.

    This is intentionally owned by the transport boundary: prompts explain intent,
    while the provider receives an executable contract. Unknown versions retain a
    minimal object contract so diagnostics/tests can still exercise the adapter;
    production runtimes use one of the explicit versions below.
    """
    if schema_version == "fluctlight.initialization.v1":
        scalar_identity = {
            key: _nullable("string")
            for key in (
                "name",
                "gender",
                "occupation",
                "residence",
                "timezone",
                "birthday",
                "background",
                "biography",
                "worldview",
                "notes",
            )
        }
        identity = _object(
            scalar_identity
            | {
                "age": {"type": ["number", "null"]},
                "core_values": {"type": "array", "items": {"type": "string"}},
            }
        )
        personality = _object(
            {
                key: {"type": "number", "minimum": 0, "maximum": 1}
                for key in (
                    "openness",
                    "conscientiousness",
                    "extraversion",
                    "agreeableness",
                    "neuroticism",
                    "curiosity",
                    "independence",
                    "patience",
                    "empathy",
                    "assertiveness",
                    "humor",
                    "sociability",
                    "risk_tolerance",
                )
            }
        )
        policy = _object(
            {
                "response_style": _nullable("string"),
                "message_length": _nullable("string"),
                "emoji_frequency": {"type": "number", "minimum": 0, "maximum": 1},
                "punctuation_style": _nullable("string"),
                "humor_style": _nullable("string"),
                "sarcasm_tendency": {"type": "number", "minimum": 0, "maximum": 1},
                "directness": {"type": "number", "minimum": 0, "maximum": 1},
                "initiative": {"type": "number", "minimum": 0, "maximum": 1},
                "topic_initiation": {"type": "number", "minimum": 0, "maximum": 1},
                "silence_tolerance": {"type": "number", "minimum": 0, "maximum": 1},
                "response_delay": {"type": "number", "minimum": 0},
                "emotional_expression": {"type": "number", "minimum": 0, "maximum": 1},
                "conflict_style": _nullable("string"),
                "refusal_style": _nullable("string"),
                "intimacy_expression": _nullable("string"),
            }
        )
        free_item = {"type": "object", "additionalProperties": True}
        life_profile = _object(
            {
                "appearance": {"type": "object", "additionalProperties": True},
                "social_background": {"type": "object", "additionalProperties": True},
                "preferences": {"type": "object", "additionalProperties": True},
                **{
                    key: {"type": "array", "items": free_item}
                    for key in (
                        "life_habits",
                        "recurring_commitments",
                        "relationship_seeds",
                        "character_constraints",
                    )
                },
            }
        )
        provenance = {"type": "object", "additionalProperties": True}
        goal = _object(
            {
                "description": {"type": "string"},
                "importance": {"type": "number", "minimum": 0, "maximum": 1},
                "urgency": {"type": "number", "minimum": 0, "maximum": 1},
            }
        )
        intention = _object(
            {
                "goal_index": {"type": "integer", "minimum": 0},
                "action": {"type": "string"},
                "confidence": {"type": "number", "minimum": 0, "maximum": 1},
                "expiration_hours": {"type": "number", "exclusiveMinimum": 0, "maximum": 168},
            }
        )
        return _object(
            {
                "foundation": _object(
                    {
                        "identity": identity,
                        "personality": personality,
                        "behavioral_policy": policy,
                        "life_profile": life_profile,
                        "provenance": provenance,
                        "initial_goals": {
                            "type": "array",
                            "items": goal,
                            "minItems": 1,
                            "maxItems": 3,
                        },
                        "initial_intentions": {"type": "array", "items": intention, "minItems": 1},
                    }
                )
            }
        )
    if schema_version == "semantic.assessment.v1":
        perception = _object(
            {
                "event_kind": {"type": "string"},
                "observed_intent": _nullable("string"),
                "sentiment": _nullable("string"),
                "social_signals": {"type": "array", "items": {"type": "string"}},
                "environment_meaning": _nullable("string"),
            }
        )
        appraisal = _object(
            {
                key: {"type": "number", "minimum": 0, "maximum": 1}
                for key in (
                    "relevance",
                    "goal_congruence",
                    "reward",
                    "loss",
                    "social_threat",
                    "controllability",
                    "responsibility",
                    "relationship_significance",
                    "expected_effect",
                )
            }
        )
        effect = _object(
            {
                "id": {"type": "string"},
                "action_type": {"type": "string"},
                "payload": {"type": "object", "additionalProperties": True},
            }
        )
        assessment = _object(
            {
                "perception": perception,
                "appraisal": appraisal,
                "direction": {"type": "string"},
                "strength": {"type": "number", "minimum": 0, "maximum": 1},
                "confidence": {"type": "number", "minimum": 0, "maximum": 1},
                "evidence_refs": {"type": "array", "items": {"type": "string"}, "minItems": 1},
            }
        )
        decision = _object(
            {
                "effects": {"type": "array", "items": effect, "minItems": 1},
                "confidence": {"type": "number", "minimum": 0, "maximum": 1},
                "evidence_refs": {"type": "array", "items": {"type": "string"}, "minItems": 1},
                "decision_id": {"type": "string"},
            }
        )
        return _object(
            {
                "assessment": assessment,
                "decision": decision,
                "model_version": {"type": "string"},
                "prompt_version": {"type": "string"},
            }
        )
    if schema_version == "life.schedule.initial.v1":
        item = _object(
            {
                "start_at": {"type": "string"},
                "end_at": {"type": "string"},
                "activity": {"type": "string"},
                "scene": {"type": "string"},
                "item_type": {"type": "string"},
                "status": {"type": "string"},
                "priority": {"type": "number", "minimum": 0, "maximum": 1},
                "flexibility": {"type": "number", "minimum": 0, "maximum": 1},
                "interruption_cost": {"type": "number", "minimum": 0, "maximum": 1},
            }
        )
        return _object(
            {
                "items": {"type": "array", "items": item, "minItems": 1},
                "reschedule_policy": {"type": "object", "additionalProperties": True},
            }
        )
    if schema_version == "media.prompt.v1":
        return _object({"prompt": {"type": "string", "minLength": 1}})
    if schema_version == "reflection.v1":
        return _object(
            {
                "proposal_id": {"type": "string"},
                "evidence_refs": {"type": "array", "items": {"type": "string"}},
                "memory_candidates": {
                    "type": "array",
                    "items": {"type": "object", "additionalProperties": True},
                },
                "relationship_candidates": {
                    "type": "array",
                    "items": {"type": "object", "additionalProperties": True},
                },
            }
        )
    return {"type": "object", "additionalProperties": True}


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
        secret: SecretValue | None,
    ) -> CapabilityReport:
        try:
            models = await self.list_models(
                endpoint,
                secret,
                timeout_seconds=float(assignment.timeout_seconds),
            )
        except RuntimeError:
            return CapabilityReport(
                assignment.role, False, detail="configured model is unavailable"
            )
        if assignment.model_id not in models:
            return CapabilityReport(
                assignment.role, False, detail="configured model is unavailable"
            )
        return CapabilityReport(
            assignment.role,
            True,
            capability_version="model-listed",
        )

    async def list_models(
        self,
        endpoint: ProviderEndpoint,
        secret: SecretValue | None,
        *,
        timeout_seconds: float = 10.0,
    ) -> tuple[str, ...]:
        """List model identifiers without invoking a model capability."""

        if endpoint.kind not in {"openai", "openai-compatible"}:
            raise RuntimeError("unsupported provider kind")
        payload = await self._call(
            "GET",
            f"{endpoint.base_url.rstrip('/')}/models",
            self._headers(secret, None),
            {},
            timeout_seconds,
        )
        data = payload.get("data") if isinstance(payload, dict) else None
        if not isinstance(data, list):
            raise RuntimeError("Provider model list was invalid")
        return tuple(
            sorted(
                {
                    item["id"].strip()
                    for item in data
                    if isinstance(item, dict)
                    and isinstance(item.get("id"), str)
                    and item["id"].strip()
                }
            )
        )

    async def complete_structured(
        self,
        assignment: RoleAssignment,
        endpoint: ProviderEndpoint,
        secret: SecretValue | None,
        *,
        messages: list[dict[str, object]],
        schema_version: str,
        json_schema: dict[str, object] | None = None,
        request_id: str | None = None,
    ) -> dict:
        if assignment.role not in {
            ModelRole.INITIALIZATION,
            ModelRole.COGNITIVE_ASSESSMENT,
            ModelRole.ACTION_REALIZATION,
            ModelRole.REFLECTION,
            ModelRole.MEDIA_PROMPT,
        }:
            raise ValueError("role does not support structured completion")
        payload = {
            "model": assignment.model_id,
            "messages": messages,
            "response_format": {
                "type": "json_schema",
                "json_schema": {
                    "name": schema_version.replace(".", "_"),
                    "strict": True,
                    "schema": json_schema or _structured_schema(schema_version),
                },
            },
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
        secret: SecretValue | None,
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
        secret: SecretValue | None,
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
                content=json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode(
                    "utf-8"
                ),
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

    async def stream_media_tool_call(
        self,
        assignment: RoleAssignment,
        endpoint: ProviderEndpoint,
        secret: SecretValue | None,
        *,
        messages: list[dict[str, object]],
        request_id: str | None = None,
    ) -> ToolCallResult:
        if assignment.role is not ModelRole.ACTION_REALIZATION:
            raise ValueError("role does not support tool calls")
        payload = {
            "model": assignment.model_id,
            "messages": messages,
            "stream": True,
            "tools": [
                {
                    "type": "function",
                    "function": {
                        "name": "request_media",
                        "description": "Request image generation with complete visual parameters.",
                        "parameters": {
                            "type": "object",
                            "additionalProperties": True,
                        },
                    },
                }
            ],
            "tool_choice": {"type": "function", "function": {"name": "request_media"}},
        }
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
                raise RuntimeError("media tool call failed")
            return self._parse_tool_result(result.body)

        decoder = codecs.getincrementaldecoder("utf-8")()
        buffer = ""
        text_chunks: list[str] = []
        argument_chunks: list[str] = []
        saw_done = False
        async with httpx.AsyncClient(timeout=float(assignment.timeout_seconds)) as client:
            async with client.stream(
                "POST",
                f"{endpoint.base_url.rstrip('/')}/chat/completions",
                headers=headers,
                content=json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode(
                    "utf-8"
                ),
            ) as response:
                response.raise_for_status()
                async for raw in response.aiter_bytes():
                    buffer += decoder.decode(raw)
                    while "\n" in buffer:
                        line, buffer = buffer.split("\n", 1)
                        data = line.strip()
                        if not data.startswith("data:"):
                            continue
                        value = data[5:].strip()
                        if value == "[DONE]":
                            saw_done = True
                            continue
                        try:
                            event = json.loads(value)
                        except json.JSONDecodeError as exc:
                            raise RuntimeError("media tool stream was not valid JSON") from exc
                        choices = event.get("choices", []) if isinstance(event, dict) else []
                        if not choices or not isinstance(choices[0], dict):
                            continue
                        delta = choices[0].get("delta", {})
                        if not isinstance(delta, dict):
                            continue
                        if isinstance(delta.get("content"), str):
                            text_chunks.append(delta["content"])
                        tool_calls = delta.get("tool_calls", [])
                        if isinstance(tool_calls, list):
                            for tool_call in tool_calls:
                                if not isinstance(tool_call, dict):
                                    continue
                                function = tool_call.get("function", {})
                                if isinstance(function, dict) and isinstance(
                                    function.get("arguments"), str
                                ):
                                    argument_chunks.append(function["arguments"])
                buffer += decoder.decode(b"", final=True)
                if not saw_done:
                    raise RuntimeError("media tool stream was incomplete")
        try:
            arguments = json.loads("".join(argument_chunks))
        except json.JSONDecodeError as exc:
            raise RuntimeError("media tool arguments were not valid JSON") from exc
        if not isinstance(arguments, dict) or not arguments:
            raise RuntimeError("media tool call returned no arguments")
        return ToolCallResult("".join(text_chunks), arguments)

    @staticmethod
    def _parse_tool_result(body: bytes) -> ToolCallResult:
        if body.lstrip().startswith(b"data:"):
            text_chunks: list[str] = []
            argument_chunks: list[str] = []
            for line in body.decode("utf-8").splitlines():
                value = line.strip()
                if not value.startswith("data:") or value[5:].strip() == "[DONE]":
                    continue
                try:
                    event = json.loads(value[5:].strip())
                except json.JSONDecodeError as exc:
                    raise RuntimeError("media tool stream was not valid JSON") from exc
                choices = event.get("choices", []) if isinstance(event, dict) else []
                delta = choices[0].get("delta", {}) if choices else {}
                if not isinstance(delta, dict):
                    continue
                if isinstance(delta.get("content"), str):
                    text_chunks.append(delta["content"])
                for tool_call in delta.get("tool_calls", []):
                    function = tool_call.get("function", {}) if isinstance(tool_call, dict) else {}
                    if isinstance(function, dict) and isinstance(function.get("arguments"), str):
                        argument_chunks.append(function["arguments"])
            try:
                arguments = json.loads("".join(argument_chunks))
            except json.JSONDecodeError as exc:
                raise RuntimeError("media tool arguments were not valid JSON") from exc
            if not isinstance(arguments, dict) or not arguments:
                raise RuntimeError("media tool call returned no arguments")
            return ToolCallResult("".join(text_chunks), arguments)
        try:
            payload = json.loads(body)
        except json.JSONDecodeError as exc:
            raise RuntimeError("media tool response was not valid JSON") from exc
        choices = payload.get("choices", []) if isinstance(payload, dict) else []
        message = choices[0].get("message", {}) if choices and isinstance(choices[0], dict) else {}
        tool_calls = message.get("tool_calls", []) if isinstance(message, dict) else []
        function = tool_calls[0].get("function", {}) if tool_calls else {}
        raw_arguments = function.get("arguments") if isinstance(function, dict) else None
        if not isinstance(raw_arguments, str):
            raise RuntimeError("media tool response had no function arguments")
        try:
            arguments = json.loads(raw_arguments)
        except json.JSONDecodeError as exc:
            raise RuntimeError("media tool arguments were not valid JSON") from exc
        if not isinstance(arguments, dict) or not arguments:
            raise RuntimeError("media tool call returned no arguments")
        text = message.get("content", "") if isinstance(message, dict) else ""
        return ToolCallResult(text if isinstance(text, str) else "", arguments)

    async def embed(
        self,
        assignment: RoleAssignment,
        endpoint: ProviderEndpoint,
        secret: SecretValue | None,
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
            {"model": assignment.model_id, "input": [text]},
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
    def _headers(secret: SecretValue | None, request_id: str | None) -> dict[str, str]:
        headers = {"content-type": "application/json"}
        if secret is not None:
            headers["authorization"] = f"Bearer {secret.reveal_for_provider()}"
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
        encoded = (
            b""
            if method == "GET"
            else json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
        )
        result = await asyncio.to_thread(self._request, method, url, headers, encoded, timeout)
        return result if 200 <= result.status < 300 else None
