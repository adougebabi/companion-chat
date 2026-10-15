// 页面读取的视图模型。数据只来自 BrowserClient 的契约响应，不读取种子，也不写入 localStorage。
import { BrowserClient, currentRevision, studioApplyUnfinished } from "@fluctlight/browser-client";
import { viewFromBundle } from "../contract-map.mjs";
import { confirm, toast } from "./ui.js";

export const IDENTITIES = ["交流", "行动请求", "所有者修改", "系统运维"];
const COLS = ["characters", "messages", "memories", "goals", "schedule", "actions", "items", "media", "places", "npcs", "events", "relations", "diary", "tasks", "resources", "proposals", "posts", "postSuggestions", "reflections"];
const subs = new Set();
let client = null;
let data = emptyData();

function emptyData() {
  return { meta: { seedVersion: 4, seq: 100, unwired: {}, drafts: {}, ownerRequests: {} }, ui: { charId: "", identity: "交流", notes: false }, groups: {}, characters: {}, worlds: {}, user: { id: "u1", name: "我", avatar: "/v3/assets/avatars/user.svg", background: {} }, places: {}, npcs: {}, relations: {}, messages: {}, memories: {}, goals: {}, schedule: {}, actions: {}, items: {}, outfit: {}, media: {}, events: {}, diary: {}, tasks: {}, proposals: {}, resources: {}, instances: {}, studioDrafts: [], posts: {}, postSuggestions: {}, reflections: {}, worldSettings: {}, prefs: { global: {}, autonomy: {} }, session: null, notices: [], audit: [] };
}

function emit(event) { subs.forEach((fn) => { try { fn(event); } catch (error) { console.error(error); } }); }
function missing(error) { return error?.status === 404 || error?.status === 501 || error?.code === "not_provided"; }
async function optional(promise) { try { return await promise; } catch (error) { if (missing(error) || error?.status === 401) return undefined; throw error; } }

export function bindClient(next) { client = next; }
export function getClient() { return client; }
export function get(col, id) { if (!col) return data; const collection = data[col]; return id === undefined ? collection : collection?.[id]; }
export function list(col, pred = () => true) { const collection = data[col]; return (Array.isArray(collection) ? collection : Object.values(collection || {})).filter(pred); }
export function setUi(patch) { Object.assign(data.ui, patch); emit({ type: "ui", patch }); }
export function subscribe(fn) { subs.add(fn); return () => subs.delete(fn); }
export function auditLog() { return data.audit; }
export function uid(prefix) { data.meta.seq += 1; return prefix + data.meta.seq; }
export function resetDemo() { throw new Error("不提供演示数据重置，也没有时间线回档"); }
export function find(id) { for (const col of COLS) if (data[col]?.[id]) return { col, obj: data[col][id] }; return null; }
export function pending(charId = "c1") {
  const out = [];
  if (data.meta.unwired.pending) return out;
  list("proposals", (item) => item.charId === charId && item.state === "候选").forEach((item) => out.push({ id: item.id, text: item.title, link: `#/c/${charId}/profile` }));
  list("resources").forEach((resource) => { const inst = data.instances[charId]?.[resource.id]; if (inst && inst.applied !== resource.published) out.push({ id: resource.id, text: `${resource.name} 定义 ${resource.published} 待应用（当前 ${inst.applied}）`, link: `#/studio/edit/${resource.id}` }); });
  list("tasks", (item) => item.charId === charId && item.state === "失败").forEach((item) => out.push({ id: item.id, text: `任务 ${item.id} 失败：${item.title}`, link: `#/diag/task/${item.id}` }));
  return out;
}
export function bellCount() {
  if (data.meta.unreadUnwired || typeof data.meta.unreadCount !== "number") return null;
  const waiting = typeof data.meta.pendingCount === "number" ? data.meta.pendingCount : 0;
  return data.meta.unreadCount + waiting;
}
export function currentDraft(target) { return data.meta.drafts?.[target] || null; }

function key() { return globalThis.crypto?.randomUUID?.() || `idem_${Date.now()}`; }
function guard() { data.meta.toastGuard = Date.now() + 2500; }

