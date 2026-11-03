#!/usr/bin/env python3
"""Fixed synthetic OpenAI-compatible probes; never executes any tool."""
import argparse
import copy
import io
import json
import socket
import time
import unittest
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

BODY_LIMIT = 2 * 1024 * 1024
ERROR_LIMIT = 16 * 1024
RESULT = "PROBE_RESULT_7318"


def closed(properties):
    return {"type": "object", "properties": properties,
            "required": list(properties), "additionalProperties": False}


def tool(name, empty=False):
    return {"type": "function", "function": {
        "name": name, "description": "Read-only synthetic compatibility probe. No business side effects.",
        "parameters": closed({} if empty else {"value": {"type": "string", "enum": ["7318"]}})}}


def cases(model):
    result = {}

    def add(name, prompt, kind, required=True, **extra):
        request = {"model": model, "messages": [
            {"role": "system", "content": "Follow the synthetic compatibility test instructions exactly."},
            {"role": "user", "content": prompt}], "temperature": 0,
            "max_completion_tokens": 128, "enable_thinking": False, "stream": False}
        request.update(extra)
        result[name] = {"request": request, "kind": kind, "required": required}

    add("text", "Reply with exactly PROBE_TEXT_7318.", "text")
    basic = closed({"value": {"type": "string", "enum": ["7318"]}})
    nullable = closed({"value": {"anyOf": [{"type": "string"}, {"type": "null"}]}})
    conditional = {"oneOf": [closed({"kind": {"const": "text"}, "value": {"type": "string"}}),
                             closed({"kind": {"const": "number"}, "value": {"type": "integer"}})]}
    for name, schema, prompt, required in [
        ("strict_json", basic, 'Return exactly {"value":"7318"}.', True),
        ("nullable_anyof", nullable, 'Return exactly {"value":null}.', False),
        ("conditional_oneof", conditional, 'Return exactly {"kind":"number","value":7318}.', False)]:
        add(name, prompt, name, required, max_completion_tokens=256 if name == "conditional_oneof" else 128,
            response_format={"type": "json_schema", "json_schema": {
                "name": "compatibility_probe", "strict": True, "schema": schema}})
    for name, function, empty in [("required_empty", "probe_read", True),
                                  ("required_underscore", "probe_read", False),
                                  ("required_dot", "probe.read", False)]:
        add(name, "Call the provided read-only probe once" + (" with no arguments." if empty else ' with value="7318".'),
            "native", tools=[tool(function, empty)], tool_choice="required")
    add("auto", 'Call probe_read with value="7318" to read the synthetic value.', "native", False,
        tools=[tool("probe_read")], tool_choice="auto")
    add("none", "Reply with exactly PROBE_TEXT_7318. Do not call a tool.", "text", False,
        tools=[tool("probe_read")], tool_choice="none")
    add("required_sse", 'Call probe_read with value="7318" once.', "native", stream=True,
        stream_options={"include_usage": True}, tools=[tool("probe_read")], tool_choice="required")
    add("two_tools", 'Call both probe_read and probe_other with value="7318".', "native", False,
        tools=[tool("probe_read"), tool("probe_other")], tool_choice="required", parallel_tool_calls=True)
    add("tools_strict_json", 'Call probe_read with value="7318" first; do not return final JSON yet.',
        "native", False, tools=[tool("probe_read")], tool_choice="required",
        response_format=copy.deepcopy(result["strict_json"]["request"]["response_format"]))
    add("auto_tools_strict_json", 'Call probe_read with value="7318" first; do not return final JSON yet.',
        "native", False, tools=[tool("probe_read")], tool_choice="auto",
        response_format=copy.deepcopy(result["strict_json"]["request"]["response_format"]))
    for variant in ("empty", "null"):
        add("continuation_" + variant, "Return the exact text from the previous tool result. Do not call a tool.",
            "continuation", variant == "empty", tools=[tool("probe_read")], tool_choice="none")
        request = result["continuation_" + variant]["request"]
        request["messages"][1:1] = [
            {"role": "user", "content": "Read a synthetic value using probe_read."},
            {"role": "assistant", "content": "" if variant == "empty" else None,
             "tool_calls": [{"id": "probe0001", "type": "function", "function": {
                 "name": "probe_read", "arguments": '{"value":"7318"}'}}]},
            {"role": "tool", "tool_call_id": "probe0001", "content": RESULT}]
    return result


def read_bounded(response, limit, deadline):
    chunks, size = [], 0
    while size <= limit:
        if time.monotonic() >= deadline:
            raise TimeoutError("total response deadline exceeded")
        transport = getattr(getattr(getattr(response, "fp", None), "raw", None), "_sock", None)
        if transport is not None:
            transport.settimeout(max(0.001, deadline - time.monotonic()))
        chunk = response.read1(min(65536, limit + 1 - size))
        if not chunk:
            break
        chunks.append(chunk)
        size += len(chunk)
    body = b"".join(chunks)
    return body[:limit], len(body) > limit


