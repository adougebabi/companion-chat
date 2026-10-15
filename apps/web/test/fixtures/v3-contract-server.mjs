// 契约夹具：用原型种子投影出的响应提供页面内容。浏览器包不导入种子。
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { extname, join, normalize } from "node:path";
import { bundleFromSeed } from "../../src/v3/contract-map.mjs";
import { seed } from "./proto-seed.mjs";

const bundle = bundleFromSeed(seed());
const dist = join(process.cwd(), "apps/web/dist");
const port = Number(process.env.FLUCTLIGHT_V3_FIXTURE_PORT || 4180);
const types = { ".html": "text/html; charset=utf-8", ".js": "text/javascript; charset=utf-8", ".css": "text/css; charset=utf-8", ".svg": "image/svg+xml", ".json": "application/json" };

function ownerRequest(kind) {
  return { request: { id: `req_${kind}`, kind, fluctlight_id: "c1", status: "pending", payload: {}, decided_by: null, decision_reason: null, result_refs: [], created_at: "2026-10-15T09:40:00+08:00", updated_at: "2026-10-15T09:40:00+08:00", revision: 1 }, audit_id: "au_fixture" };
}

function api(method, pathname, loggedOut) {
  if (pathname === "/auth/session" && method === "GET") return loggedOut ? 401 : { authenticated: true, actorId: "owner" };
  if (pathname === "/auth/login" && method === "POST") return { authenticated: true, actorId: "owner" };
  if (pathname === "/auth/logout" && method === "POST") return 204;
  if (pathname.startsWith("/api/ops/timeline-checkpoints")) return 404;
  const fid = pathname.match(/^\/api\/fluctlights\/([^/]+)/)?.[1];
  if (method === "GET" && pathname === "/api/fluctlights") return bundle.fluctlights;
  if (method === "GET" && pathname === "/api/actor-groups") return bundle.groups;
  if (method === "GET" && pathname === "/api/notifications/unread-count") return bundle.unread;
  if (method === "GET" && pathname === "/api/notifications") return bundle.notifications;
  if (method === "GET" && pathname === "/api/pending/count") return bundle.pendingCount;
  if (method === "GET" && pathname === "/api/studio/resources") return bundle.resources;
  if (method === "GET" && pathname === "/api/studio/previews") return bundle.previews;
  if (method === "GET" && pathname === "/api/settings") return bundle.settings;
  if (method === "GET" && pathname === "/api/diagnostics/harm-intercepts") return bundle.harm;
  if (method === "GET" && fid && pathname.endsWith("/detail")) return bundle.details[fid] ?? 404;
  if (method === "GET" && fid && pathname.endsWith("/wardrobe")) return bundle.wardrobes[fid] ?? 404;
  if (method === "GET" && fid && pathname.endsWith("/sidebar")) return bundle.sidebars[fid] ?? 404;
  if (method === "GET" && fid && pathname.endsWith("/media")) return bundle.media[fid] ?? 404;
  if (method === "GET" && fid && pathname.endsWith("/diary")) return bundle.diaries[fid] ?? 404;
  if (method === "GET" && fid && pathname.endsWith("/nearby")) return bundle.nearby[fid] ?? 404;
  if (method === "GET" && fid && pathname.endsWith("/world")) return bundle.worlds[fid] ?? 404;
  if (method === "GET" && fid && pathname.endsWith("/templates")) return bundle.templates[fid] ?? 404;
  if (method === "GET" && fid && pathname.endsWith("/desires")) return bundle.desires[fid] ?? 404;
  if (method === "GET" && fid && pathname.endsWith("/reflections")) return bundle.reflections[fid] ?? 404;
  if (method === "GET" && fid && pathname.endsWith("/autonomy-policy")) return bundle.autonomy[fid] ?? 404;
  if (method === "GET" && fid && pathname.endsWith("/conversation")) return bundle.conversations[fid] ?? 404;
  if (method === "GET" && fid && pathname.endsWith("/goals")) return bundle.goals[fid] ?? { items: [], next_cursor: null };
  if (method === "GET" && fid && pathname.endsWith("/moments")) return (bundle.moments || []).filter((post) => post.author_actor_id === fid || post.world_id === bundle.details[fid]?.events?.[0]?.world_id || post.world_id === "w1");
  if (method === "GET" && pathname.startsWith("/api/diagnostics/tasks/")) return bundle.tasks[pathname.split("/").pop()] ?? 404;
  if (method === "POST" && pathname.endsWith("/wardrobe/wear")) return [202, ownerRequest("wardrobe_wear")];
  if (method === "POST" && pathname.endsWith("/promises")) return [202, ownerRequest("promise")];
  if (method === "POST" && pathname.endsWith("/moment-suggestions")) return [202, ownerRequest("moment_suggestion")];
  if (method === "POST" && /\/schedules\/[^/]+\/items$/.test(pathname)) return [202, ownerRequest("schedule_item_suggestion")];
  if (method === "POST" && pathname.includes("/suggestions")) return [202, ownerRequest("memory_suggestion")];
  if (method === "POST" && pathname.endsWith("/photos")) return [202, { asset_id: "photo_1", status: "queued", factual: false, label: "生成图·非事实" }];
  if (method === "POST" && pathname === "/api/moments") return [201, { id: "po_new", author_actor_id: "owner", author_type: "user", created_at: "2026-10-15T10:00:00+08:00", audit_id: "au_moment" }];
  if (pathname.startsWith("/api/") || pathname.startsWith("/auth/")) return 404;
  return null;
}

function send(response, status, body) {
  if (status === 204) { response.writeHead(204); response.end(); return; }
  const missing = body === undefined;
  const payload = missing ? { code: status === 404 ? "not_found" : "error", message: "fixture" } : body;
  response.writeHead(status, { "content-type": "application/json; charset=utf-8" });
  response.end(JSON.stringify(payload));
}

createServer(async (request, response) => {
  const url = new URL(request.url, `http://127.0.0.1:${port}`);
  const loggedOut = (request.headers.cookie || "").includes("v3loggedout=1");
  const result = api(request.method, url.pathname, loggedOut);
  if (result !== null) {
    if (Array.isArray(result) && result.length === 2 && typeof result[0] === "number") return send(response, result[0], result[1]);
    if (typeof result === "number") return send(response, result);
    return send(response, 200, result);
  }
  const relative = normalize(url.pathname === "/" ? "/index.html" : url.pathname).replace(/^(\.\.[/\\])+/, "");
  const file = join(dist, relative);
  try {
    const bytes = await readFile(file);
    response.writeHead(200, { "content-type": types[extname(file)] || "application/octet-stream" });
    response.end(bytes);
  } catch {
    const html = await readFile(join(dist, "index.html"));
    response.writeHead(200, { "content-type": "text/html; charset=utf-8" });
    response.end(html);
  }
}).listen(port, "127.0.0.1", () => console.log(`v3 fixture http://127.0.0.1:${port}`));
