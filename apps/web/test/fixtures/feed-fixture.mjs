// Isolated API transport for the supplied Demo's feed acceptance. Never imported by production.
export function feedFixture({
  owner = "feed-owner",
  fid = "A",
  scenario = "normal",
} = {}) {
  const clone = (v) => JSON.parse(JSON.stringify(v)),
    calls = [],
    receipts = new Map(),
    requests = new Map();
  let seq = 10,
    firstList = 0,
    lost = false,
    conflicted = false,
    readbackLost = false,
    wrote = false;
  const date = "2026-10-28T03:00:00.000Z";
  const make = (id, text, author = owner, scope = null) => ({
    id,
    owner_fluctlight_id: scope,
    author_actor_id: author,
    author_type:
      author === owner ? "user" : author === fid ? "fluctlight" : "npc",
    author_name: author === fid ? "林青" : author === owner ? "我" : "苏禾",
    text,
    status: "visible",
    visibility: "owner",
    revision: 3,
    media_asset_ids: [],
    media: [],
    created_at: date,
    request_id: null,
    source_event_id: null,
    like_count: 2,
    reaction_count: 3,
    viewer_reaction: null,
    comment_count: 0,
  });
  const posts =
    scenario === "empty"
      ? []
      : [
          make(
            "post-one",
            "今天在窗边读到一句很喜欢的话。\n\n停下来看看普通的一刻。",
            fid,
            fid,
          ),
          make("post-two", "把雨后的街角留在这里。"),
          {
            ...make("post-hidden", "这是一条已隐藏的片刻。", fid, fid),
            status: "hidden",
          },
        ];
  if (posts[0]) {
    posts[0].comment_count = 2;
    posts[0].media_asset_ids = ["feed-image"];
    posts[0].media = [
      {
        id: "feed-image",
        kind: "image",
        title: "窗边的阅读记录",
        readable: true,
        read_state: "readable",
        origin: "owner_upload",
        factual: false,
      },
    ];
  }
  const comments = new Map([
    [
      "post-one",
      [
        {
          id: "comment-one",
          moment_id: "post-one",
          author_actor_id: owner,
          author_type: "user",
          author_name: "我",
          text: "这一句读起来很温柔。",
          created_at: date,
          source_interaction_id: null,
        },
        {
          id: "comment-two",
          moment_id: "post-one",
          author_actor_id: "npc-actor",
          author_type: "npc",
          author_name: "苏禾",
          text: "我也想读读这本书。",
          created_at: date,
          source_interaction_id: "npc-existing",
        },
      ],
    ],
  ]);
  const npc = {
    id: "npc-interaction",
    moment_id: "post-one",
    fluctlight_id: fid,
    npc_id: "npc-one",
    status: "failed",
    revision: 4,
    attempt: 1,
    error_code: "provider_unavailable",
    response: { reason: "上次读取未完成。" },
    tasks: [],
    encounter_id: "actual-encounter",
    source_event_id: "actual-event",
    copresent: false,
  };
  if (scenario !== "empty")
    requests.set("suggestion-failed", {
      id: "suggestion-failed",
      fluctlight_id: fid,
      kind: "moment_suggestion",
      status: "failed",
      revision: 1,
      payload: { text: "想不想分享窗边阅读的近况？", media_ids: [] },
      result: { error_code: "provider_unavailable", mutation_committed: false },
      created_at: date,
      tasks: [],
    });
  const asset = {
    id: "feed-image",
    kind: "image",
    title: "窗边的阅读记录",
    origin: "owner_upload",
    readable: true,
    read_state: "readable",
    factual: false,
    revision: 1,
  };
  async function handler({ path, method, body = {}, url, json, res }) {
    if (
      !path.startsWith("/api/moment") &&
      !/\/moment-suggestions$/.test(path) &&
      !path.startsWith("/api/owner-requests/suggestion-") &&
      !path.startsWith(
        "/api/fluctlights/" + fid + "/owner-requests/suggestion-",
      ) &&
      path !== "/api/media-library" &&
      path !== "/api/media-assets/feed-image"
    )
      return false;
    calls.push({
      method,
      path,
      body: clone(body),
      query: Object.fromEntries(url.searchParams),
    });
    const send = (v, status = 200) => {
        json(res, clone(v), status);
        return true;
      },
      error = (code, status = 409) => send({ code, message: code }, status);
    const page = (items, cursor, tag) => {
      const offset = cursor ? Number(cursor.split(":").at(-1)) : 0;
      return {
        items: items.slice(offset, offset + 2),
        next_cursor: offset + 2 < items.length ? tag + ":" + (offset + 2) : "",
      };
    };
    const replay = () => {
      const hit = receipts.get(body.idempotencyKey);
      if (!hit) return null;
      if (hit.body !== JSON.stringify(body) || hit.path !== path)
        return error("idempotency_conflict");
      return send(hit.result);
    };
    const remember = (result) => {
      receipts.set(body.idempotencyKey, {
        path,
        body: JSON.stringify(body),
        result: clone(result),
      });
      wrote = true;
    };
    if (path === "/api/media-library")
      return send({ items: [asset], next_cursor: "" });
    if (path === "/api/media-assets/feed-image") return send(asset);
    if (path === "/api/moment-feed" && method === "GET") {
      firstList++;
      if (scenario === "loading" && firstList === 1)
        await new Promise((r) => setTimeout(r, 4500));
      if (scenario === "failure" && firstList === 1)
        return error("request_failed", 503);
      let items = posts.filter(
        (p) =>
          (!url.searchParams.get("fluctlightId") ||
            p.owner_fluctlight_id === url.searchParams.get("fluctlightId")) &&
          (url.searchParams.get("status") === "all" ||
            p.status === (url.searchParams.get("status") || "visible")) &&
          p.text
            .toLowerCase()
            .includes((url.searchParams.get("q") || "").toLowerCase()),
      );
      return send(page(items, url.searchParams.get("cursor"), "feed"));
    }
    if (path === "/api/moment-suggestions")
      return send(
        page(
          [...requests.values()].filter(
            (r) =>
              !url.searchParams.get("fluctlightId") ||
              r.fluctlight_id === url.searchParams.get("fluctlightId"),
          ),
          url.searchParams.get("cursor"),
          "suggest",
        ),
      );
    if (method === "GET" && path.includes("/owner-requests/")) {
      const r = requests.get(path.split("/").at(-1));
      if (!r) return error("not_found", 404);
      if (r.status === "pending" && ++r.reads >= 3) {
        r.status = "completed";
        r.revision++;
        const p = make(
          "published-" + r.id,
          "窗边的光很好，想把这段阅读时光留在这里。",
          r.fluctlight_id,
          r.fluctlight_id,
        );
        p.request_id = r.id;
        posts.unshift(p);
        r.result = {
          effects: [{ output: { target_kind: "moment", target_ref: p.id } }],
        };
      }
      return send(r);
    }
    if (method === "POST" && path.endsWith("/moment-suggestions")) {
      const hit = replay();
      if (hit) return hit;
      const target = path.split("/")[3],
        id = "suggestion-" + ++seq,
        r = {
          id,
          fluctlight_id: target,
          kind: "moment_suggestion",
          status: "pending",
          revision: 1,
          payload: { text: body.text, media_ids: body.mediaIds || [] },
          result: {},
          created_at: date,
          tasks: [],
          reads: 0,
        };
      requests.set(id, r);
      const out = { request: r, audit_id: "feed-audit" };
      remember(out);
      return send(out, 202);
    }
    if (path === "/api/moments" && method === "POST") {
      const hit = replay();
      if (hit) return hit;
      if ("authorFluctlightId" in body) throw Error("Forbidden author");
      const p = make("post-" + ++seq, body.text, owner, body.worldId || null);
      p.media_asset_ids = body.mediaIds || [];
      p.media = p.media_asset_ids.map((id) => ({ ...asset, id }));
      posts.unshift(p);
      const out = {
        id: p.id,
        author_actor_id: owner,
        author_type: "user",
        created_at: date,
        audit_id: "feed-audit",
      };
      remember(out);
      if (scenario === "unknown" && !lost) {
        lost = true;
        return error("request_failed", 503);
      }
      return send(out);
    }
    const match = path.match(/^\/api\/moment-feed\/([^/]+)(?:\/(.*))?$/);
    if (!match) return false;
    const p = posts.find((p) => p.id === decodeURIComponent(match[1])),
      tail = match[2] || "";
    if (!p) return error("not_found", 404);
    if (method === "GET" && !tail) {
      if (scenario === "readback" && wrote && !readbackLost) {
        readbackLost = true;
        return error("request_failed", 503);
      }
      return send(p);
    }
    if (tail === "comments" && method === "GET")
      return send(
        page(
          comments.get(p.id) || [],
          url.searchParams.get("cursor"),
          "comments",
        ),
      );
    if (tail === "npc-interactions" && method === "GET")
      return send({
        items: p.id === "post-one" && scenario !== "empty" ? [npc] : [],
        next_cursor: "",
      });
    if (tail === "npc-interactions/" + npc.id && method === "GET")
      return send(npc);
    if (method !== "GET") {
      const hit = replay();
      if (hit) return hit;
      if (tail.endsWith("/retry")) {
        if (body.expectedRevision !== npc.revision)
          return error("revision_conflict");
        npc.status = "pending";
        npc.revision++;
        npc.attempt++;
        remember(npc);
        return send(npc);
      }
      if (scenario === "conflict" && tail === "comments" && !conflicted) {
        conflicted = true;
        p.revision++;
        return error("revision_conflict");
      }
      if (body.expectedRevision !== p.revision)
        return error("revision_conflict");
      if (p.status === "hidden" && ["comments", "reaction"].includes(tail))
        return error("moment_hidden");
      if (tail === "comments") {
        const c = {
          id: "comment-" + ++seq,
          moment_id: p.id,
          author_actor_id: owner,
          author_type: "user",
          author_name: "我",
          text: body.text,
          created_at: new Date().toISOString(),
          source_interaction_id: null,
        };
        comments.set(p.id, [c, ...(comments.get(p.id) || [])]);
        p.comment_count++;
      } else if (tail === "reaction") {
        p.viewer_reaction = body.kind === "none" ? null : body.kind;
        p.like_count = 2 + (body.kind === "like" ? 1 : 0);
        p.reaction_count = 3 + (body.kind === "none" ? 0 : 1);
      } else if (tail === "hide" || tail === "restore")
        p.status = tail === "hide" ? "hidden" : "visible";
      else return false;
      p.revision++;
      remember(p);
      return send(p);
    }
    return false;
  }
  return { handler, calls, posts, comments, requests, npc, receipts };
}
