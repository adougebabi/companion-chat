#!/usr/bin/env node

import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

export const REQUIRED_COLUMNS = [
  "id",
  "stage",
  "requirement",
  "scope",
  "implementation",
  "test",
  "evidence",
  "status",
  "notes",
];
export const STATUSES = new Set(["PASS", "FAIL", "BLOCKED", "NOT_RUN", "NOT_APPLICABLE"]);

export function parseCsv(text) {
  const rows = [];
  let row = [];
  let cell = "";
  let quoted = false;
  for (let index = 0; index < text.length; index += 1) {
    const char = text[index];
    const next = text[index + 1];
    if (quoted) {
      if (char === '"' && next === '"') {
        cell += '"';
        index += 1;
      } else if (char === '"') {
        quoted = false;
      } else {
        cell += char;
      }
      continue;
    }
    if (char === '"' && cell === "") {
      quoted = true;
    } else if (char === ",") {
      row.push(cell);
      cell = "";
    } else if (char === "\n") {
      row.push(cell.replace(/\r$/, ""));
      rows.push(row);
      row = [];
      cell = "";
    } else {
      cell += char;
    }
  }
  if (quoted) throw new Error("CSV contains an unterminated quoted field");
  if (cell !== "" || row.length > 0) {
    row.push(cell.replace(/\r$/, ""));
    rows.push(row);
  }
  return rows.filter((candidate) => !(candidate.length === 1 && candidate[0] === ""));
}

function isPathLike(value) {
  return value.includes("/") || /\.(txt|md|json|csv|go|ts|mjs|sh|yml|yaml)$/.test(value);
}

function resolveEvidencePath(runDir, repositoryRoot, value) {
  const candidate = value.trim();
  if (!candidate || !isPathLike(candidate)) return null;
  if (path.isAbsolute(candidate)) return null;
  const local = path.resolve(runDir, candidate);
  const relative = path.relative(runDir, local);
  if (relative.startsWith(`..${path.sep}`) || path.isAbsolute(relative)) return null;
  if (fs.existsSync(local) && fs.statSync(local).isFile()) return local;
  return null;
}

function splitEvidence(value) {
  return value.split(";").map((item) => item.trim()).filter(Boolean);
}

