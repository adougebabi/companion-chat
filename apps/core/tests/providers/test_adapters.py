import asyncio
import json
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from threading import Thread

import pytest
from fluctlight_core.providers.adapters import HttpResult, OpenAICompatibleAdapter
from fluctlight_core.providers.contracts import ModelRole
from fluctlight_core.providers.service import ProviderEndpoint, RoleAssignment
from fluctlight_core.settings.crypto import SecretValue


def test_openai_adapter_preflight_only_checks_authenticated_model_availability() -> None:
    calls: list[tuple[str, str, dict[str, str], bytes]] = []

    def fake(
        method: str, url: str, headers: dict[str, str], body: bytes, timeout: float
    ) -> HttpResult:
        calls.append((method, url, headers, body))
        if url.endswith("/models"):
            return HttpResult(200, json.dumps({"data": [{"id": "model"}]}).encode())
        return HttpResult(200, b"{}")

    adapter = OpenAICompatibleAdapter(fake)
    endpoint = ProviderEndpoint("endpoint", "openai-compatible", "http://provider", "provider:key")
    secret = SecretValue("super-secret")
    reports = [
        asyncio.run(
            adapter.preflight(RoleAssignment(role, "endpoint", "model", 10, 1), endpoint, secret)
        )
        for role in ModelRole
    ]
    assert all(report.available for report in reports)
    assert {report.capability_version for report in reports} == {"model-listed"}
    assert all(headers["authorization"] == "Bearer super-secret" for _, _, headers, _ in calls)


def test_openai_adapter_lists_only_available_model_ids() -> None:
    calls: list[tuple[str, str, dict[str, str], bytes]] = []

    def fake(
        method: str, url: str, headers: dict[str, str], body: bytes, timeout: float
    ) -> HttpResult:
        calls.append((method, url, headers, body))
        return HttpResult(
            200,
            json.dumps(
                {
                    "data": [
                        {"id": "model-b", "created": 1},
                        {"id": "model-a", "owned_by": "provider"},
                        {"id": "model-a"},
                        {"id": "  "},
                        {"id": 42},
                        "not-a-model",
                    ]
                }
            ).encode(),
        )

    models = asyncio.run(
        OpenAICompatibleAdapter(fake).list_models(
            ProviderEndpoint("endpoint", "openai-compatible", "http://provider", "provider:key"),
            SecretValue("super-secret"),
        )
    )

    assert models == ("model-a", "model-b")
    assert calls == [("GET", "http://provider/models", calls[0][2], b"")]
    assert calls[0][2]["authorization"] == "Bearer super-secret"


def test_openai_adapter_rejects_invalid_model_list_without_exposing_payload() -> None:
    adapter = OpenAICompatibleAdapter(lambda *_: HttpResult(200, b'{"data": {}}'))
    with pytest.raises(RuntimeError, match="Provider model list was invalid"):
        asyncio.run(
            adapter.list_models(
                ProviderEndpoint(
                    "endpoint", "openai-compatible", "http://provider", "provider:key"
                ),
                SecretValue("super-secret"),
            )
        )


def test_openai_adapter_supports_an_explicitly_secretless_local_endpoint() -> None:
    calls: list[tuple[str, str, dict[str, str]]] = []

    def fake(
        method: str, url: str, headers: dict[str, str], body: bytes, timeout: float
    ) -> HttpResult:
        calls.append((method, url, headers))
        if url.endswith("/models"):
            return HttpResult(200, json.dumps({"data": [{"id": "local-embedding"}]}).encode())
        return HttpResult(200, json.dumps({"data": [{"embedding": [0.5, -0.25]}]}).encode())

    adapter = OpenAICompatibleAdapter(fake)
    endpoint = ProviderEndpoint(
        "local", "openai-compatible", "http://provider/v1", "provider:local"
    )
    assert asyncio.run(adapter.list_models(endpoint, None)) == ("local-embedding",)
    assert asyncio.run(
        adapter.embed(
            RoleAssignment(ModelRole.EMBEDDING, "local", "local-embedding", 100, 10),
            endpoint,
            None,
            text="local embedding",
        )
    ) == (0.5, -0.25)
    assert all("authorization" not in headers for _, _, headers in calls)


def test_openai_adapter_rejects_unknown_model_without_fallback() -> None:
    adapter = OpenAICompatibleAdapter(
        lambda *_: HttpResult(200, json.dumps({"data": [{"id": "other"}]}).encode())
    )
    report = asyncio.run(
        adapter.preflight(
            RoleAssignment(ModelRole.COGNITIVE_ASSESSMENT, "endpoint", "missing", 10, 1),
            ProviderEndpoint("endpoint", "openai", "http://provider", "provider:key"),
            SecretValue("secret"),
        )
    )
    assert report.available is False


