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
  assert.match(store, /diagnosticsSourceEpochs: \{ lifecycle: "", events: "", modelRuns: "", mediaPrompts: "" \}/);
  assert.match(store, /diagnosticsSourceEpochs\.lifecycle !== epochs\.lifecycle/);
  assert.match(store, /diagnosticsSourceEpochs\.events === epochs\.events/);
  assert.match(store, /diagnosticsSourceEpochs\.modelRuns === epochs\.modelRuns/);
  assert.match(store, /requestId !== this\.diagnosticsRequestId/);
  assert.match(store, /error\.details\.correlation_id/);
  assert.match(store, /error\.code, correlationId/);
});

test("background diagnostics refresh does not request the workflow runtime list", () => {
  const refreshStart = store.indexOf("async loadDiagnostics()");
  const workflowStart = store.indexOf("async loadWorkflows()", refreshStart);
  const refreshEnd = store.indexOf("async exportDiagnostics()", workflowStart);
  assert.ok(refreshStart >= 0 && workflowStart > refreshStart && refreshEnd > workflowStart);
  assert.doesNotMatch(store.slice(refreshStart, workflowStart), /client\.listWorkflows\(/);
  assert.match(view, /currentSection\.value === "workflows"/);
  assert.match(view, /watch\(currentSection, \(section\) => \{\s*if \(section === "workflows"\) void controlCenter\.loadWorkflows\(\)/);
  assert.match(view, /刷新工作流列表/);
  assert.match(view, /@click="controlCenter\.loadWorkflows\(\)"/);
  const workflowLoader = store.slice(workflowStart, refreshEnd);
  assert.match(workflowLoader, /if \(this\.workflowListLoading\) return/);
  assert.match(workflowLoader, /client\.listWorkflows\(/);
  assert.match(view, /controlCenter\.diagnosticsWarning/);
  const clearStart = store.indexOf("async clearDiagnostics()");
  const clearEnd = store.indexOf("async retryMediaPrompt(", clearStart);
  assert.ok(clearStart > refreshEnd && clearEnd > clearStart);
  assert.doesNotMatch(store.slice(clearStart, clearEnd), /workflowListLoading|this\.workflows = \[\]/);
});

test("workflow list loader coalesces pending requests and keeps its failure local", async () => {
  const start = store.indexOf("async loadWorkflows()");
  const end = store.indexOf("async exportDiagnostics()", start);
  assert.ok(start >= 0 && end > start);
  const pending = [];
  const client = {
    listWorkflows: () => new Promise((resolve, reject) => pending.push({ resolve, reject })),
  };
  const loadWorkflows = new Function("client", `return ({${store.slice(start, end)}}).loadWorkflows`)(client);
  const state = { workflowListLoading: false, diagnosticsWarning: "", workflows: [] };

  const first = loadWorkflows.call(state);
  const duplicate = loadWorkflows.call(state);
  assert.equal(pending.length, 1);
  assert.equal(state.workflowListLoading, true);
  pending[0].resolve([{ workflow_id: "workflow-1" }]);
  await Promise.all([first, duplicate]);
  assert.deepEqual(state.workflows, [{ workflow_id: "workflow-1" }]);
  assert.equal(state.workflowListLoading, false);

  const retry = loadWorkflows.call(state);
  assert.equal(pending.length, 2);
  pending[1].reject(new Error("Temporal unavailable"));
  await retry;
  assert.deepEqual(state.workflows, []);
  assert.match(state.diagnosticsWarning, /工作流运行时暂不可用/);
  assert.equal(state.workflowListLoading, false);
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
