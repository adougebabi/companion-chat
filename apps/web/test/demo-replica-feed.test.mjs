import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";
import { act, installDOM, mountReact } from "./helpers/react-dom.mjs";
import { feedFixture } from "./fixtures/feed-fixture.mjs";
const env = installDOM(),
  vite = await createServer({
    root: fileURLToPath(new URL("../", import.meta.url)),
    appType: "custom",
    logLevel: "silent",
    server: { middlewareMode: true },
  });
const { createWorkspace } = await vite.ssrLoadModule(
    "/src/state/workspace.mjs",
  ),
  store = await vite.ssrLoadModule("/src/v3/proto/store.js"),
  { default: Feed } = await vite.ssrLoadModule(
    "/src/react-app/pages/FeedPage.jsx",
  );
test.after(async () => {
  await new Promise((r) => setTimeout(r, 25));
  await vite.close();
  env.restore();
});
const button = (root, text) =>
    [...root.querySelectorAll("button")].find(
      (n) => n.textContent.trim() === text,
    ),
  dialog = () => [...document.querySelectorAll("[role=dialog]")].at(-1),
  clone = (v) => JSON.parse(JSON.stringify(v));
async function choose(f, label, text) {
  await f.v.click(document.querySelector('[aria-label="' + label + '"]'));
  await f.v.click(
    [...document.querySelectorAll("[role=option]")].find(
      (n) => n.textContent.trim() === text,
    ),
  );
}
async function fixture({
  api,
  scenario = "normal",
  retainStorage = false,
  query = "",
  postId,
  noCharacter = false,
  intercept,
} = {}) {
  if (!retainStorage) sessionStorage.clear();
  api ||= feedFixture({ scenario });
  const w = createWorkspace();
  store.replaceSnapshot({
    ...store.get(),
    session: { actorId: "feed-owner" },
    characters: noCharacter
      ? {}
      : {
          A: { id: "A", name: "林青", status: "active" },
          B: { id: "B", name: "江屿", status: "active" },
        },
    ui: { charId: noCharacter ? "" : "A" },
  });
  w.client.fetcher = async (input, init = {}) => {
    const url = new URL(input, "http://feed.test"),
      method = init.method || "GET",
      body = init.body ? JSON.parse(init.body) : {};
    const override = await intercept?.({ url, method, body, api });
    if (override instanceof Response) return override;
    let response;
    const handled = await api.handler({
      path: url.pathname,
      url,
      method,
      body,
      json: (_res, value, status = 200) =>
        (response = new Response(JSON.stringify(value), {
          status,
          headers: { "content-type": "application/json" },
        })),
    });
    assert.ok(handled, "Unhandled " + method + " " + url.pathname);
    return response;
  };
  const { installMomentsProvider } = await vite.ssrLoadModule(
    "/src/v3/providers/moments.ts",
  );
  const { installMediaProvider } = await vite.ssrLoadModule(
    "/src/v3/providers/media.ts",
  );
  const { installLifeProvider } = await vite.ssrLoadModule(
    "/src/v3/providers/life.ts",
  );
  installMomentsProvider(w.client);
  installMediaProvider(w.client, async () => {});
  installLifeProvider(w.client);
  const props = {
    route: { page: "动态", query: new URLSearchParams(query) },
    initialPostId: postId,
  };
  const v = await mountReact(vite, Feed, { workspace: w, props });
  return {
    w,
    v,
    api,
    props,
    writes: () => api.calls.filter((c) => c.method !== "GET"),
    async dispose() {
      await v.dispose();
      w.dispose();
      await new Promise((r) => setTimeout(r, 12));
    },
  };
}
async function open(f, id = "post-one") {
  await f.v.click(
    f.v.host.querySelector(
      '[data-testid="feed-post-' + id + '"] button.companion-feed-detail-link',
    ),
  );
}
test("feed reads real mixed authors, comments, source-safe text and cursor partitions", async () => {
  const f = await fixture();
  try {
    assert.match(f.v.host.textContent, /林青/);
    assert.match(f.v.host.textContent, /这一句读起来很温柔/);
    assert.equal(
      f.api.calls.find((c) => c.path === "/api/moment-feed").query.status,
      "visible",
    );
    assert.equal(f.v.host.querySelectorAll(".companion-feed-card").length, 2);
    await open(f);
    assert.match(dialog().textContent, /一个生活片刻/);
    assert.match(dialog().textContent, /苏禾/);
    assert.equal(f.writes().length, 0);
  } finally {
    await f.dispose();
  }
});
test("feed filters preserve scope and query rather than treating a role scope as author", async () => {
  const f = await fixture({ query: "fid=A&status=all&q=窗边" });
  try {
    const query = f.api.calls.find((c) => c.path === "/api/moment-feed").query;
    assert.equal(query.fluctlightId, "A");
    assert.equal(query.status, "all");
    assert.equal(query.q, "窗边");
    assert.match(f.v.host.textContent, /包含其中不同作者/);
    let href;
    f.w.navigatePath = (p) => (href = p);
    await choose(f, "动态角色范围", "我发布的");
    assert.match(href, /author=me/);
    assert.doesNotMatch(href, /fid=A/);
  } finally {
    await f.dispose();
  }
});
test("publishing uses my real identity and only a read-back record opens the created detail", async () => {
  const f = await fixture();
  try {
    await f.v.click(button(f.v.host, "我来发动态"));
    await f.v.input(
      dialog().querySelector('[aria-label="动态正文"]'),
      "留下一段今天的生活。",
    );
    await f.v.click(button(dialog(), "从我的素材库选择附件 · 0/8"));
    await f.v.click(button(dialog(), "选择"));
    await f.v.click(button(dialog(), "发布动态"));
    assert.equal(f.writes().length, 1);
    const sent = f.writes()[0];
    assert.equal(sent.path, "/api/moments");
    assert.equal("authorFluctlightId" in sent.body, false);
    assert.equal("worldId" in sent.body, false);
    assert.deepEqual(sent.body.mediaIds, ["feed-image"]);
    assert.match(dialog().textContent, /留下一段今天的生活/);
    assert.match(dialog().textContent, /由你发布/);
  } finally {
    await f.dispose();
  }
});
test("cancelled publish does not create a post", async () => {
  const f = await fixture();
  try {
    await f.v.click(button(f.v.host, "我来发动态"));
    await f.v.input(
      dialog().querySelector('[aria-label="动态正文"]'),
      "取消时不要发布",
    );
    await f.v.click(button(dialog(), "取消"));
    assert.equal(f.writes().length, 0);
    assert.equal(dialog(), undefined);
  } finally {
    await f.dispose();
  }
});
test("unknown publication survives refresh and retries identical body and key without another post", async () => {
  const api = feedFixture({ scenario: "unknown" });
  let f = await fixture({ api });
  try {
    await f.v.click(button(f.v.host, "我来发动态"));
    await f.v.input(
      dialog().querySelector('[aria-label="动态正文"]'),
      "原发布内容",
    );
    await f.v.click(button(dialog(), "发布动态"));
    assert.ok(button(dialog(), "核对或重试原提交"));
    const original = clone(f.writes()[0]);
    await f.dispose();
    f = await fixture({ api, retainStorage: true });
    assert.ok(button(f.v.host, "继续核对动态发布"));
    await f.v.click(button(f.v.host, "继续核对动态发布"));
    assert.equal(
      dialog().querySelector('[aria-label="动态正文"]').value,
      "原发布内容",
    );
    await f.v.click(button(dialog(), "核对或重试原提交"));
    assert.deepEqual(f.writes()[1], original);
    assert.equal(api.posts.filter((p) => p.text === "原发布内容").length, 1);
    assert.match(dialog().textContent, /原发布内容/);
  } finally {
    await f.dispose();
  }
});
test("comment CAS conflict retains input and requires explicit latest-version confirmation", async () => {
  const f = await fixture({ scenario: "conflict" });
  try {
    await open(f);
    await f.v.input(
      dialog().querySelector('[aria-label="评论内容"]'),
      "保留我的评论",
    );
    await f.v.click(button(dialog(), "发布评论"));
    assert.ok(button(dialog(), "同步资料，保留输入"));
    assert.equal(
      dialog().querySelector('[aria-label="评论内容"]').value,
      "保留我的评论",
    );
    await f.v.click(button(dialog(), "同步资料，保留输入"));
    await f.v.click(button(dialog(), "已核对，采用最新基线"));
    await f.v.click(button(dialog(), "发布评论"));
    const writes = f.writes();
    assert.equal(writes.length, 2);
    assert.equal(writes[0].body.expectedRevision, 3);
    assert.equal(writes[1].body.expectedRevision, 4);
    assert.notEqual(
      writes[0].body.idempotencyKey,
      writes[1].body.idempotencyKey,
    );
    assert.match(dialog().textContent, /你的评论已保存并读回/);
    assert.equal(
      f.api.comments.get("post-one")[0].author_actor_id,
      "feed-owner",
    );
  } finally {
    await f.dispose();
  }
});
test("received comment receipt restores GET readback without sending the mutation twice", async () => {
  const f = await fixture({ scenario: "readback" });
  try {
    await open(f);
    await f.v.input(
      dialog().querySelector('[aria-label="评论内容"]'),
      "只恢复读回",
    );
    await f.v.click(button(dialog(), "发布评论"));
    assert.equal(f.writes().length, 1);
    assert.ok(button(dialog(), "只读恢复页面结果"));
    await f.v.click(button(dialog(), "只读恢复页面结果"));
    assert.equal(f.writes().length, 1);
    assert.match(dialog().textContent, /你的评论已保存并读回/);
  } finally {
    await f.dispose();
  }
});
test("reactions and hide/restore use real revisions; hidden records cannot be commented", async () => {
  let f = await fixture();
  try {
    await open(f);
    await f.v.click(button(dialog(), "更多回应与实际来源"));
    await f.v.click(button(dialog(), "关心"));
    await f.v.click(button(dialog(), "确认回应"));
    assert.equal(f.api.posts[0].viewer_reaction, "care");
    assert.match(dialog().textContent, /你已关心/);
    await f.v.click(dialog().querySelector('[aria-label="隐藏这条动态"]'));
    await f.v.click(button(dialog(), "确认隐藏"));
    assert.equal(f.api.posts[0].status, "hidden");
    assert.equal(dialog().querySelector('[aria-label="评论内容"]'), null);
    await f.v.click(dialog().querySelector('[aria-label="恢复展示动态"]'));
    await f.v.click(button(dialog(), "确认恢复展示"));
    assert.equal(f.api.posts[0].status, "visible");
    const writes = f.writes();
    assert.deepEqual(
      writes.map((x) => x.body.expectedRevision),
      [3, 4, 5],
    );
  } finally {
    await f.dispose();
  }
});
test("suggestions remain actual Owner requests and can target another role without changing current character", async () => {
  const f = await fixture();
  try {
    await f.v.click(button(f.v.host, "邀请分享近况"));
    await choose(f, "建议哪位角色", "江屿");
    await f.v.input(
      dialog().querySelector('[aria-label="分享建议"]'),
      "想不想分享最近喜欢的一句话？",
    );
    await f.v.click(button(dialog(), "提出分享建议"));
    const sent = f.writes()[0];
    assert.equal(sent.path, "/api/fluctlights/B/moment-suggestions");
    assert.equal(f.w.state.selectedId, "A");
    assert.match(dialog().textContent, /等待回应|正在考虑/);
    assert.doesNotMatch(dialog().textContent, /查看角色实际发布/);
    await f.v.click(button(dialog(), "刷新原请求"));
    await f.v.flush();
    assert.match(dialog().textContent, /查看角色实际发布/);
    assert.equal(f.writes().length, 1);
  } finally {
    await f.dispose();
  }
});
test("failed suggestions bring back original text for a new explicit proposal and preserve history", async () => {
  const f = await fixture();
  try {
    await f.v.click(f.v.host.querySelector(".companion-suggestion-row"));
    await f.v.click(button(dialog(), "重新和她商量"));
    assert.equal(
      dialog().querySelector('[aria-label="分享建议"]').value,
      "想不想分享窗边阅读的近况？",
    );
    assert.equal(f.writes().length, 0);
    await f.v.click(button(dialog(), "提出分享建议"));
    assert.equal(f.writes().length, 1);
    assert.equal(f.api.requests.get("suggestion-failed").status, "failed");
  } finally {
    await f.dispose();
  }
});
test("NPC retry preserves the interaction identity and revision, with no invented acquaintance", async () => {
  const f = await fixture();
  try {
    await open(f);
    await f.v.click(button(dialog(), "世界人物互动与实际来源"));
    await f.v.click(button(dialog(), "重试本次互动"));
    await f.v.click(button(dialog(), "确认重新核对"));
    const sent = f.writes()[0];
    assert.equal(
      sent.path,
      "/api/moment-feed/post-one/npc-interactions/npc-interaction/retry",
    );
    assert.equal(sent.body.expectedRevision, 4);
    assert.equal(f.api.npc.status, "pending");
    assert.equal(f.api.npc.copresent, false);
    assert.match(dialog().textContent, /等待独立判断/);
  } finally {
    await f.dispose();
  }
});
test("empty, initial failure/recovery and global feed without a character remain distinct", async () => {
  let f = await fixture({ scenario: "empty", noCharacter: true });
  try {
    assert.match(f.v.host.textContent, /还没有这样的片刻/);
    assert.equal(
      f.api.calls.filter((c) => c.path === "/api/moment-feed").length,
      1,
    );
    await f.v.click(button(f.v.host, "我来发动态"));
    await f.v.input(
      dialog().querySelector('[aria-label="动态正文"]'),
      "没有角色也能发布自己的动态",
    );
    await f.v.click(button(dialog(), "发布动态"));
    assert.equal(f.writes().length, 1);
  } finally {
    await f.dispose();
  }
  f = await fixture({ scenario: "failure" });
  try {
    assert.match(f.v.host.textContent, /重新读取动态/);
    assert.doesNotMatch(f.v.host.textContent, /还没有这样的片刻/);
    await f.v.click(button(f.v.host, "重新读取动态"));
    assert.match(f.v.host.textContent, /今天在窗边/);
  } finally {
    await f.dispose();
  }
});

