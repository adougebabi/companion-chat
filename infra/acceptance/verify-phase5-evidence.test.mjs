import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { parseCsv, validateEvidence } from "./verify-phase5-evidence.mjs";

const header = "id,stage,requirement,scope,implementation,test,evidence,status,notes";
const row = (status = "PASS", evidence = "evidence/test.txt") =>
  `S1-TEST-01,S1,requirement,scope,src/file.go,TestThing,${evidence},${status},a concrete reason for this state`;

function fixture(matrix, executions = null) {
  const runDir = fs.mkdtempSync(path.join(os.tmpdir(), "fluctlight-g0-"));
  fs.mkdirSync(path.join(runDir, "evidence"), { recursive: true });
  fs.writeFileSync(path.join(runDir, "acceptance-matrix.csv"), `${matrix}\n`);
  fs.writeFileSync(path.join(runDir, "evidence", "test.txt"), "synthetic evidence\n");
  let executionsPath = null;
  if (executions) {
    executionsPath = path.join(runDir, "executions.json");
    fs.writeFileSync(executionsPath, JSON.stringify(executions));
  }
  return { runDir, executionsPath };
}

test("G0 rejects CSV column drift", () => {
  const { runDir } = fixture(`${header}\n${row()}`.replace("notes", "note"));
  const errors = validateEvidence({ runDir, repositoryRoot: runDir, matrixPath: path.join(runDir, "acceptance-matrix.csv") });
  assert.match(errors.join("\n"), /header mismatch/);
});

test("G0 rejects missing evidence and invalid status", () => {
  const { runDir } = fixture(`${header}\n${row("MAYBE", "evidence/missing.txt")}`);
  const errors = validateEvidence({ runDir, repositoryRoot: runDir, matrixPath: path.join(runDir, "acceptance-matrix.csv") });
  assert.match(errors.join("\n"), /invalid status/);
  assert.match(errors.join("\n"), /evidence path missing or outside run directory/);
});

test("G0 rejects evidence references outside the run directory and malformed stages", () => {
  const { runDir } = fixture(`${header}\n${row("PASS", "../outside.txt")}`);
  const matrixPath = path.join(runDir, "acceptance-matrix.csv");
  fs.appendFileSync(matrixPath, "");
  const text = fs.readFileSync(matrixPath, "utf8").replace("S1,", "S9,");
  fs.writeFileSync(matrixPath, text);
  const errors = validateEvidence({ runDir, repositoryRoot: runDir, matrixPath });
  assert.match(errors.join("\n"), /invalid stage/);
  assert.match(errors.join("\n"), /outside run directory/);
});

test("G0 rejects parent pass with required child skip", () => {
  const { runDir, executionsPath } = fixture(`${header}\n${row()}`, {
    exitCode: 0,
    required: [{ package: "pkg", test: "TestParent/required-child" }],
    actual: [{ package: "pkg", test: "TestParent", action: "pass" }, { package: "pkg", test: "TestParent/required-child", action: "skip" }],
  });
  const errors = validateEvidence({ runDir, repositoryRoot: runDir, matrixPath: path.join(runDir, "acceptance-matrix.csv"), executionsPath });
  assert.match(errors.join("\n"), /action=skip/);
});

test("G0 rejects a failed process hidden behind a successful summary", () => {
  const { runDir, executionsPath } = fixture(`${header}\n${row()}`, {
    exitCode: 1,
    required: [{ package: "pkg", test: "TestThing" }],
    actual: [{ package: "pkg", test: "TestThing", action: "pass" }],
  });
  const errors = validateEvidence({ runDir, repositoryRoot: runDir, matrixPath: path.join(runDir, "acceptance-matrix.csv"), executionsPath });
  assert.match(errors.join("\n"), /exitCode=1/);
});
