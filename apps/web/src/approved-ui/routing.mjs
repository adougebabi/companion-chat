export const lifePages = ["此刻", "日程", "目标", "衣柜", "日记", "记忆"];
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
};
export function approvedRoute(hash) {
  let value;
  try {
    value = decodeURIComponent(String(hash || "").replace(/^#/, ""));
  } catch {
    return { legacy: true, path: "/auth" };
  }
  const [path, search = ""] = value.split("?"),
    query = new URLSearchParams(search);
  if (query.get("legacy") === "1") return { legacy: true, path, query };
  if (!path || workspacePages.includes(path))
    return { page: path || "聊天", fid: null, query, alias: true };
  const match = /^\/c\/([^/]+)\/(.*)$/.exec(path);
  if (match && modules[match[2]])
    return { page: modules[match[2]], fid: match[1], query };
  const global = {
    "/feed": "动态",
    "/inbox": "通知与待处理",
    "/library": "角色库",
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
  return { legacy: true, path, query };
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
      创作中心: "#/studio",
      管理中心: "#/settings",
    }[page] || "#/library"
  );
}
