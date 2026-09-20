#!/usr/bin/env node

import fs from "node:fs";
import process from "node:process";

function usage() {
  console.error("usage: collect-go-test-events.mjs <go-test-jsonl> <exit-code> <required-json> <output-json>");
  process.exit(2);
}

const [eventsPath, exitCodeText, requiredPath, outputPath] = process.argv.slice(2);
if (!eventsPath || !exitCodeText || !requiredPath || !outputPath) usage();

const actual = new Map();
const malformed = [];
for (const [lineNumber, line] of fs.readFileSync(eventsPath, "utf8").split(/\r?\n/).entries()) {
  if (!line.trim()) continue;
  let event;
  try {
    event = JSON.parse(line);
  } catch (error) {
    malformed.push({ line: lineNumber + 1, error: error.message });
    continue;
  }
  if (!event.Test || !["pass", "fail", "skip"].includes(event.Action)) continue;
  const key = `${event.Package}\u001f${event.Test}`;
  actual.set(key, {
    package: event.Package,
    test: event.Test,
    action: event.Action,
    elapsed: event.Elapsed ?? null,
  });
}

const expected = JSON.parse(fs.readFileSync(requiredPath, "utf8"));
const result = {
  command: "go test -json",
  exitCode: Number.parseInt(exitCodeText, 10),
  required: expected.required ?? [],
  conditional: expected.conditional ?? [],
  actual: [...actual.values()],
  malformedEventLines: malformed,
};
fs.writeFileSync(outputPath, `${JSON.stringify(result, null, 2)}\n`);
if (result.exitCode !== 0 || malformed.length > 0) process.exit(1);
