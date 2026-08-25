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
                time.sleep(0.25)
            return
        self._json(
            {
                "choices": [
                    {"message": {"content": json.dumps(assessment_payload())}}
                ]
            }
        )

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
    args = parser.parse_args()
    ThreadingHTTPServer((args.host, args.port), Handler).serve_forever()


if __name__ == "__main__":
    main()