def test_openai_adapter_does_not_infer_runtime_capabilities_during_preflight() -> None:
    def fake(
        method: str, url: str, headers: dict[str, str], body: bytes, timeout: float
    ) -> HttpResult:
        if url.endswith("/models"):
            return HttpResult(200, json.dumps({"data": [{"id": "model"}]}).encode())
        return HttpResult(
            200,
            json.dumps({"choices": [{"message": {"content": "not-json"}}]}).encode(),
        )

    adapter = OpenAICompatibleAdapter(fake)
    endpoint = ProviderEndpoint("endpoint", "openai-compatible", "http://provider", "provider:key")
    secret = SecretValue("secret")
    report = asyncio.run(
        adapter.preflight(
            RoleAssignment(ModelRole.REFLECTION, "endpoint", "model", 10, 1),
            endpoint,
            secret,
        )
    )
    assert report.available is True


def test_openai_adapter_executes_structured_realization_and_embedding_ports() -> None:
    calls: list[tuple[str, str, bytes]] = []

    def fake(
        method: str, url: str, headers: dict[str, str], body: bytes, timeout: float
    ) -> HttpResult:
        calls.append((method, url, body))
        if url.endswith("/embeddings"):
            return HttpResult(200, json.dumps({"data": [{"embedding": [0.25, -0.5]}]}).encode())
        if b'"stream": true' in body:
            return HttpResult(
                200,
                b'data: {"choices":[{"delta":{"content":"stream "}}]}\n'
                b'data: {"choices":[{"delta":{"content":"text"}}]}\n'
                b"data: [DONE]\n",
            )
        if b'"stream": true' not in body:
            return HttpResult(
                200,
                json.dumps(
                    {
                        "choices": [
                            {
                                "message": {
                                    "content": json.dumps({"ok": True, "schema": "assessment.v1"})
                                }
                            }
                        ]
                    }
                ).encode(),
            )
        raise AssertionError("unexpected provider request")

    adapter = OpenAICompatibleAdapter(fake)
    endpoint = ProviderEndpoint("endpoint", "openai-compatible", "http://provider", "provider:key")
    secret = SecretValue("super-secret")
    assignment = RoleAssignment(ModelRole.COGNITIVE_ASSESSMENT, "endpoint", "model", 100, 10)
    structured = asyncio.run(
        adapter.complete_structured(
            assignment,
            endpoint,
            secret,
            messages=[{"role": "user", "content": "assess"}],
            schema_version="semantic.assessment.v1",
            request_id="provider-request-1",
        )
    )
    media_response = asyncio.run(
        adapter.complete_structured(
            RoleAssignment(ModelRole.ACTION_REALIZATION, "endpoint", "model", 100, 10),
            endpoint,
            secret,
            messages=[{"role": "user", "content": "prepare media response"}],
            schema_version="action.realization.media.v1",
            request_id="provider-request-1",
        )
    )
    realized = asyncio.run(
        adapter.stream_realization(
            RoleAssignment(ModelRole.ACTION_REALIZATION, "endpoint", "model", 100, 10),
            endpoint,
            secret,
            messages=[{"role": "user", "content": "realize"}],
            request_id="provider-request-1",
        )
    )

    async def collect_chunks() -> list[str]:
        return [
            chunk
            async for chunk in adapter.stream_realization_chunks(
                RoleAssignment(ModelRole.ACTION_REALIZATION, "endpoint", "model", 100, 10),
                endpoint,
                secret,
                messages=[{"role": "user", "content": "realize"}],
            )
        ]

    realized_chunks = asyncio.run(collect_chunks())
    vector = asyncio.run(
        adapter.embed(
            RoleAssignment(ModelRole.EMBEDDING, "endpoint", "model", 100, 10),
            endpoint,
            secret,
            text="embed",
        )
    )
    assert structured == {"ok": True, "schema": "assessment.v1"}
    assert realized == "stream text"
    assert realized_chunks == ["stream text"]
    assert vector == (0.25, -0.5)
    assert structured == {"ok": True, "schema": "assessment.v1"}
    assert media_response == {"ok": True, "schema": "assessment.v1"}
    assert len(calls) == 5
    embedding_payload = json.loads(calls[-1][2])
    assert embedding_payload["input"] == ["embed"]
    structured_payload = json.loads(calls[0][2])
    response_format = structured_payload["response_format"]
    assert response_format["type"] == "json_schema"
    assert response_format["json_schema"]["name"] == "semantic_assessment_v1"
    assert response_format["json_schema"]["strict"] is True
    assert response_format["json_schema"]["schema"]["properties"]["assessment"]["type"] == "object"
    assert structured_payload["metadata"]["schema_version"] == "semantic.assessment.v1"


