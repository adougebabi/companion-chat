import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";
import { computed, ref, reactive, watch, nextTick } from "vue";

const component = await readFile(new URL("../src/components/instances/GoalPanel.vue", import.meta.url), "utf8");
const script = component.match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1].replace(/^import .*;\n/gm, "");
const js = ts.transpileModule(script, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.None } }).outputText;
class ApiError extends Error { constructor(status, code = "revision_conflict") { super("conflict"); this.status = status; this.code = code; } }
const goal = (id, revision = 1) => ({ id, revision, status: "active", desired_outcome: id, motivation: "original", scope: "general", success_criteria: ["actual result"], execution: {} });
const deferred = () => { let resolve, reject; const promise = new Promise((ok, fail) => { resolve = ok; reject = fail; }); return { promise, resolve, reject }; };
async function harness(overrides = {}) {
  const props = reactive({ fluctlightId: "instance-a", readOnly: false });
  const client = { goals: async () => ({ items: [], next_cursor: "" }), goalDetail: async (_, id) => goal(id), goalHistory: async () => ({ items: [], next_cursor: "older" }), ...overrides };
  const create = new Function("ref", "computed", "watch", "defineProps", "BrowserClient", "BrowserApiError", "apiOrigin", `${js}; return {load,inspect,edit,command,moreEvidence,evidenceCursor,evidenceBusy,moreRevisions,reloadDraftAuthority,selected,revisions,revisionCursor,revisionBusy,error,editing,desired,reason,frozenRevision,historyTab};`);
  const state = create(ref, computed, watch, () => props, function () { return client; }, ApiError, "");
  await nextTick(); await state.load();
  return { props, client, state };
}

test("instance switch releases pending revision pagination and discards its stale failure", async () => {
  const older = deferred();
  const { props, state } = await harness({ goalHistory: async (_, id, cursor) => cursor ? older.promise : { items: [{ id: `${id}-current` }], next_cursor: "older" } });
  await state.inspect(goal("goal-a"));
  const pending = state.moreRevisions();
  assert.equal(state.revisionBusy.value, true);
  props.fluctlightId = "instance-b";
  await nextTick(); await state.load();
  await state.inspect(goal("goal-b"));
  older.reject(new Error("old instance unavailable"));
  await pending;
  assert.equal(state.revisionBusy.value, false);
  assert.equal(state.error.value, "");
  assert.deepEqual(state.revisions.value, [{ id: "goal-b-current" }]);
});

test("refreshing the same Goal fences an old revision page while a new page is pending", async () => {
  const oldPage = deferred(), newPage = deferred(); let pages = 0;
  const { state } = await harness({ goalHistory: async (_, id, cursor) => cursor ? (++pages === 1 ? oldPage.promise : newPage.promise) : { items: [{ id: `${id}-current` }], next_cursor: "older" } });
  await state.inspect(goal("same"));
  const oldPending = state.moreRevisions();
  await state.inspect(goal("same"));
  const newPending = state.moreRevisions();
  oldPage.resolve({ items: [{ id: "stale" }], next_cursor: "stale-cursor" });
  await oldPending;
  assert.equal(state.revisionBusy.value, true);
  assert.deepEqual(state.revisions.value, [{ id: "same-current" }]);
  newPage.resolve({ items: [{ id: "same-current" }, { id: "older-valid" }], next_cursor: "" });
  await newPending;
  assert.equal(state.revisionBusy.value, false);
  assert.deepEqual(state.revisions.value, [{ id: "same-current" }, { id: "older-valid" }]);
});

test("CAS conflict preserves the draft, frozen revision and retry identity until explicit reread", async () => {
  const calls = [];
  const { state, client } = await harness({ goalDetail: async (_, id) => goal(id, 7), goalCommand: async (...args) => { calls.push(args); throw new ApiError(409); } });
  await state.inspect(goal("edited")); state.edit();
  state.desired.value = "my retained draft"; state.reason.value = "clarify";
  await state.command("update"); await state.command("update");
  assert.equal(state.editing.value, true);
  assert.equal(state.desired.value, "my retained draft");
  assert.equal(state.frozenRevision.value, 7);
  assert.equal(calls[0][3].expectedRevision, 7);
  assert.equal(calls[0][3].idempotencyKey, calls[1][3].idempotencyKey);
  client.goalDetail = async (_, id) => goal(id, 8);
  await state.reloadDraftAuthority();
  assert.equal(state.frozenRevision.value, 8);
  assert.equal(state.desired.value, "my retained draft");
  await state.command("update");
  assert.equal(calls[2][3].expectedRevision, 8);
  assert.notEqual(calls[2][3].idempotencyKey, calls[0][3].idempotencyKey);
});


test("capacity errors preserve a create draft and explain the available governance actions", async () => {
 const {state} = await harness({createGoal:async()=>{throw new ApiError(409,"goal_capacity_exceeded");}});
 state.edit(true);state.desired.value="retained sixth goal";state.reason.value="create";
 await state.command("create");
 assert.equal(state.editing.value,true);
 assert.equal(state.desired.value,"retained sixth goal");
 assert.match(state.error.value,/名额已满/);
 assert.match(state.error.value,/候选/);
 assert.doesNotMatch(state.error.value,/目标已发生变化/);
});

test("evidence continuation deduplicates stable IDs and rejects a stale selection", async () => {
 const pending = deferred();
 const {state} = await harness({goalDetail:async(_,id)=>({...goal(id),evidence:[{id:id+"-proof"}],evidence_next_cursor:"older"}),goalEvidence:async()=>pending.promise});
 await state.inspect(goal("a"));
 const oldPage=state.moreEvidence();
 await state.inspect(goal("b"));
 pending.resolve({items:[{id:"stale"}],next_cursor:"stale-cursor"});
 await oldPage;
 assert.deepEqual(state.selected.value.evidence,[{id:"b-proof"}]);
 assert.equal(state.evidenceCursor.value,"older");
 assert.equal(state.evidenceBusy.value,false);
});