export function validateEvidence({ runDir, repositoryRoot, matrixPath, executionsPath = null, requireClear = false }) {
  const errors = [];
  if (!fs.existsSync(matrixPath)) {
    errors.push(`matrix missing: ${matrixPath}`);
    return errors;
  }
  let rows;
  try {
    rows = parseCsv(fs.readFileSync(matrixPath, "utf8"));
  } catch (error) {
    errors.push(error.message);
    return errors;
  }
  if (rows.length < 2) errors.push("matrix must contain a header and at least one data row");
  const header = rows[0] ?? [];
  if (header.join("\u001f") !== REQUIRED_COLUMNS.join("\u001f")) {
    errors.push(`matrix header mismatch: expected ${REQUIRED_COLUMNS.join(",")}`);
  }
  const ids = new Set();
  for (let index = 1; index < rows.length; index += 1) {
    const row = rows[index];
    if (row.length !== REQUIRED_COLUMNS.length) {
      errors.push(`matrix row ${index + 1} has ${row.length} columns, expected ${REQUIRED_COLUMNS.length}`);
      continue;
    }
    const item = Object.fromEntries(REQUIRED_COLUMNS.map((key, column) => [key, row[column].trim()]));
    if (!item.id) errors.push(`matrix row ${index + 1} has empty id`);
    if (ids.has(item.id)) errors.push(`duplicate matrix id: ${item.id}`);
    ids.add(item.id);
    if (!/^(?:G[0-3]|F|R|S[1-4]|CROSS)$/.test(item.stage)) errors.push(`invalid stage for ${item.id}: ${item.stage}`);
    if (!STATUSES.has(item.status)) errors.push(`invalid status for ${item.id}: ${item.status}`);
    for (const field of ["requirement", "scope", "implementation", "test", "evidence", "notes"]) {
      if (!item[field]) errors.push(`${item.id} has empty ${field}`);
    }
    const evidenceRefs = splitEvidence(item.evidence);
    if (evidenceRefs.length === 0) errors.push(`${item.id} has no evidence references`);
    for (const ref of evidenceRefs) {
      if (isPathLike(ref) && !resolveEvidencePath(runDir, repositoryRoot, ref)) {
        errors.push(`${item.id} evidence path missing or outside run directory: ${ref}`);
      }
    }
    if (item.status === "PASS") {
      if (!evidenceRefs.some((ref) => isPathLike(ref) && resolveEvidencePath(runDir, repositoryRoot, ref))) {
        errors.push(`${item.id} PASS has no existing path evidence`);
      }
    }
    if (["BLOCKED", "NOT_RUN", "FAIL"].includes(item.status) && item.notes.length < 12) {
      errors.push(`${item.id} ${item.status} needs a concrete reason in notes`);
    }
  }

  if (executionsPath) {
    if (!fs.existsSync(executionsPath)) {
      errors.push(`execution result missing: ${executionsPath}`);
    } else {
      let executions;
      try {
        executions = JSON.parse(fs.readFileSync(executionsPath, "utf8"));
      } catch (error) {
        errors.push(`execution result is not JSON: ${error.message}`);
        executions = null;
      }
      if (executions) {
        if (executions.exitCode !== 0) errors.push(`execution command exitCode=${executions.exitCode}`);
        const required = executions.required ?? [];
        const actual = executions.actual ?? [];
        for (const expected of required) {
          const matches = actual.filter((item) => item.package === expected.package && item.test === expected.test);
          if (matches.length !== 1) {
            errors.push(`required test ${expected.package} ${expected.test} matched ${matches.length} times`);
          } else if (matches[0].action !== "pass") {
            errors.push(`required test ${expected.package} ${expected.test} action=${matches[0].action}`);
          }
        }
        for (const expected of executions.conditional ?? []) {
          if (!expected.reason || String(expected.reason).trim().length < 12) {
            errors.push(`conditional test ${expected.package} ${expected.test} needs a concrete reason`);
            continue;
          }
          const matches = actual.filter((item) => item.package === expected.package && item.test === expected.test);
          if (matches.length !== 1) {
            errors.push(`conditional test ${expected.package} ${expected.test} matched ${matches.length} times`);
          } else if (!["pass", "skip"].includes(matches[0].action)) {
            errors.push(`conditional test ${expected.package} ${expected.test} action=${matches[0].action}`);
          }
        }
      }
    }
  }
  if (requireClear) {
    const blocked = rows.slice(1).filter((row) => ["FAIL", "BLOCKED", "NOT_RUN"].includes(row[7])).map((row) => row[0]);
    if (blocked.length > 0) errors.push(`clear gate blocked by statuses: ${blocked.join(",")}`);
  }
  return errors;
}

function main() {
  const args = process.argv.slice(2);
  const runDir = path.resolve(args[0] ?? "");
  if (!runDir || !fs.existsSync(runDir)) {
    console.error("usage: verify-phase5-evidence.mjs <run-dir> [--require-clear] [--executions file]");
    process.exit(2);
  }
  const repositoryRoot = path.resolve(runDir, "../../../../..");
  const errors = validateEvidence({
    runDir,
    repositoryRoot,
    matrixPath: path.join(runDir, "acceptance-matrix.csv"),
    executionsPath: args.includes("--executions") ? path.resolve(args[args.indexOf("--executions") + 1]) : null,
    requireClear: args.includes("--require-clear"),
  });
  if (errors.length > 0) {
    console.error(errors.map((error) => `G0: ${error}`).join("\n"));
    process.exit(1);
  }
  console.log(`G0 evidence validation: PASS (${runDir})`);
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main();