export async function hydrate() {
  if (!client) client = new BrowserClient();
  const bundle = {};
  const session = await client.session().catch(() => ({ authenticated: false }));
  if (!session.authenticated) { data = emptyData(); emit({ type: "reset" }); return; }
  bundle.fluctlights = await optional(client.listFluctlights());
  bundle.groups = await optional(client.listActorGroups());
  bundle.notifications = await optional(client.listNotifications());
  bundle.unread = await optional(client.unreadNotificationCount());
  bundle.pendingCount = await optional(client.pendingCount());
  bundle.resources = await optional(client.listStudioResources());
  bundle.previews = await optional(client.listStudioPreviews());
  bundle.settings = await optional(client.settings());
  bundle.harm = await optional(client.harmIntercepts({ window: "7d" }));
  bundle.details = {}; bundle.wardrobes = {}; bundle.sidebars = {}; bundle.media = {}; bundle.diaries = {}; bundle.nearby = {}; bundle.worlds = {}; bundle.templates = {}; bundle.desires = {}; bundle.reflections = {}; bundle.autonomy = {}; bundle.conversations = {}; bundle.goals = {}; bundle.tasks = {};
  const momentMap = new Map();
  for (const item of bundle.fluctlights || []) {
    const id = item.id;
    bundle.details[id] = await optional(client.detail(id));
    bundle.wardrobes[id] = await optional(client.wardrobe(id));
    bundle.sidebars[id] = await optional(client.sidebar(id));
    bundle.media[id] = await optional(client.listCharacterMedia(id));
    bundle.diaries[id] = await optional(client.listDiary(id));
    bundle.nearby[id] = await optional(client.nearby(id));
    bundle.worlds[id] = await optional(client.world(id));
    bundle.templates[id] = await optional(client.worldTemplates(id));
    bundle.desires[id] = await optional(client.desires(id));
    bundle.reflections[id] = await optional(client.listReflections(id));
    bundle.autonomy[id] = await optional(client.autonomyPolicy(id));
    bundle.conversations[id] = await optional(client.directConversation(id));
    bundle.goals[id] = await optional(client.goals(id, { limit: 50 }));
    const posts = await optional(client.moments(id));
    for (const post of posts || []) momentMap.set(post.id, post);
  }
  if (momentMap.size) bundle.moments = [...momentMap.values()];
  const taskIds = new Set();
  for (const detail of Object.values(bundle.details)) for (const action of detail?.actions || []) if (action.taskId) taskIds.add(action.taskId);
  for (const taskId of taskIds) {
    const task = await optional(client.diagnosticTask(taskId));
    if (task) bundle.tasks[taskId] = task;
  }
  const view = viewFromBundle(bundle);
  view.ui.identity = data.ui.identity || "交流";
  view.ui.notes = false;
  if (data.ui.charId && view.characters[data.ui.charId]) view.ui.charId = data.ui.charId;
  view.meta.drafts = data.meta.drafts || {};
  view.meta.ownerRequests = data.meta.ownerRequests || {};
  view.meta.unreadUnwired = view.meta.unreadCount === undefined;
  const backups = await optional(client.listBackups());
  view.meta.backupAvailable = backups && backups.available === false ? false : backups ? true : undefined;
  data = view;
  emit({ type: "reset" });
}

async function rememberConflict(error, target, draft, wardrobe = false) {
  const revision = currentRevision(error);
  const resource = error?.details?.resource;
  if (error?.status === 409 && error?.code === "revision_conflict" && target) {
    data.meta.drafts[target] = draft;
    data.meta.conflictRevision = revision;
    if (wardrobe && resource === "wardrobe") {
      const fid = data.ui.charId || "c1";
      data.meta.wardrobeRevisions = data.meta.wardrobeRevisions || {};
      if (typeof revision === "number") data.meta.wardrobeRevisions[fid] = revision;
      toast(`她刚更新过。草稿已保留。当前衣橱 revision ${revision ?? "未知"}。`, "err");
    } else toast(`版本冲突，草稿已保留。当前 revision ${revision ?? "未知"}。`, "err");
    return true;
  }
  return false;
}

function ownerSaved(response) {
  return response?.status === "saved" && response?.identity === "owner_edit" && response?.actor === "owner" && typeof response?.revision === "number" && Boolean(response?.audit_id);
}

function managing() { return data.ui.identity === "管理"; }