test("comment readback follows older pages when newer comments move the accepted one off the first page", async () => {
  const f = await fixture({ scenario: "readback" });
  try {
    await open(f);
    await f.v.input(
      dialog().querySelector('[aria-label="评论内容"]'),
      "分页里的原评论",
    );
    await f.v.click(button(dialog(), "发布评论"));
    const accepted = f.api.comments.get("post-one")[0];
    f.api.comments.set("post-one", [
      ...Array.from({ length: 5 }, (_, i) => ({
        ...accepted,
        id: "newer-" + i,
        author_actor_id: "someone-else",
        text: "后来留下的回应 " + i,
      })),
      ...f.api.comments.get("post-one"),
    ]);
    f.api.posts.find((p) => p.id === "post-one").comment_count += 5;
    await f.v.click(button(dialog(), "只读恢复页面结果"));
    assert.match(dialog().textContent, /你的评论已保存并读回/);
    assert.equal(f.writes().length, 1);
    assert.ok(
      f.api.calls.some(
        (c) => c.path.endsWith("/comments") && c.query.cursor === "comments:4",
      ),
    );
  } finally {
    await f.dispose();
  }
});

test("a refreshed hidden detail updates the visible feed before it is closed", async () => {
  const f = await fixture();
  try {
    await open(f);
    const row = f.api.posts.find((p) => p.id === "post-one");
    row.status = "hidden";
    row.revision++;
    await f.v.click(button(dialog(), "刷新动态与评论"));
    assert.match(dialog().textContent, /这条动态已隐藏/);
    await f.v.click(dialog().querySelector('[data-slot="dialog-close"]'));
    assert.equal(
      f.v.host.querySelector('[data-testid="feed-post-post-one"]'),
      null,
    );
  } finally {
    await f.dispose();
  }
});

