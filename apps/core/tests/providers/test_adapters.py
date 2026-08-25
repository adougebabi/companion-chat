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


def test_openai_adapter_probes_structured_stream_and_embedding_capabilities() -> None:
    calls: list[tuple[str, str, dict[str, str], bytes]] = []

    def fake(
        method: str, url: str, headers: dict[str, str], body: bytes, timeout: float
    ) -> HttpResult:
        calls.append((method, url, headers, body))
        if url.endswith("/models"):
            return HttpResult(200, json.dumps({"data": [{"id": "model"}]}).encode())
        if url.endswith("/embeddings"):
            return HttpResult(200, json.dumps({"data": [{"embedding": [0.1, 0.2]}]}).encode())
        if b'"stream": true' in body:
            return HttpResult(
                200,
                b'data: {"choices":[{"delta":{"content":"ok"}}]}\ndata: [DONE]\n\n',
            )
        return HttpResult(200, json.dumps({"choices": [{"message": {"content": "{}"}}]}).encode())

    adapter = OpenAICompatibleAdapter(fake)
    endpoint = ProviderEndpoint("endpoint", "openai-compatible", "http://provider", "provider:key")
    secret = SecretValue("super-secret")
    structured = asyncio.run(
        adapter.preflight(
            RoleAssignment(ModelRole.REFLECTION, "endpoint", "model", 10, 1), endpoint, secret
        )
    )
    stream = asyncio.run(
        adapter.preflight(
            RoleAssignment(ModelRole.ACTION_REALIZATION, "endpoint", "model", 10, 1),
            endpoint,
            secret,
        )
    )
    embedding = asyncio.run(
        adapter.preflight(
            RoleAssignment(ModelRole.EMBEDDING, "endpoint", "model", 10, 1), endpoint, secret
        )
    )
    assert structured.available and stream.available and embedding.available
    assert embedding.capability_version == "dimensions:2"
    assert all(headers["authorization"] == "Bearer super-secret" for _, _, headers, _ in calls)


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


def test_openai_adapter_rejects_empty_stream_and_invalid_structured_payload() -> None:
    def fake(
        method: str, url: str, headers: dict[str, str], body: bytes, timeout: float
    ) -> HttpResult:
        if url.endswith("/models"):
            return HttpResult(200, json.dumps({"data": [{"id": "model"}]}).encode())
        if b'"stream": true' in body:
            return HttpResult(200, b'data: {"choices":[]}\ndata: [DONE]\n')
        return HttpResult(
            200,
            json.dumps({"choices": [{"message": {"content": "not-json"}}]}).encode(),
        )

    adapter = OpenAICompatibleAdapter(fake)
    endpoint = ProviderEndpoint("endpoint", "openai-compatible", "http://provider", "provider:key")
    secret = SecretValue("secret")
    stream = asyncio.run(
        adapter.preflight(
            RoleAssignment(ModelRole.ACTION_REALIZATION, "endpoint", "model", 10, 1),
            endpoint,
            secret,
        )
    )
    structured = asyncio.run(
        adapter.preflight(
            RoleAssignment(ModelRole.REFLECTION, "endpoint", "model", 10, 1),
            endpoint,
            secret,
        )
    )
    assert stream.available is False
    assert structured.available is False


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
        if b"assessment.v1" in body:
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
        return HttpResult(
            200, json.dumps({"choices": [{"message": {"content": "realized text"}}]}).encode()
        )

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
            schema_version="assessment.v1",
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
    assert len(calls) == 4


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