async function refreshWardrobe(fid) {
  const page = await client.wardrobe(fid);
  data.meta.wardrobeRevisions = data.meta.wardrobeRevisions || {};
  data.meta.ownerRevisions = data.meta.ownerRevisions || {};
  if (typeof page?.revision === "number") data.meta.wardrobeRevisions[fid] = page.revision;
  if (typeof page?.owner_revision === "number") data.meta.ownerRevisions[fid] = page.owner_revision;
  if (Array.isArray(page?.worn_items)) {
    const current = page.worn_items.map((entry) => entry.item_id).filter(Boolean);
    const recordedBy = Object.fromEntries(page.worn_items.filter((entry) => entry.recorded_by === "owner" || entry.recorded_by === "fluctlight").map((entry) => [entry.item_id, entry.recorded_by]));
    const outfit = { ...(data.outfit[fid] || { history: [] }), current, since: page.worn_items[0]?.since || data.outfit[fid]?.since || "", history: data.outfit[fid]?.history || [] };
    if (Object.keys(recordedBy).length) outfit.recordedBy = recordedBy;
    data.outfit[fid] = outfit;
  }
  for (const entry of page?.used_items || []) {
    if ((entry.recorded_by === "owner" || entry.recorded_by === "fluctlight") && data.items[entry.item_id]) data.items[entry.item_id].recordedBy = entry.recorded_by;
  }
}

