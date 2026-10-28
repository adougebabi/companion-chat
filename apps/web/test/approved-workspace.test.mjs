import test from "node:test";

import assert from "node:assert/strict";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";
test("approved workspace drops late reads, recovers original commands after switching, and checkpoints request observers", async () => {
  const previous = {
    window: globalThis.window,
    document: globalThis.document,
    location: globalThis.location,
    localStorage: globalThis.localStorage,
    sessionStorage: globalThis.sessionStorage,
  };
  const values = new Map(),
    saved = {
      getItem: (k) => values.get(k) ?? null,
      setItem: (k, v) => values.set(k, String(v)),
      removeItem: (k) => values.delete(k),
    };
  globalThis.window = {
    location: { origin: "http://scope.test" },
    addEventListener() {},
    removeEventListener() {},
  };
  globalThis.document = new EventTarget();
  document.hidden = false;
  globalThis.location = { hash: "" };
  globalThis.localStorage = saved;
  globalThis.sessionStorage = saved;
  const vite = await createServer({
    root: fileURLToPath(new URL("../", import.meta.url)),
    appType: "custom",
    logLevel: "silent",
    server: { middlewareMode: true },
  });
  let workspace;
  try {
    const { createWorkspace } = await vite.ssrLoadModule(
        "/src/state/workspace.mjs",
      ),
      store = await vite.ssrLoadModule("/src/v3/proto/store.js"),
      services = await vite.ssrLoadModule("/src/v3/proto/services.js");
    workspace = createWorkspace();
    store.replaceSnapshot({
      ...store.get(),
      session: { actorId: "owner" },
      characters: { A: { id: "A", name: "A" }, B: { id: "B", name: "B" } },
      ui: { charId: "A" },
    });
    let release;
    workspace.client.detail = async () =>
      new Promise((resolve) => (release = resolve));
    const reading = workspace.detail();
    workspace.select("B");
    release({ identity: { occupation: "stale-A" } });
    await reading;
    assert.equal(workspace.current.value.job, "");
    assert.equal(workspace.current.value.id, "B");
    workspace.select("A");
    let reads = 0;
    const deferred = [];
    workspace.client.detail = async () =>
      new Promise((resolve) => deferred.push(resolve));
    const first = workspace.detail(),
      second = workspace.detail();
    deferred[1]({ identity: { occupation: "newest" } });
    await second;
    deferred[0]({ identity: { occupation: "older" } });
    await first;
    assert.equal(workspace.current.value.job, "newest");
    const calls = [];
    const command = workspace.command(
      "世界",
      "edit:A",
      { expectedRevision: 1, text: "original" },
      async (body, key) => {
        calls.push({ body, key });
        if (calls.length === 1) throw Error("unknown result");
        return { revision: 2 };
      },
    );
    await assert.rejects(command.completion, /unknown/);
    workspace.select("B");
    workspace.select("A");
    await command.run();
    assert.equal(command.status, "success");
    assert.deepEqual(calls[0], calls[1]);
    const staged = workspace.command(
      "世界",
      "expansion",
      { worldId: "world-A" },
      async () => {
        const error = new Error("apply validation after source committed");
        error.status = 422;
        throw error;
      },
      { multiStage: true },
    );
    await assert.rejects(staged.completion);
    assert.ok(
      [...values.keys()].some(
        (k) => k.includes("approved-command:") && k.includes("expansion"),
      ),
      "a later-stage validation cannot release the parent identity",
    );
    const stale = workspace.command("角色详情", "profile:A", { expectedRevision: 1 }, async () => {
      throw Object.assign(new Error("stale proposal"), { status: 409, code: "foundation_proposal_stale" });
    }, { preAdmissionConflicts: ["revision_conflict", "foundation_proposal_stale"] });
    await assert.rejects(stale.completion, /stale proposal/);
    const staleKey = [...values.keys()].find((key) => key.includes('approved-command:') && key.includes('profile:A'));
    assert.equal(JSON.parse(values.get(staleKey)).rejection.code, "foundation_proposal_stale");
    services.configureDomainService("life", {
      request: async () => {
        reads++;
        return {
          id: "request-A",
          fluctlight_id: "A",
          status: "failed",
          decision_reason: "actual failure",
        };
      },
    });
    document.hidden = true;
    const accepted = workspace.command(
      "换装",
      "wear:A",
      { outfitId: "saved" },
      async () => ({ request: { id: "request-A", status: "pending" } }),
    );
    await accepted.completion;
    assert.equal(reads, 0);
    assert.ok(
      [...values.keys()].some((k) =>
        k.startsWith("fluctlight:approved-receipts:"),
      ),
    );
    assert.equal(
      [...values.keys()].some(
        (k) => k.includes("approved-command:") && k.includes("wear:A"),
      ),
      false,
    );
    document.hidden = false;
    document.dispatchEvent(new Event("visibilitychange"));
    await new Promise((resolve) => setTimeout(resolve, 0));
    assert.equal(reads, 1);
    assert.equal(accepted.status, "error");
    accepted.run();
    assert.equal(location.hash, "#/diag/request/request-A");
    assert.equal(reads, 1);
  } finally {
    workspace?.dispose();
    await vite.close();
    Object.assign(globalThis, previous);
  }
});

test('workspace starts an Owner-wide count without Inbox navigation and retains it through role selection',async()=>{
 const {installDOM}=await import('./helpers/react-dom.mjs');const env=installDOM();
 const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});let w;
 try{
  const {createWorkspace}=await vite.ssrLoadModule('/src/state/workspace.mjs'),store=await vite.ssrLoadModule('/src/v3/proto/store.js'),services=await vite.ssrLoadModule('/src/v3/proto/services.js');
  let release,reads=0;services.configureDomainService('operations',{notificationCount:()=>{reads++;return new Promise(r=>release=r);}});
  w=createWorkspace();store.replaceSnapshot({...store.get(),session:{actorId:'counter-owner'},characters:{A:{id:'A',name:'甲'},B:{id:'B',name:'乙'}},ui:{charId:'A'}});
  assert.equal(reads,1);assert.equal(w.state.notificationCount,null);w.select('B');release({count:61,alert_count:3});await new Promise(r=>setTimeout(r,0));
  assert.equal(w.state.notificationCount.count,61);assert.equal(w.state.notifications.length,0);assert.equal(w.current.value.id,'B');
 }finally{w?.dispose();await vite.close();env.restore();}
});
