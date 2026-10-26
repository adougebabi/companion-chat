export const lifePages = [
  "此刻",
  "日程",
  "目标",
  "衣柜",
  "日记",
  "记忆",
  "角色详情",
];
export const workspacePages = [
  "聊天",
  ...lifePages,
  "世界",
  "动态",
  "人物关系",
  "媒体",
  "通知与待处理",
  "角色库",
  "创作中心",
  "初始化角色",
  "管理中心",
];
const modules = {
  chat: "聊天",
  "life/now": "此刻",
  "life/schedule": "日程",
  "life/goals": "目标",
  "life/wardrobe": "衣柜",
  "life/diary": "日记",
  growth: "记忆",
  world: "世界",
  people: "人物关系",
  media: "媒体",
  profile: "角色详情",
};
export function approvedRoute(hash) {
  let value;
  try {
    value = decodeURIComponent(String(hash || "").replace(/^#/, ""));
  } catch {
    return { notFound: true };
  }
  const [path, search = ""] = value.split("?"),
    query = new URLSearchParams(search);
  if (query.getAll("legacy").includes("1")) return { removed: true, path, query };
  if (!path || workspacePages.includes(path))
    return { page: path || "聊天", fid: null, query, alias: true };
  const match = /^\/c\/([^/]+)\/(.*)$/.exec(path);
  if (match && modules[match[2]])
    return { page: modules[match[2]], fid: match[1], query };
  if (path === "/settings/kev") { query.set("section", "高级"); return {page:"管理中心",query}; }
  const studio = /^\/studio\/edit\/([^/]+)$/.exec(path);
  if (studio) return { page: "资源编辑", resourceId: studio[1], kind: query.get("kind") || "world", query };
  const diagnostic = /^\/diag\/(request|task|record|turns|media-prompts|pending|kev|lifecycle|agent-runs|workflow-intents|events|links|harm)(?:\/([^/]+))?(?:\/([^/]+))?$/.exec(path);
  if (diagnostic) return { page: "运行详情", query, diagnostic: { kind: diagnostic[1], id: diagnostic[1] === "record" ? diagnostic[3] : diagnostic[2], recordKind: diagnostic[1] === "record" ? diagnostic[2] : undefined, query: query.get("correlationId") || query.get("q") || "" } };
  if (match) {
    const tool = /^(?:media\/(avatar|identity)|(governance))$/.exec(match[2]);
    if (tool) return {page:"角色工具",fid:match[1],tool:tool[1]||tool[2],tab:query.get("tab")||"foundation",query};
    const memory = /^growth\/memory\/([^/]+)$/.exec(match[2]);
    if (memory) return {page:"记忆",fid:match[1],memoryId:memory[1],query};
    const world = /^worlds\/([^/]+)\/edit$/.exec(match[2]);
    if (world) return {page:"世界",fid:match[1],worldId:world[1],query};
  }
  const global = {
    "/auth": "登录",
    "/feed": "动态",
    "/inbox": "通知与待处理",
    "/library": "角色库",
    "/library/create": "初始化角色",
    "/studio": "创作中心",
    "/settings": "管理中心",
    "/diag": "管理中心",
  };
  if (global[path])
    return {
      page: global[path],
      fid: query.get("fid"),
      query,
      diagnostics: path === "/diag",
    };
  return { notFound: true, path, query };
}
export function approvedHref(page, fid) {
  const module = Object.entries(modules).find(
    ([, label]) => label === page,
  )?.[0];
  if (module)
    return fid ? "#/c/" + encodeURIComponent(fid) + "/" + module : "#/library";
  return (
    {
      动态: "#/feed" + (fid ? "?fid=" + encodeURIComponent(fid) : ""),
      通知与待处理: "#/inbox",
      角色库: "#/library",
      初始化角色: "#/library/create",
      创作中心: "#/studio",
      管理中心: "#/settings",
    }[page] || "#/library"
  );
}