async function dispatchWrite(draft, meta) {
  const summary = meta.summary || "";
  const fid = data.ui.charId || "c1";
  try {
    if (/建议小遥日程/.test(summary)) {
      const response = await client.suggestScheduleItem(fid, "today", { activity: summary, startAt: new Date().toISOString(), endAt: new Date().toISOString(), evidenceRefs: [], expectedRevision: 0, expectedLifeContextRevision: 0, reason: summary, idempotencyKey: key() });
      data.meta.ownerRequests[response.request.id] = response.request;
      toast(`日程建议已提交，状态 ${response.request.status}。日程本身还没变。`, "");
      return;
    }
    if (/向小遥提出约定/.test(summary)) {
      const response = await client.proposePromise(fid, { title: summary, dueAt: new Date().toISOString(), reason: summary, idempotencyKey: key() });
      data.meta.ownerRequests[response.request.id] = response.request;
      toast(`约定已提交，状态 ${response.request.status}。她还没决定。`, "");
      return;
    }
    if (/建议小遥发一条动态/.test(summary)) {
      const response = await client.suggestMoment(fid, { text: summary, mediaIds: [], idempotencyKey: key() });
      data.meta.ownerRequests[response.request.id] = response.request;
      toast(`发帖建议已提交，状态 ${response.request.status}。不会代她发布。`, "");
      return;
    }
    if (/告诉小遥关于记忆|用户告诉小遥/.test(summary)) {
      const response = await client.suggestMemoryRevision(meta.target, { text: summary, reason: summary, expectedRevision: data.memories[meta.target]?.version || 0, idempotencyKey: key() });
      data.meta.ownerRequests[response.request.id] = response.request;
      toast(`已告诉她，状态 ${response.request.status}。记忆正文还没改。`, "");
      return;
    }
    if (/物品拍照/.test(summary)) {
      const response = await client.requestItemPhoto(fid, meta.target, { prompt: summary, idempotencyKey: key() });
      toast(`拍照已排队（${response.status}），${response.label}。没有改动穿着或事实。`, "");
      return;
    }
    if (/换装|穿上|标记使用|结束使用/.test(summary)) {
      const revision = data.meta.wardrobeRevisions?.[fid];
      const using = /标记使用|结束使用/.test(summary);
      const send = (confirmOwner) => using
        ? client.requestItemUse(fid, meta.target, { active: !/结束使用/.test(summary), ...(typeof revision === "number" ? { expectedRevision: revision } : {}), reason: summary, evidenceRefs: [], ...(confirmOwner ? { confirm: true } : {}), idempotencyKey: key() })
        : client.requestWardrobeWear(fid, { itemId: meta.target, slot: "worn", ...(typeof revision === "number" ? { expectedRevision: revision } : {}), reason: summary, ...(confirmOwner ? { confirm: true } : {}), idempotencyKey: key() });
      if (managing()) {
        if (typeof revision !== "number") { toast("衣橱 revision 尚未接入，不能把缺失当成 0。草稿已保留。", "err"); if (meta.target) data.meta.drafts[meta.target] = draft; return; }
        const ok = await confirm({ title: using ? "所有者修改使用" : "所有者修改衣橱", text: `用衣橱 revision ${revision} 直接记录。owner_revision 只用于展示，不会当作期望版本。`, identity: "所有者修改", okText: "确认修改" });
        if (!ok) { toast("已取消，草稿已保留。", "err"); if (meta.target) data.meta.drafts[meta.target] = draft; return; }
        const response = await send(true);
        if (response?.request) { data.meta.ownerRequests[response.request.id] = response.request; toast(`仍在处理中，状态 ${response.request.status}。事实还没变。`, ""); return; }
        if (!ownerSaved(response)) { toast("没有收到所有者修改回执，没有把这次写成已保存。", "err"); if (meta.target) data.meta.drafts[meta.target] = draft; return; }
        data.meta.wardrobeRevisions[fid] = response.revision;
        if (typeof response.owner_revision === "number") data.meta.ownerRevisions[fid] = response.owner_revision;
        await refreshWardrobe(fid).catch(() => {});
        toast(`已记为所有者修改，衣橱 revision ${response.revision}。`, "ok");
        emit({ type: "update", entry: { summary: "所有者修改" } });
        return;
      }
      const response = await send(false);
      data.meta.ownerRequests[response.request.id] = response.request;
      toast(using ? `使用请求已提交，状态 ${response.request.status}。使用状态还没变。` : `换装请求已提交，状态 ${response.request.status}。穿着还没变。`, "");
      return;
    }
    if (/用户以自己身份发布动态/.test(summary)) {
      const created = draft.posts?.[meta.target];
      const response = await client.createMoment({ text: created?.text || summary, mediaIds: [], idempotencyKey: key() });
      if (created) data.posts[response.id] = { ...created, id: response.id, authorType: "user" };
      toast("已用你自己的身份发布。", "ok");
      emit({ type: "update", entry: { summary } });
      return;
    }
    if (/记录系统偏好/.test(summary)) {
      await client.updateSettings({ values: draft.prefs.global });
      data.prefs.global = draft.prefs.global;
      toast("偏好已记录。运行时权限没有改变。", "ok");
      emit({ type: "update", entry: { summary } });
      return;
    }
    if (/自治偏好/.test(summary)) {
      const pref = draft.prefs.autonomy[fid];
      const quiet = String(pref?.免打扰 || "").split("-");
      const response = await client.saveAutonomyPolicy(fid, { level: pref?.自治程度 || "", quiet_hours: quiet.length === 2 ? { start: quiet[0], end: quiet[1] } : null, spend_cap: null, outreach_limit: pref?.主动触达 === "高" ? "high" : pref?.主动触达 === "低" ? "low" : "medium", allow_moments: Boolean(pref?.允许发动态), confirm_action_types: [], expectedRevision: 0, reason: summary, idempotencyKey: key() });
      if (response.runtime_effect && response.runtime_effect !== "preference_only") toast("服务器返回了意外的运行时效果。", "err");
      data.prefs.autonomy[fid] = pref;
      toast("自治偏好已记录（preference_only）。运行时权限没有改变。", "ok");
      emit({ type: "update", entry: { summary } });
      return;
    }
    if (/移动 .+ 到分组/.test(summary)) {
      const character = draft.characters?.[meta.target];
      if (character?.groupId) await client.assignActorGroupMember(character.groupId, meta.target);
      data.characters[meta.target].groupId = character.groupId;
      emit({ type: "update", entry: { summary } });
      return;
    }
    if (/回应反思/.test(summary)) {
      const response = await client.submitReflectionFeedback(fid, meta.target, { text: summary, idempotencyKey: key() });
      if (response?.request) data.meta.ownerRequests[response.request.id] = response.request;
      toast(`反思回应已作为候选提交，状态 ${response?.request?.status || "pending"}。没有改她的反思正文。`, "");
      return;
    }
    if (/通知已读|通知全部已读/.test(summary)) {
      if (data.meta.unreadUnwired || data.meta.unwired?.notices) { toast("通知尚未接入，没有标成已读。", "err"); return; }
      const ids = summary.startsWith("通知全部已读") ? data.notices.filter((item) => !item.read).map((item) => item.id) : [summary.split(" ").pop()];
      for (const id of ids) {
        const saved = await client.markNotificationRead(id, { idempotencyKey: key() });
        const notice = data.notices.find((item) => item.id === id);
        if (notice && saved?.read_at) notice.read = true;
      }
      emit({ type: "update", entry: { summary } });
      return;
    }
    toast(`「${summary || "这次修改"}」尚未接入，草稿已保留。`, "err");
    if (meta.target) data.meta.drafts[meta.target] = draft;
  } catch (error) {
    if (await rememberConflict(error, meta.target, draft, /换装|穿上|衣橱/.test(summary))) return;
    toast(error?.userMessage || error?.message || "提交失败", "err");
    if (meta.target) data.meta.drafts[meta.target] = draft;
  }
}