def aggregate_sse(raw):
    events, calls, content, finish, usage, done = [], {}, "", None, None, False
    normalized = raw.replace("\r\n", "\n").replace("\r", "\n")
    for frame in normalized.split("\n\n"):
        data = "\n".join(line[5:].lstrip(" ") for line in frame.split("\n") if line.startswith("data:"))
        if not data:
            continue
        if done:
            raise ValueError("SSE data after DONE")
        if data == "[DONE]":
            done = True
            continue
        event = json.loads(data)
        events.append(event)
        if event.get("usage") is not None:
            usage = event["usage"]
        for choice in event.get("choices", []):
            if choice.get("index", 0) != 0:
                continue
            if choice.get("finish_reason") is not None:
                finish = choice["finish_reason"]
            delta = choice.get("delta", {})
            content += delta.get("content") or ""
            for call in delta.get("tool_calls", []):
                index = call.get("index")
                if type(index) is not int or index < 0:
                    raise ValueError("native delta lacks valid tool index")
                target = calls.setdefault(index, {"id": "", "type": "", "function": {"name": "", "arguments": ""}})
                for key in ("id", "type"):
                    if call.get(key):
                        # IDs/type are metadata, not argument fragments; repeated identical values are legal.
                        if target[key] and target[key] != call[key]:
                            raise ValueError("conflicting native tool metadata")
                        target[key] = call[key]
                for key in ("name", "arguments"):
                    target["function"][key] += call.get("function", {}).get(key) or ""
    return {"choices": [{"index": 0, "message": {"content": content,
            "tool_calls": [calls[i] for i in sorted(calls)]}, "finish_reason": finish}],
            "usage": usage}, {"events": events, "done": done, "tool_indices": sorted(calls)}


def inspect_response(case, response, stream=None):
    choices = response.get("choices", [])
    if not choices or not isinstance(choices[0].get("message"), dict):
        raise ValueError("missing completion message")
    choice, native = choices[0], []
    message = choice["message"]
    content = message.get("content") or ""
    offered = {t["function"]["name"]: t["function"]["parameters"]
               for t in case["request"].get("tools", [])}
    calls = message.get("tool_calls") or []
    seen = set()
    for call in calls:
        function = call.get("function") or {}
        arguments, parse_valid = None, False
        try:
            arguments = json.loads(function.get("arguments", ""))
            parse_valid = isinstance(arguments, dict)
        except (ValueError, TypeError):
            pass
        name, identity = function.get("name"), call.get("id")
        expected = {} if offered.get(name, {}).get("properties") == {} else {"value": "7318"}
        valid = (call.get("type") == "function" and isinstance(identity, str) and bool(identity.strip())
                 and identity not in seen and name in offered and parse_valid and arguments == expected)
        seen.add(identity)
        native.append({"id": identity, "name": name, "arguments": function.get("arguments"),
                       "arguments_parse_valid": parse_valid, "arguments_object": arguments, "valid": valid})
    parsed, json_valid = None, False
    try:
        parsed = json.loads(content)
        json_valid = True
    except (ValueError, TypeError):
        pass
    kind = case["kind"]
    expected_json = {"strict_json": {"value": "7318"}, "nullable_anyof": {"value": None},
                     "conditional_oneof": {"kind": "number", "value": 7318}}
    if kind == "native":
        passed = bool(native) and all(c["valid"] for c in native)
    elif kind in expected_json:
        passed = json_valid and parsed == expected_json[kind] and not calls
        if kind == "conditional_oneof" and passed:
            passed = type(parsed["value"]) is int
    else:
        passed = content.strip() == (RESULT if kind == "continuation" else "PROBE_TEXT_7318") and not calls
    if stream is not None:
        passed = passed and stream["done"] and bool(stream["events"])
    return {"classification": "PASS" if passed else "FAIL", "failure_layer": None if passed else "semantic_or_native_protocol",
            "content": content, "native_calls": native, "native_count": len(native),
            "multiple_native_calls_observed": len(native) > 1, "final_json_parse_valid": json_valid,
            "final_json": parsed, "return_shape": "native" if calls else "final_content",
            "finish_reason": choice.get("finish_reason"), "usage": response.get("usage")}


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def run_case(endpoint, case, timeout):
    started = time.monotonic()
    record = {"request": case["request"], "required": case["required"], "http_status": None,
              "response": None, "classification": "BLOCKED", "failure_layer": "transport"}
    request = urllib.request.Request(endpoint, data=json.dumps(case["request"]).encode(),
                                     headers={"Content-Type": "application/json"}, method="POST")
    try:
        opener = urllib.request.build_opener(NoRedirect())
        with opener.open(request, timeout=timeout) as response:
            record["http_status"] = response.status
            body, truncated = read_bounded(response, BODY_LIMIT, started + timeout)
            record["raw_response"] = body.decode("utf-8", errors="replace")
            record["response_truncated"] = truncated
            if truncated:
                raise ValueError("response exceeds 2MiB observation limit")
            stream = None
            if case["request"]["stream"]:
                if "text/event-stream" not in response.headers.get("Content-Type", ""):
                    raise ValueError("stream request did not return text/event-stream")
                parsed, stream = aggregate_sse(record["raw_response"])
                record["sse"] = stream
            else:
                parsed = json.loads(record["raw_response"])
            record["response"] = parsed
            record.update(inspect_response(case, parsed, stream))
    except urllib.error.HTTPError as error:
        record["http_status"] = error.code
        record["failure_layer"] = "http"
        try:
            with error:
                body, truncated = read_bounded(error, ERROR_LIMIT, started + timeout)
            detail = body.decode("utf-8", errors="replace")
            record.update(error_body=detail, error_body_truncated=truncated)
            # Only an explicit feature rejection is unsupported; other HTTP errors stay blocked.
            if error.code in (400, 422, 501) and any(s in detail.lower() for s in
                    ("not supported", "unsupported", "not implemented")):
                record["classification"] = "UNSUPPORTED"
        except (OSError, TimeoutError) as read_error:
            record["error_body_read_error"] = str(read_error)[:2048]
    except (urllib.error.URLError, TimeoutError, socket.timeout, OSError) as error:
        record["error"] = str(error)[:2048]
    except (ValueError, TypeError, KeyError, AttributeError) as error:
        record.update(error=str(error)[:2048], failure_layer="response_protocol")
    record["duration_ms"] = round((time.monotonic() - started) * 1000, 3)
    return record


