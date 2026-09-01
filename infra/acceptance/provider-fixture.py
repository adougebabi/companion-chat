#!/usr/bin/env python3
"""Small OpenAI-compatible fixture used only by the disposable T12 smoke."""

from __future__ import annotations

import argparse
import json
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


def assessment_payload() -> dict[str, object]:
    return {
        "assessment": {
            "perception": {
                "event_kind": "conversation.message",
                "observed_intent": "reply",
                "sentiment": "neutral",
                "social_signals": [],
                "environment_meaning": None,
            },
            "appraisal": {
                "relevance": 0.8,
                "goal_congruence": 0.7,
                "reward": 0.5,
                "loss": 0.0,
                "social_threat": 0.0,
                "controllability": 0.8,
                "responsibility": 0.5,
                "relationship_significance": 0.7,
                "expected_effect": 0.7,
            },
            "direction": "neutral",
            "strength": 0.2,
            "confidence": 0.95,
            "evidence_refs": ["fixture-event"],
        },
        "decision": {
            "action_type": "reply",
            "payload": {"text": "configured Provider reply"},
            "confidence": 0.95,
            "evidence_refs": ["fixture-event"],
            "decision_id": "fixture-decision",
        },
        "model_version": "fixture-v1",
        "prompt_version": "fixture-prompt-v1",
    }


def wake_up_payload() -> dict[str, object]:
    """Return the smallest valid internal-life decision for the smoke stack."""
    return {
        "attention": "fixture context",
        "thought": "fixture internal summary",
        "desire": "maintain continuity",
        "agency": "observe without an external action",
        "action_type": "no_op",
        "evidence_refs": [],
    }


def preflight_payload(schema_version: str) -> dict[str, object]:
    contracts: dict[str, dict[str, object]] = {
        "fluctlight.initialization.v1": {
            "schema": "fluctlight.initialization.v1",
            "foundation": {},
        },
        "semantic.assessment.v1": {
            "schema": "semantic.assessment.v1",
            "assessment": {},
            "decision": {},
        },
        "fluctlight.reflection.v1": {
            "schema": "fluctlight.reflection.v1",
            "proposal": {},
        },
        "fluctlight.media-prompt.v1": {
            "schema": "fluctlight.media-prompt.v1",
            "prompt": {},
        },
    }
    return contracts.get(schema_version, assessment_payload())


class Handler(BaseHTTPRequestHandler):
    def log_message(self, format: str, *args: object) -> None:
        return

    def do_GET(self) -> None:
        if self.path == "/models":
            self._json({"data": [{"id": "fixture-model"}]})
        else:
            self.send_error(404)

    def do_POST(self) -> None:
        length = int(self.headers.get("content-length", "0"))
        payload = json.loads(self.rfile.read(length) or b"{}")
        if self.path == "/embeddings":
            self._json({"data": [{"embedding": [0.1, 0.2, 0.3]}]})
            return
        if self.path != "/chat/completions":
            self.send_error(404)
            return
        if payload.get("stream") is True:
            self.send_response(200)
            self.send_header("content-type", "text/event-stream")
            self.send_header("connection", "close")
            self.end_headers()
            for line in (
                b'data: {"choices":[{"delta":{"content":"configured "}}]}\n',
                b'data: {"choices":[{"delta":{"content":"Provider reply"}}]}\n',
                b"data: [DONE]\n",
            ):
                self.wfile.write(line)
                self.wfile.flush()
                time.sleep(self.server.stream_delay_seconds)
            return
        messages = payload.get("messages", [])
        first_content = (
            messages[0].get("content", "")
            if isinstance(messages, list) and messages and isinstance(messages[0], dict)
            else ""
        )
        schema_version = str(payload.get("metadata", {}).get("schema_version", ""))
        response = (
            preflight_payload(schema_version)
            if isinstance(first_content, str) and first_content.startswith("Return JSON only for")
            else wake_up_payload()
            if isinstance(first_content, str) and "internal wake-up" in first_content
            else assessment_payload()
        )
        self._json({"choices": [{"message": {"content": json.dumps(response)}}]})

    def _json(self, payload: dict[str, object]) -> None:
        body = json.dumps(payload).encode("utf-8")
        self.send_response(200)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=18081)
    parser.add_argument("--stream-delay-seconds", type=float, default=0.25)
    args = parser.parse_args()
    server = ThreadingHTTPServer((args.host, args.port), Handler)
    server.stream_delay_seconds = args.stream_delay_seconds
    server.serve_forever()


if __name__ == "__main__":
    main()
