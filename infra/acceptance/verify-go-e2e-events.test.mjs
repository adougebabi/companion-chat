import assert from "node:assert/strict";
import test from "node:test";

import { verifyGoE2EEvents } from "./verify-go-e2e-events.mjs";

function event(action, testName, output = undefined) {
  return JSON.stringify({
    Time: "2026-09-22T00:00:00Z",
    Action: action,
    Package: "github.com/fluctlight/local-ai-companion/apps/core-go/internal/core",
    Test: testName,
    ...(output === undefined ? {} : { Output: output }),
  });
}

test("accepts a required passing test", () => {
  const result = verifyGoE2EEvents({
    text: `${event("run", "TestRequired")}\n${event("pass", "TestRequired")}\n`,
    goExitCode: 0,
    expectedTests: ["TestRequired"],
  });
  assert.equal(result.ok, true);
});

test("rejects zero matched tests even when go exits zero", () => {
  const result = verifyGoE2EEvents({ text: `${event("pass", undefined)}\n`, goExitCode: 0, requireAny: true });
  assert.equal(result.ok, false);
  assert.match(result.errors.join("\n"), /zero passing test events/);
});

test("rejects a skipped child even when its parent passes", () => {
  const result = verifyGoE2EEvents({
    text: `${event("skip", "TestRequired/child")}\n${event("pass", "TestRequired")}\n`,
    goExitCode: 0,
    expectedTests: ["TestRequired"],
  });
  assert.equal(result.ok, false);
  assert.match(result.errors.join("\n"), /skipped test event/);
});

test("rejects a fail event even if a wrapper reports exit zero", () => {
  const result = verifyGoE2EEvents({
    text: `${event("fail", "TestRequired/child")}\n${event("pass", "TestRequired")}\n`,
    goExitCode: 0,
    expectedTests: ["TestRequired"],
  });
  assert.equal(result.ok, false);
  assert.match(result.errors.join("\n"), /failed test event/);
});

test("rejects a package FAIL hidden behind exit zero", () => {
  const result = verifyGoE2EEvents({
    text: `${event("pass", "TestRequired")}\n${event("fail", undefined, "FAIL\n")}\n`,
    goExitCode: 0,
    expectedTests: ["TestRequired"],
  });
  assert.equal(result.ok, false);
  assert.match(result.errors.join("\n"), /package terminal failure/);
  assert.match(result.errors.join("\n"), /explicit FAIL/);
});

test("rejects a missing expected test when another test passes", () => {
  const result = verifyGoE2EEvents({
    text: `${event("pass", "TestDifferent")}\n`,
    goExitCode: 0,
    expectedTests: ["TestRequired"],
  });
  assert.equal(result.ok, false);
  assert.match(result.errors.join("\n"), /missing required passing test/);
});

test("rejects explicit BLOCKED output and a nonzero go exit", () => {
  const result = verifyGoE2EEvents({
    text: `${event("output", "TestRequired", "BLOCKED: provider unavailable\\n")}\n${event("pass", "TestRequired")}\n`,
    goExitCode: 1,
    expectedTests: ["TestRequired"],
  });
  assert.equal(result.ok, false);
  assert.match(result.errors.join("\n"), /BLOCKED/);
  assert.match(result.errors.join("\n"), /go exit code was 1/);
});