def self_test():
    from unittest.mock import patch

    class Tests(unittest.TestCase):
        def test_sse(self):
            frames = [{"choices": [{"index": 0, "delta": {"tool_calls": [
                {"index": 1, "id": "probe0002", "type": "function", "function": {"name": "probe_other", "arguments": '{"value":'}},
                {"index": 0, "id": "probe0001", "type": "function", "function": {"name": "probe_read", "arguments": '{"value":'}}]}}]},
                {"choices": [{"index": 0, "delta": {"tool_calls": [
                    {"index": 0, "function": {"arguments": '"7318"}'}}, {"index": 1, "function": {"arguments": '"7318"}'}}]}, "finish_reason": "tool_calls"}]},
                {"choices": [], "usage": {"completion_tokens": 17}}]
            raw = "".join("data: " + json.dumps(f) + "\r\n\r\n" for f in frames) + "data: [DONE]\r\n\r\n"
            response, stream = aggregate_sse(raw)
            self.assertEqual(stream["tool_indices"], [0, 1])
            observed = inspect_response(cases("fake")["two_tools"], response, stream)
            self.assertEqual(observed["classification"], "PASS")
            self.assertEqual(observed["usage"]["completion_tokens"], 17)
            self.assertEqual(observed["finish_reason"], "tool_calls")
            self.assertTrue(observed["multiple_native_calls_observed"])

        def test_content_is_not_native(self):
            response = {"choices": [{"message": {"content": '{"tool_calls":[{}]}'}}]}
            self.assertEqual(inspect_response(cases("fake")["required_empty"], response)["classification"], "FAIL")

        def test_fake_stream_transport(self):
            class FakeResponse(io.BytesIO):
                status = 200
                headers = {"Content-Type": "text/event-stream"}
            event = {"choices": [{"index": 0, "delta": {"tool_calls": [{"index": 0,
                "id": "probe0001", "type": "function", "function": {"name": "probe_read",
                "arguments": '{"value":"7318"}'}}]}, "finish_reason": "tool_calls"}]}
            response = FakeResponse(("data: " + json.dumps(event) + "\n\ndata: [DONE]\n\n").encode())
            with patch("urllib.request.OpenerDirector.open", return_value=response):
                observed = run_case("http://synthetic/v1/chat/completions", cases("fake")["required_sse"], 1)
            self.assertEqual(observed["classification"], "PASS")
            self.assertTrue(observed["sse"]["done"])
            self.assertEqual(observed["http_status"], 200)
            self.assertTrue(response.closed)

        def test_limits_and_deadline(self):
            body, truncated = read_bounded(io.BytesIO(b"x" * 20), 16, time.monotonic() + 1)
            self.assertEqual(body, b"x" * 16)
            self.assertTrue(truncated)
            with self.assertRaises(TimeoutError):
                read_bounded(io.BytesIO(b"x"), 16, time.monotonic() - 1)

        def test_http_error_is_not_semantic_failure(self):
            case = cases("fake")["required_empty"]
            for status, body, expected in [(400, b'{"error":{}}', "BLOCKED"),
                                            (422, b'feature not supported', "UNSUPPORTED"),
                                            (507, b'x' * (ERROR_LIMIT + 100), "BLOCKED")]:
                error = urllib.error.HTTPError("http://synthetic/v1/chat/completions", status,
                                               "synthetic", {}, io.BytesIO(body))
                with patch("urllib.request.OpenerDirector.open", side_effect=error):
                    observed = run_case("http://synthetic/v1/chat/completions", case, 1)
                self.assertEqual(observed["classification"], expected)
                self.assertEqual(observed["http_status"], status)
                self.assertLessEqual(len(observed["error_body"]), ERROR_LIMIT)
                self.assertEqual(observed["error_body_truncated"], len(body) > ERROR_LIMIT)

        def test_native_validation(self):
            case = cases("fake")["required_underscore"]
            call = {"id": "probe0001", "type": "function", "function": {"name": "probe_read", "arguments": '{"value":"7318"}'}}
            response = {"choices": [{"message": {"tool_calls": [call]}}]}
            self.assertEqual(inspect_response(case, response)["classification"], "PASS")
            call["id"] = ""
            self.assertEqual(inspect_response(case, response)["classification"], "FAIL")
            call["id"] = "probe0001"
            call["function"]["arguments"] = "{"
            self.assertFalse(inspect_response(case, response)["native_calls"][0]["arguments_parse_valid"])

        def test_continuation_and_cases(self):
            matrix = cases("fake")
            for case in matrix.values():
                self.assertFalse(case["request"]["enable_thinking"])
                self.assertLessEqual(case["request"]["max_completion_tokens"], 256)
            auto_schema = matrix["auto_tools_strict_json"]
            self.assertFalse(auto_schema["required"])
            self.assertEqual(auto_schema["request"]["tool_choice"], "auto")
            self.assertEqual(auto_schema["request"]["response_format"], matrix["strict_json"]["request"]["response_format"])
            self.assertIsNot(auto_schema["request"]["response_format"], matrix["strict_json"]["request"]["response_format"])
            history = matrix["continuation_empty"]["request"]["messages"]
            self.assertEqual(history[-2]["content"], RESULT)
            self.assertNotIn(RESULT, history[-1]["content"])
            response = {"choices": [{"message": {"content": RESULT}}]}
            self.assertEqual(inspect_response(matrix["continuation_empty"], response)["classification"], "PASS")
    return unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(Tests)).wasSuccessful()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", help="OpenAI base URL, e.g. http://localhost:1234/v1")
    parser.add_argument("--model")
    parser.add_argument("--output", type=Path)
    parser.add_argument("--case", action="append", choices=list(cases("unused")), help="Repeat to select cases; others are NOT_RUN")
    parser.add_argument("--timeout", type=float, default=30)
    parser.add_argument("--self-test", action="store_true")
    args = parser.parse_args()
    if args.self_test:
        return 0 if self_test() else 1
    if not args.base_url or not args.model or not args.output:
        parser.error("--base-url, --model and --output are required unless --self-test")
    url = urllib.parse.urlsplit(args.base_url)
    if url.scheme not in ("http", "https") or not url.hostname or url.username or url.password or url.query or url.fragment:
        parser.error("base URL must be HTTP(S) without credentials, query or fragment")
    if not 0 < args.timeout <= 120:
        parser.error("timeout must be greater than zero and at most 120 seconds")
    endpoint = args.base_url.rstrip("/") + "/chat/completions"
    matrix, records = cases(args.model), {}
    for name, case in matrix.items():
        records[name] = (run_case(endpoint, case, args.timeout) if not args.case or name in args.case else
                         {"request": case["request"], "required": case["required"], "classification": "NOT_RUN",
                          "http_status": None, "duration_ms": None, "response": None})
        print(name + ": " + records[name]["classification"], flush=True)
    passed = all(r["classification"] == "PASS" for r in records.values() if r["required"])
    report = {"version": "lmstudio-compatibility.v1", "endpoint": endpoint, "model": args.model,
              "prerequisites_passed": passed, "cases": records,
              "limits": {"timeout_seconds": args.timeout, "response_bytes": BODY_LIMIT, "error_bytes": ERROR_LIMIT},
              "scope": "Fixed synthetic provider requests only. No tool execution, Core calls, retries or model loading."}
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
