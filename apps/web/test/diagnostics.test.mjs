import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const view = await readFile(new URL("../src/views/DiagnosticsView.vue", import.meta.url), "utf8");
const store = await readFile(new URL("../src/stores/control-center.ts", import.meta.url), "utf8");
const navigation = await readFile(new URL("../src/app/navigation.ts", import.meta.url), "utf8");
const browserClient = await readFile(new URL("../../../packages/browser-client/src/index.ts", import.meta.url), "utf8");

test("Lifecycle Diagnostics exposes typed filters and PostgreSQL intent snapshots", () => {
  assert.match(navigation, /DiagnosticsSection = "lifecycle"/);
  assert.match(browserClient, /BrowserLifecycleDiagnosticsFilter/);
  assert.match(browserClient, /BrowserWorkflowIntentSnapshot/);
  assert.match(browserClient, /lifecycleDiagnostics\(options: BrowserLifecycleDiagnosticsFilter/);
  for (const field of ["diagnosticsFluctlightFilter", "diagnosticsCorrelationFilter", "diagnosticsIntentFilter", "diagnosticsWorkflowFilter", "diagnosticsRunFilter", "diagnosticsSurfaceFilter", "diagnosticsStatusFilter"]) {
    assert.match(store, new RegExp(field));
  }
  assert.match(store, /workflowIntentSnapshots/);
  assert.match(view, /PostgreSQL Intent 快照/);
});

test("diagnostics sources use independent filter epochs", () => {
  assert.match(store, /diagnosticsSourceEpochs: \{ lifecycle: "", events: "", modelRuns: "", mediaPrompts: "", workflows: "" \}/);
  assert.match(store, /diagnosticsSourceEpochs\.lifecycle !== epochs\.lifecycle/);
  assert.match(store, /diagnosticsSourceEpochs\.events === epochs\.events/);
  assert.match(store, /diagnosticsSourceEpochs\.modelRuns === epochs\.modelRuns/);
  assert.match(store, /requestId !== this\.diagnosticsRequestId/);
  assert.match(store, /error\.details\.correlation_id/);
  assert.match(store, /error\.code, correlationId/);
});

test("Lifecycle Diagnostics renders empty, overdue, no-op, retry and failure states safely on narrow screens", () => {
  assert.match(view, /currentSection === 'lifecycle'.*!controlCenter\.lifecycleDiagnostics\.length/s);
  assert.match(view, /run-overdue/);
  assert.match(view, /run-no_op/);
  assert.match(view, /run-retry/);
  assert.match(view, /run-failed/);
  assert.match(view, /overflow-wrap: anywhere/);
  assert.match(view, /@media \(max-width: 760px\)[\s\S]*grid-template-columns: 1fr/);
  assert.doesNotMatch(view, /v-html/);
});

test("filtered export reuses every active lifecycle filter", () => {
  const exportStart = store.indexOf("async exportDiagnostics()");
  const exportEnd = store.indexOf("async queryWorkflowStatus", exportStart);
  assert.ok(exportStart >= 0 && exportEnd > exportStart);
  const body = store.slice(exportStart, exportEnd);
  for (const field of ["correlationId", "fluctlightId", "intentId", "workflowId", "runId", "surface", "status"]) {
    assert.match(body, new RegExp(`${field}:`));
  }
});