test("loading and old query replies cannot replace a newer filter snapshot", async () => {
  let release;
  const gate = new Promise((r) => (release = r));
  const f = await fixture({
    intercept: async ({ url, method }) => {
      if (
        method === "GET" &&
        url.pathname === "/api/moment-feed" &&
        !url.searchParams.get("q")
      ) {
        await gate;
      }
    },
  });
  try {
    assert.match(f.v.host.textContent, /正在读取动态/);
    await f.v.update({
      ...f.props,
      route: { page: "动态", query: new URLSearchParams("q=雨后") },
    });
    assert.match(f.v.host.textContent, /把雨后的街角/);
    await act(async () => release());
    await f.v.flush();
    assert.equal(
      f.v.host.querySelector('[data-testid="feed-post-post-one"]'),
      null,
    );
    assert.match(f.v.host.textContent, /把雨后的街角/);
  } finally {
    release();
    await f.dispose();
  }
});

test("a late accepted publication retains newer text separately from the confirmed original", async () => {
  let release;
  const gate = new Promise((r) => (release = r));
  const f = await fixture({
    intercept: async ({ url, method }) => {
      if (method === "POST" && url.pathname === "/api/moments") await gate;
    },
  });
  try {
    await f.v.click(button(f.v.host, "我来发动态"));
    await f.v.input(
      dialog().querySelector('[aria-label="动态正文"]'),
      "原来的片刻",
    );
    await f.v.click(button(dialog(), "发布动态"));
    await f.v.input(
      dialog().querySelector('[aria-label="动态正文"]'),
      "等待时想到的另一段话",
    );
    await act(async () => release());
    await f.v.flush();
    assert.equal(f.writes().length, 1);
    assert.equal(f.api.posts[0].text, "原来的片刻");
    assert.equal(
      dialog().querySelector('[aria-label="动态正文"]').value,
      "等待时想到的另一段话",
    );
    assert.ok(button(dialog(), "同步资料，保留输入"));
    assert.equal(button(dialog(), "发布动态").disabled, true);
  } finally {
    release();
    await f.dispose();
  }
});

