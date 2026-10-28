import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";
import { act, installDOM, mountReact } from "./helpers/react-dom.mjs";
import { creationFixture, validAnalysis } from "./support/creation-fixture.mjs";
const env = installDOM(),
  vite = await createServer({
    root: fileURLToPath(new URL("../", import.meta.url)),
    appType: "custom",
    logLevel: "silent",
    server: { middlewareMode: true },
  }),
  { default: View } = await vite.ssrLoadModule(
    "/src/react-app/views/CharacterWizard.jsx",
  );
test.after(async () => {
  await vite.close();
  env.restore();
});
const button = (root, text) =>
    [...root.querySelectorAll("button")].find(
      (n) => n.textContent.trim() === text,
    ),
  dialog = () => [...document.querySelectorAll("[role=dialog]")].at(-1);
async function preview() {
  const source = validAnalysis();
  source.core_persona.identity.name = "原名字";
  source.core_persona.life_profile = {
    custom_extension: null,
    life_habits: "阅读",
    wardrobe_items: [],
  };
  const f = creationFixture(async () => source);
  f.c.draft.bio = "一段描述";
  await f.c.analyze();
  const m = {
    c: f.c,
    draft: f.c.draft,
    step: "preview",
    busy: false,
    pending: null,
    currentStep: 2,
    previewFields: f.c.previewFields.value,
    error: "",
    fieldErrors: {},
    groups: [],
    existingWorlds: [],
    worlds: [],
    templates: [],
    canContinue: true,
    showError: (message) => (f.c.error.value = message),
    prev() {},
    goStep() {},
    newDraft() {},
    configHref: () => "#/settings",
  };
  const v = await mountReact(vite, View, { props: { model: m } });
  return { ...f, v, m };
}
async function selectTab(f, name) {
  await act(async () =>
    [...f.v.host.querySelectorAll("[role=tab]")]
      .find((n) => n.textContent.trim() === name)
      .dispatchEvent(
        new window.MouseEvent("mousedown", {
          bubbles: true,
          button: 0,
          ctrlKey: false,
        }),
      ),
  );
  await f.v.flush();
}
test("opening and applying a nullable structured extension preserves null without manufacturing an empty collection", async () => {
  const f = await preview();
  try {
    await selectTab(f, "生活设定");
    const card = [...f.v.host.querySelectorAll(".creation-structured")].find(
      (n) => n.textContent.includes("custom_extension"),
    );
    await f.v.click(button(card, "编辑内容"));
    assert.equal(dialog().querySelector("textarea").value, "null");
    await f.v.click(button(dialog(), "应用修改"));
    assert.equal(
      f.c.draft.analysis.core_persona.life_profile.custom_extension,
      null,
    );
    assert.equal(dialog(), undefined);
  } finally {
    await f.v.dispose();
    f.c.dispose();
  }
});
test("cancelling a structured edit leaves its original scalar type and value unchanged", async () => {
  const f = await preview();
  try {
    await selectTab(f, "生活设定");
    const card = [...f.v.host.querySelectorAll(".creation-structured")].find(
      (n) => n.textContent.includes("生活习惯"),
    );
    await f.v.click(button(card, "编辑内容"));
    await f.v.input(dialog().querySelector("textarea"), '"取消的输入"');
    await f.v.click(button(dialog(), "取消"));
    assert.equal(
      f.c.draft.analysis.core_persona.life_profile.life_habits,
      "阅读",
    );
  } finally {
    await f.v.dispose();
    f.c.dispose();
  }
});

test("legacy creation without a local world checkpoint exposes meeting after actual world readiness and keeps unresolved binding review reachable", async () => {
  const f = await preview();
  try {
    f.c.draft.activatedId = "created";
    f.c.draft.worldSource = { worldId: "source", fluctlightId: "source-character" };
    delete f.c.draft.worldSelectionConfirmed;
    const model = {
      ...f.m,
      step: "complete",
      currentStep: 3,
      progress: { complete: true, stages: [{ key: "world", title: "世界", state: "ready" }] },
    };
    await f.v.update({ model });
    assert.equal(f.v.host.querySelector('.creation-meet a')?.getAttribute('href'), "#/c/created/chat");
    assert.equal(button(f.v.host, "核对已有世界的最新配置"), undefined);
    await f.v.update({ model: {
      ...model,
      step: "progress",
      progress: { complete: false, stages: [{ key: "world", title: "世界", state: "pending" }] },
    } });
    assert.equal(f.v.host.querySelector('.creation-meet'), null);
    assert.ok(button(f.v.host, "核对已有世界的最新配置"));
  } finally {
    await f.v.dispose();
    f.c.dispose();
  }
});

test("retrying an unreadable publication page retains its original cursor and updates existing cards without appending duplicates", async () => {
  const { proxy, snapshot, subscribe } = await import("valtio/vanilla"),
    { default: Page } = await vite.ssrLoadModule(
      "/src/react-app/pages/CharacterWizardPage.jsx",
    ),
    { directory } = await vite.ssrLoadModule("/src/v3/providers/real.ts");
  const original = directory.publication,
    requests = [];
  let fail = true;
  directory.publication = async (id, version) => {
    if (id === "world-two" && fail) {
      fail = false;
      throw Error("publication offline");
    }
    return {
      id,
      version,
      kind: "world",
      name: id,
      body: { background: "精确发布背景" },
      immutable: true,
    };
  };
  const state = proxy({
      session: { actorId: "creation-page-owner" },
      characters: [],
      errors: {},
    }),
    w = {
      state,
      subscribe: (fn) => subscribe(state, fn),
      getSnapshot: () => snapshot(state),
      day: () => "2026-10-28",
      refreshDirectory: async () => {},
      services: {
        configurationClient: { read: async () => ({ bindings: [] }) },
      },
      client: {
        listActorGroups: async () => [],
        worldCatalog: async () => ({ items: [] }),
        listStudioResources: async (query) => {
          requests.push({ ...query });
          if (query.kind !== "world") return { items: [], next_cursor: "" };
          return query.cursor === "second"
            ? {
                items: [
                  {
                    id: "world-two",
                    kind: "world",
                    name: "第二个世界",
                    published_version: "version:2",
                  },
                ],
                next_cursor: "",
              }
            : {
                items: [
                  {
                    id: "world-one",
                    kind: "world",
                    name: "第一个世界",
                    published_version: "version:1",
                  },
                ],
                next_cursor: "second",
              };
        },
      },
    };
  localStorage.clear();
  const v = await mountReact(vite, Page, { workspace: w });
  try {
    await act(async () =>
      [...v.host.querySelectorAll("[role=tab]")]
        .find((n) => n.textContent.trim() === "已发布的世界底稿")
        .dispatchEvent(
          new window.MouseEvent("mousedown", {
            bubbles: true,
            button: 0,
            ctrlKey: false,
          }),
        ),
    );
    await v.flush();
    await v.click(button(v.host, "读取更多世界"));
    assert.equal(v.host.querySelectorAll(".creation-world-card").length, 2);
    assert.match(v.host.textContent, /精确发布定义暂未读到/);
    await v.click(button(v.host, "重新读取世界"));
    assert.equal(v.host.querySelectorAll(".creation-world-card").length, 2);
    assert.equal(
      [...v.host.querySelectorAll(".creation-world-card")].find((n) =>
        n.textContent.includes("第二个世界"),
      ).disabled,
      false,
    );
    assert.deepEqual(
      requests.filter((r) => r.kind === "world").map((r) => r.cursor),
      ["", "second", "second"],
    );
  } finally {
    await v.dispose();
    directory.publication = original;
  }
});
