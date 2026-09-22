#!/usr/bin/env node

import fs from "node:fs";
import { pathToFileURL } from "node:url";
import process from "node:process";

function usage() {
  console.error(
    "usage: verify-go-e2e-events.mjs <events.jsonl> <go-exit-code> <summary.json> (--require-any | <expected-test>...)",
  );
  process.exit(2);
}

export function verifyGoE2EEvents({ text, goExitCode, expectedTests = [], requireAny = false }) {
  const terminal = new Map();
  const malformedLines = [];
  const blockedLines = [];
  const explicitFailLines = [];
  const nonTestTerminalFailures = [];

  for (const [index, line] of text.split(/\r?\n/).entries()) {
    if (!line.trim()) continue;
    let event;
    try {
      event = JSON.parse(line);
    } catch (error) {
      malformedLines.push({ line: index + 1, error: error.message });
      continue;
    }
    if (typeof event.Output === "string" && /\bBLOCKED\b/i.test(event.Output)) {
      blockedLines.push(index + 1);
    }
    if (typeof event.Output === "string" && /(^|\n)(--- FAIL:|FAIL(?:\s|$))/m.test(event.Output)) {
      explicitFailLines.push(index + 1);
    }
    if (!event.Test && ["fail", "skip"].includes(event.Action)) {
      nonTestTerminalFailures.push({ package: event.Package ?? "", action: event.Action });
    }
    if (event.Test && ["pass", "fail", "skip"].includes(event.Action)) {
      terminal.set(event.Test, {
        package: event.Package ?? "",
        test: event.Test,
        action: event.Action,
        elapsed: event.Elapsed ?? null,
      });
    }
  }

  const actual = [...terminal.values()];
  const failed = actual.filter((entry) => entry.action === "fail");
  const skipped = actual.filter((entry) => entry.action === "skip");
  const missing = expectedTests.filter((name) => terminal.get(name)?.action !== "pass");
  const passed = actual.filter((entry) => entry.action === "pass");
  const errors = [];

  if (!Number.isInteger(goExitCode) || goExitCode !== 0) errors.push(`go exit code was ${goExitCode}`);
  if (malformedLines.length > 0) errors.push(`${malformedLines.length} malformed JSON event line(s)`);
  if (blockedLines.length > 0) errors.push(`BLOCKED appeared in output on line(s) ${blockedLines.join(",")}`);
  if (explicitFailLines.length > 0) errors.push(`explicit FAIL appeared in output on line(s) ${explicitFailLines.join(",")}`);
  if (nonTestTerminalFailures.length > 0) {
    errors.push(
      `package terminal failure(s): ${nonTestTerminalFailures.map((entry) => `${entry.package || "<unknown>"}:${entry.action}`).join(", ")}`,
    );
  }
  if (failed.length > 0) errors.push(`failed test event(s): ${failed.map((entry) => entry.test).join(", ")}`);
  if (skipped.length > 0) errors.push(`skipped test event(s): ${skipped.map((entry) => entry.test).join(", ")}`);
  if (missing.length > 0) errors.push(`missing required passing test event(s): ${missing.join(", ")}`);
  if (requireAny && passed.length === 0) errors.push("zero passing test events");
  if (!requireAny && expectedTests.length === 0) errors.push("no expected tests were supplied");

  return {
    goExitCode,
    requireAny,
    expectedTests,
    actual,
    malformedLines,
    blockedLines,
    explicitFailLines,
    nonTestTerminalFailures,
    errors,
    ok: errors.length === 0,
  };
}

function main() {
  const [eventsPath, exitCodeText, summaryPath, ...requirements] = process.argv.slice(2);
  if (!eventsPath || exitCodeText === undefined || !summaryPath || requirements.length === 0) usage();
  const requireAny = requirements.length === 1 && requirements[0] === "--require-any";
  if (!requireAny && requirements.includes("--require-any")) usage();
  const goExitCode = Number.parseInt(exitCodeText, 10);
  const result = verifyGoE2EEvents({
    text: fs.readFileSync(eventsPath, "utf8"),
    goExitCode,
    expectedTests: requireAny ? [] : requirements,
    requireAny,
  });
  fs.writeFileSync(summaryPath, `${JSON.stringify(result, null, 2)}\n`);
  if (!result.ok) {
    for (const error of result.errors) console.error(`E2E evidence rejected: ${error}`);
    process.exit(1);
  }
  console.log(`E2E evidence accepted: ${result.actual.filter((entry) => entry.action === "pass").length} passing event(s)`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) main();