test("a real Owner switch clears old recovery slots and a late response cannot repopulate them or the new Owner feed", async () => {
  let release;
  const gate = new Promise((r) => (release = r));
  const f = await fixture({
    intercept: async ({ url, method }) => {
      if (method === "POST" && url.pathname === "/api/moments") await gate;
    },
  });
  try {
    await f.v.click(button(f.v.host, "我来发动态"));
    await f.v.input(
      dialog().querySelector('[aria-label="动态正文"]'),
      "旧Owner的片刻",
    );
    await f.v.click(button(dialog(), "发布动态"));
    assert.ok(
      Object.keys(sessionStorage).some((k) =>
        k.startsWith("fluctlight-studio:feed-owner:"),
      ),
    );
    await act(async () =>
      store.replaceSnapshot({
        ...store.get(),
        session: { actorId: "next-owner" },
      }),
    );
    await f.v.flush();
    await act(async () => release());
    await f.v.flush();
    assert.equal(f.w.state.session.actorId, "next-owner");
    assert.equal(
      Object.keys(sessionStorage).filter((k) =>
        k.startsWith("fluctlight-studio:feed-owner:"),
      ).length,
      0,
    );
    assert.equal(f.v.host.querySelectorAll("[role=dialog]").length, 0);
    assert.equal(f.writes().length, 1);
  } finally {
    release();
    await f.dispose();
  }
});