def test_openai_adapter_collects_sse_media_tool_call_arguments() -> None:
    def fake(
        method: str, url: str, headers: dict[str, str], body: bytes, timeout: float
    ) -> HttpResult:
        assert b'"stream": true' in body

        def frame(delta: dict[str, object]) -> bytes:
            return f"data: {json.dumps({'choices': [{'delta': delta}]})}\n".encode()

        return HttpResult(
            200,
            b"".join(
                [
                    frame({"content": "I will take one. "}),
                    frame(
                        {
                            "tool_calls": [
                                {
                                    "index": 0,
                                    "function": {
                                        "name": "request_media",
                                        "arguments": '{"subject":"man"',
                                    },
                                }
                            ]
                        }
                    ),
                    frame(
                        {
                            "tool_calls": [
                                {
                                    "index": 0,
                                    "function": {"arguments": ',"kind":"image"}'},
                                }
                            ]
                        }
                    ),
                    b"data: [DONE]\n",
                ]
            ),
        )

    result = asyncio.run(
        OpenAICompatibleAdapter(fake).stream_media_tool_call(
            RoleAssignment(ModelRole.ACTION_REALIZATION, "endpoint", "model", 100, 10),
            ProviderEndpoint("endpoint", "openai-compatible", "http://provider", "provider:key"),
            SecretValue("secret"),
            messages=[{"role": "user", "content": "take a photo"}],
        )
    )
    assert result.text == "I will take one. "
    assert result.arguments == {"subject": "man", "kind": "image"}


def test_openai_adapter_executes_against_a_real_local_http_endpoint() -> None:
    if os.environ.get("FLUCTLIGHT_PROVIDER_SOCKET_TEST") != "1":
        pytest.skip("local socket execution requires the external integration environment")
    requests: list[tuple[str, dict[str, str], dict[str, object]]] = []

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, format: str, *args: object) -> None:
            return

        def do_GET(self) -> None:
            if self.path != "/models":
                self.send_error(404)
                return
            self._send_json({"data": [{"id": "model"}]})

        def do_POST(self) -> None:
            length = int(self.headers["content-length"])
            payload = json.loads(self.rfile.read(length))
            requests.append((self.path, dict(self.headers), payload))
            if self.path == "/embeddings":
                self._send_json({"data": [{"embedding": [1.0, 2.0]}]})
                return
            if payload.get("stream") is True:
                body = (
                    b'data: {"choices":[{"delta":{"content":"hello "}}]}\n'
                    b'data: {"choices":[{"delta":{"content":"world"}}]}\n'
                    b"data: [DONE]\n"
                )
                self.send_response(200)
                self.send_header("content-type", "text/event-stream")
                self.send_header("content-length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)
                return
            self._send_json(
                {
                    "choices": [
                        {"message": {"content": json.dumps({"ok": True, "schema": "integration"})}}
                    ]
                }
            )

        def _send_json(self, value: dict[str, object]) -> None:
            body = json.dumps(value).encode()
            self.send_response(200)
            self.send_header("content-type", "application/json")
            self.send_header("content-length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = Thread(target=server.serve_forever, daemon=True)
    thread.start()
    endpoint = ProviderEndpoint(
        "endpoint",
        "openai-compatible",
        f"http://127.0.0.1:{server.server_port}",
        "provider:key",
    )
    adapter = OpenAICompatibleAdapter()
    secret = SecretValue("integration-secret")
    try:
        structured = asyncio.run(
            adapter.complete_structured(
                RoleAssignment(ModelRole.COGNITIVE_ASSESSMENT, "endpoint", "model", 100, 10),
                endpoint,
                secret,
                messages=[{"role": "user", "content": "hello"}],
                schema_version="integration",
                request_id="provider-request-integration",
            )
        )
        realized = asyncio.run(
            adapter.stream_realization(
                RoleAssignment(ModelRole.ACTION_REALIZATION, "endpoint", "model", 100, 10),
                endpoint,
                secret,
                messages=[{"role": "user", "content": "hello"}],
                request_id="provider-request-integration",
            )
        )
        vector = asyncio.run(
            adapter.embed(
                RoleAssignment(ModelRole.EMBEDDING, "endpoint", "model", 100, 10),
                endpoint,
                secret,
                text="hello",
                request_id="embedding-request-integration",
            )
        )
    finally:
        server.shutdown()
        thread.join(timeout=2)
        server.server_close()

    assert structured == {"ok": True, "schema": "integration"}
    assert realized == "hello world"
    assert vector == (1.0, 2.0)
    assert (
        next(value for key, value in requests[0][1].items() if key.lower() == "idempotency-key")
        == "provider-request-integration"
    )
    assert (
        next(value for key, value in requests[-1][1].items() if key.lower() == "idempotency-key")
        == "embedding-request-integration"
    )