export function update(mutator, { identity, summary, target = null, silent = false } = {}) {
  if (!IDENTITIES.includes(identity)) throw new Error("update() 需要合法的操作身份: " + identity);
  const draft = structuredClone(data);
  const result = mutator(draft);
  if (silent) return result;
  guard();
  if (/小遥接受|小遥答应|小遥采纳|运行时：|小遥回复/.test(summary || "")) {
    toast("不会把角色决定或运行结果写成已完成。", "err");
    return result;
  }
  void dispatchWrite(draft, { identity, summary, target });
  return result;
}

export async function submitExchange(text) {
  const charId = data.ui.charId;
  const conversationId = data.meta.conversations?.[charId];
  if (!client || !conversationId) throw new Error("对话发送未接入");
  const response = await client.turn(conversationId, { text, fluctlightId: charId, idempotencyKey: key() });
  if (!response.ok && response.body) throw new Error("对话发送失败");
  await response.text().catch(() => "");
  const page = await client.messages(conversationId).catch(() => null);
  if (!page?.messages) return;
  for (const message of page.messages) {
    if (data.messages[message.id]) continue;
    data.messages[message.id] = { id: message.id, charId, from: message.authorActorId === "owner" ? "user" : "char", text: message.text, at: String(message.createdAt || "").match(/T(\d{2}:\d{2})/)?.[1] || "", identity: "交流" };
  }
  emit({ type: "update", entry: { summary: "用户发送消息" } });
}

export async function signIn(password) {
  if (!client) client = new BrowserClient();
  const session = await client.login(password);
  data.session = session.authenticated ? { user: session.actorId || "所有者", at: "", mode: "登录" } : null;
  await hydrate();
}

function packUnwired(response) {
  return !response || response.available === false || response.status === "not_implemented" || response.status === "not_provided";
}

export async function exportConfigPack() {
  if (!client) client = new BrowserClient();
  const response = await client.exportStudioPack();
  if (packUnwired(response) || !response.pack) throw Object.assign(new Error("配置包导出尚未接入"), { userMessage: "配置包导出尚未接入" });
  return response.pack;
}

export async function validateConfigPack(pack) {
  if (!client) client = new BrowserClient();
  return client.validateStudioPack({ pack, idempotencyKey: key() });
}

export async function importConfigPack(pack) {
  if (!client) client = new BrowserClient();
  const response = await client.importStudioPack({ pack, idempotencyKey: key() });
  if (packUnwired(response) || response.status !== "draft") throw Object.assign(new Error("配置包导入尚未接入"), { userMessage: "配置包导入尚未接入" });
  return response;
}

export async function recordStudioApply(resourceId, body) {
  if (!client) client = new BrowserClient();
  const response = await client.applyStudioResource(resourceId, body);
  data.meta.studioApplyNotes = data.meta.studioApplyNotes || {};
  if (studioApplyUnfinished(response)) data.meta.studioApplyNotes[resourceId] = "未完成·已记录选择，尚未写入实例";
  toast(data.meta.studioApplyNotes[resourceId] || "应用结果没有未完成标记，也没有改本地实例版本。", "err");
  emit({ type: "update", entry: { summary: "未完成·已记录选择，尚未写入实例" } });
  return response;
}

export async function signOut() {
  if (client) await client.logout().catch(() => {});
  data = emptyData();
  emit({ type: "reset" });
}

export const _debug = { get data() { return data; } };
