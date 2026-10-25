import { reactive, computed, ref, inject } from "vue";
import {
  client,
  registerProviders,
  refreshProduct,
  conversation,
  openConversation,
} from "../v3/providers/real.ts";
import { get, list, subscribe, setUi } from "../v3/proto/store.js";
import {
  lifeClient,
  diaryClient,
  memoryClient,
  inventoryClient,
  momentsClient,
  mediaClient,
  operationsClient,
  mockWorld as worldClient,
  mockClient as npcClient,
  configurationClient,
  diagnosticsClient,
  studioClient,
} from "../v3/proto/services.js";
import { persistentCommand } from "../v3/providers/persistent-command.mjs";
import { randomId } from "../random-id.mjs";
import { errorMessage } from "../v3/domain-presentation.mjs";
import { profileSnapshotFromDetail } from "../v3/proto/data/profile.js";
import { scheduleProjection } from "./presentation.mjs";
import { userInstant } from "../v3/presentation-time.mjs";
import { requestDecisionText } from '../v3/proto/components/request-failure.js';
export const WorkspaceKey = Symbol("approved-workspace");
export const useWorkspace = () => {
  const value = inject(WorkspaceKey);
  if (!value) throw Error("Workspace is unavailable");
  return value;
};
export const slots = [
  "外套",
  "上衣",
  "内衣",
  "内裤",
  "下衣",
  "袜子",
  "鞋子",
  "配饰",
];
const slotNames = {
  outer: "外套",
  outerwear: "外套",
  top: "上衣",
  upper_body: "上衣",
  underwear: "内衣",
  panties: "内裤",
  bottom: "下衣",
  lower_body: "下衣",
  full_body: "上衣",
  socks: "袜子",
  shoes: "鞋子",
  accessory: "配饰",
  accessories: "配饰",
};
const seasonNames = { spring: "春", summer: "夏", autumn: "秋", winter: "冬" };
const categoryNames = {
  episodic: "发生过的事情",
  semantic: "长期记忆",
  relationship: "长期记忆",
  autobiographical: "最近在想",
};
const safeText = (v) =>
  typeof v === "string" ? v : v == null ? "" : JSON.stringify(v);
export const day = (value = new Date(), zone = "Asia/Shanghai") => {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: zone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(new Date(value));
  return ["year", "month", "day"]
    .map((k) => parts.find((p) => p.type === k)?.value)
    .join("-");
};
export const clock = (value, zone = "Asia/Shanghai") =>
  value && Number.isFinite(Date.parse(value))
    ? new Intl.DateTimeFormat("en-GB", {
        timeZone: zone,
        hour: "2-digit",
        minute: "2-digit",
        hour12: false,
      }).format(new Date(value))
    : "";
export function blankCharacter() {
  return {
    id: "",
    name: "尚未选择角色",
    initial: "光",
    color: "#e5ebe2",
    job: "",
    bio: "",
    activity: "",
    location: "",
    mood: "",
    world: "",
    genre: "",
    outfit: "",
    tags: [],
    schedule: [],
    goals: [],
    posts: [],
    garments: [],
    outfits: [],
    outfitTags: [],
    wearing: {},
    people: [],
    sites: [],
    messages: [],
    life: { appointments: [], memories: [], diaries: [] },
    detail: null,
  };
}
export function garmentView(item) {
  return {
    id: item.id,
    name: item.name || item.description,
    type: slotNames[item.clothing_type] || slotNames[item.slot] || "配饰",
    seasons: (item.seasons || []).map((v) => seasonNames[v] || v),
    _source: item,
  };
}
export function memoryView(row) {
  return {
    id: row.id,
    text: row.content,
    date:
      row.created_at && Number.isFinite(Date.parse(row.created_at))
        ? day(row.created_at)
        : "",
    source: row.source_type || row.source_kind || "真实记忆记录",
    importance: Number.isFinite(row.importance)
      ? Math.round(row.importance * 5)
      : 0,
    category: categoryNames[row.type] || "长期记忆",
    tags: row.tags || row.metadata?.tags || [],
    _source: row,
  };
}
export function diaryView(row) {
  return {
    id: row.id,
    date: row.date,
    title: row.title || row.date + "的日记",
    weather: row.weather || "",
    mood: row.mood || "",
    text: row.body || "",
    time: row.created_at ? clock(row.created_at) : "",
    status: row.status,
    _source: row,
  };
}
export async function collectPages(read) {
  const rows = [],
    seen = new Set();
  let cursor = "";
  do {
    const page = await read(cursor);
    rows.push(...(page.items || []));
    cursor = page.next_cursor || "";
    if (cursor && seen.has(cursor)) throw Error("读取到重复分页，请重新读取");
    seen.add(cursor);
  } while (cursor);
  return rows;
}

export function createWorkspace() {
  const state = reactive({
    ready: false,
    session: null,
    characters: [],
    selectedId: "",
    page: "聊天",
    worlds: [],
    media: [],
    operations: [],
    diagnostics: [],
    notifications: [],
    pending: [],
    errors: {},
    loading: {},
    toast: null,
    managementTab: "配置中心",
    outfitTab: "为她搭配",
    appointmentHighlight: false,
    configuration: null,
  });
  let active = true,
    generation = 0,
    toastTimer;
  const epochs = new Map(),
    channels = new Map(),
    timers = new Set();
  const current = computed(
    () =>
      state.characters.find((c) => c.id === state.selectedId) ||
      blankCharacter(),
  );
  const currentScope = () =>
    JSON.stringify([state.session?.actorId, state.selectedId]);
  const isCurrent = (owner, fid) =>
    active && state.session?.actorId === owner && state.selectedId === fid;
  const notify = (text, link) => {
    clearTimeout(toastTimer);
    state.toast = { text, link };
    toastTimer = setTimeout(() => (state.toast = null), 3500);
  };
  function replaceDirectory() {
    const owner = get("session");
    if (state.session?.actorId !== owner?.actorId) {
      generation++;
      state.characters = [];
      channels.clear();
      state.operations = [];
      state.worlds = [];
      state.media = [];
      state.notifications = [];
      state.pending = [];
      state.errors = {};
      state.loading = {};
    }
    state.session = owner;
    state.characters = list("characters").map((row, index) => {
      const previous =
        state.characters.find((c) => c.id === row.id) || blankCharacter();
      return {
        ...previous,
        id: row.id,
        name: row.name,
        initial: [...(row.name || "光")].at(-1),
        color: row.color || ["#e4ede4", "#e6e8ed", "#eee8df"][index % 3],
        bio: row.bio || "",
        status: row.status,
        _directory: row,
      };
    });
    if (!state.characters.some((c) => c.id === state.selectedId))
      state.selectedId = get("ui")?.charId || state.characters[0]?.id || "";
  }
  const unsubscribe = subscribe(replaceDirectory);
  async function bootstrap() {
    registerProviders();
    try {
      await refreshProduct({ initialOnly: true });
      if (!active) return;
      replaceDirectory();
      state.ready = true;
    } catch (error) {
      state.errors.bootstrap = errorMessage(error);
      state.ready = true;
      throw error;
    }
  }
  async function read(key, run) {
    const owner = state.session?.actorId,
      fid = state.selectedId,
      scopeGeneration = generation,
      epoch = (epochs.get(key) || 0) + 1;
    epochs.set(key, epoch);
    state.loading[key] = true;
    const live = () =>
      isCurrent(owner, fid) &&
      generation === scopeGeneration &&
      epochs.get(key) === epoch;
    try {
      const result = await run(live);
      if (!live()) return;
      delete state.errors[key];
      return result;
    } catch (error) {
      if (live()) state.errors[key] = errorMessage(error);
      throw error;
    } finally {
      if (live()) state.loading[key] = false;
    }
  }
  async function detail() {
    const fid = state.selectedId;
    if (!fid) return;
    return read("detail", async (live) => {
      const raw = await client.detail(fid);
      if (!live()) return;
      const persona = raw.core_persona || {},
        identity = raw.identity || persona.identity || {},
        context = raw.context || {};
      Object.assign(current.value, {
        detail: raw,
        bio: profileSnapshotFromDetail(raw).bio,
        job: identity.occupation || identity.profession || "",
        activity: context.activity || "",
        location: context.location || "",
        mood:
          typeof raw.inner_state?.mood === "string"
            ? raw.inner_state.mood
            : raw.inner_state?.mood?.label || "",
        world: raw.world?.name || "",
        schedule: (raw.schedule?.items || []).map((item) =>
          scheduleProjection(
            item,
            { ...raw.schedule, timezone: identity.timezone || "Asia/Shanghai" },
            clock,
          ),
        ),
        timezone: identity.timezone || "Asia/Shanghai",
      });
      return raw;
    });
  }
  async function loadLife() {
    const fid = state.selectedId;
    if (!fid) return;
    const scope = currentScope();
    await detail();
    if (currentScope() !== scope) return;
    return read("life", async (live) => {
      const results = await Promise.allSettled([
        lifeClient.schedules(fid),
        collectPages((c) => lifeClient.goals(fid, c)),
        collectPages((c) => diaryClient.list(fid, c)),
        collectPages((c) =>
          memoryClient.list(fid, { cursor: c, status: "active", limit: 30 }),
        ),
        collectPages((c) => lifeClient.promises(fid, c)),
      ]);
      if (!live()) return;
      const c = current.value;
      const accept = (index, key, update) => {
        const result = results[index];
        if (result.status === "fulfilled") {
          delete state.errors[key];
          update(result.value);
        } else state.errors[key] = errorMessage(result.reason);
      };
      accept(0, "schedule", (value) => {
        c.schedule = (value.days || [])
          .filter(
            (d) =>
              d.local_date === (value.today || day(new Date(), value.timezone)),
          )
          .flatMap((d) =>
            (d.schedule?.items || []).map((item) =>
              scheduleProjection(
                item,
                { ...d.schedule, timezone: value.timezone },
                clock,
              ),
            ),
          );
      });
      accept(
        1,
        "goals",
        (rows) =>
          (c.goals = rows.map((g) => ({
            id: g.id,
            text: g.desired_outcome || g.title,
            done: g.status === "completed",
            category:
              {
                human: "来自约定",
                drive: "内在愿望",
                event: "经历与变化",
                self: "自主目标",
              }[g.source] || "目标",
            progress: Math.round(Number(g.progress || 0) * 100),
            _source: g,
          }))),
      );
      accept(
        2,
        "diary",
        (rows) =>
          {c.life.diaryStates=rows; c.life.diaries = rows.filter((d) => d.body?.trim()).map(diaryView);},
      );
      accept(3, "memory", (rows) => (c.life.memories = rows.map(memoryView)));
      accept(
        4,
        "promises",
        (rows) =>
          (c.life.appointments = rows
            .filter((p) => p.starts_at || p.due_at)
            .map((p) => ({
              id: p.id,
              date: day(p.starts_at || p.due_at, c.timezone),
              time: clock(p.starts_at || p.due_at, c.timezone),
              text: p.title,
              _source: p,
            }))),
      );
    });
  }
  async function loadWardrobe() {
    const fid = state.selectedId;
    return read("wardrobe", async (live) => {
      const [catalog, wearing, library, tags] = await Promise.all([
        inventoryClient.clothingCatalogue(fid),
        lifeClient.wardrobe(fid),
        inventoryClient.outfits(fid),
        inventoryClient.tags(fid),
      ]);
      if (!Array.isArray(wearing?.worn_items) || wearing.worn_items.some(item=>!item||typeof (item.item_id||item.id)!=='string'||typeof item.slot!=='string'))
        throw Error("当前穿着未能完整读回，已保留上次确认的记录，请重新读取。");
      const all = library.items || [];
      let cursor = library.next_cursor,
        seen = new Set();
      while (cursor) {
        if (seen.has(cursor)) throw Error("方案分页重复，请重新读取");
        seen.add(cursor);
        const page = await inventoryClient.outfits(fid, cursor);
        if (page.profile_id !== library.profile_id)
          throw Error("人格已变化，请重新读取");
        all.push(...page.items);
        cursor = page.next_cursor;
      }
      if (!live()) return;
      const c = current.value;
      c.garments = catalog.items.map(garmentView);
      c.wearing = Object.fromEntries(
        slots.map((s) => [
          s,
          wearing.worn_items
            .filter(
              (i) => (slotNames[i.slot] || slotNames[i.clothing_type]) === s,
            )
            .map((i) => i.id || i.item_id),
        ]),
      );
      c.outfits = all.map((p) => ({
        id: p.id,
        name: p.name,
        tag: p.tags?.[0] || "",
        tags: p.tags || [],
        items: Object.fromEntries(
          slots.map((s) => [
            s,
            (p.items || [])
              .filter(
                (i) => (slotNames[i.slot] || slotNames[i.clothing_type]) === s,
              )
              .map((i) => i.id),
          ]),
        ),
        _source: p,
      }));
      c.outfitTags = tags.tags || [];
      c._wardrobe = { catalog, wearing, library, tags };
    });
  }
  async function loadWorlds() {
    const fid = state.selectedId;
    return read("world", async (live) => {
      const catalog = await worldClient.worlds(fid),
        worlds = [],
        bindings = new Map([[fid, catalog.find((w) => w.active)?.id]]);
      for (const c of state.characters) {
        if (c.id === fid) continue;
        if (!live()) return;
        const view = await client.worldCatalog(c.id);
        if (!live()) return;
        bindings.set(c.id, view.active_binding?.world_id);
      }
      for (const w of catalog) {
        if (!live()) return;
        const places = await worldClient.places(fid, w.id);
        if (!live()) return;
        const categories = (w.venueCategories || []).map((group) => ({
          id: group.id,
          name: group.name,
          places: [],
        }));
        for (const p of places) {
          let group = categories.find((g) => g.id === p.venueCategoryId);
          if (!group) {
            const name = p.group || "其他地点",
              id = p.group ? "group:" + p.group : "unclassified";
            group = categories.find((g) => g.id === id);
            if (!group) {
              group = { id, name, places: [] };
              categories.push(group);
            }
          }
          group.places.push({
            ...p,
            custom: false,
            description: p.description || p.desc,
            _source: p,
          });
        }
        worlds.push({
          id: w.id,
          name: w.name,
          genre: w.flavor || "",
          active: w.active,
          residents: state.characters
            .filter((c) => bindings.get(c.id) === w.id)
            .map((c) => c.id),
          sites: categories,
          _source: w,
        });
      }
      if (!live()) return;
      state.worlds = worlds;
      current.value.sites = worlds.find((w) => w.active)?.sites || [];
      current.value.world = worlds.find((w) => w.active)?.name || "";
    });
  }
  async function loadBrowse(page) {
    const fid = state.selectedId;
    return read(page, async (live) => {
      if (page === "动态") {
        const rows = await collectPages((cursor) =>
          momentsClient.list({
            ...(state.feedScope ? { fluctlightId: state.feedScope } : {}),
            cursor,
          }),
        );
        const posts = [];
        for (const post of rows) {
          if (!live()) return;
          const comments = await momentsClient.comments(post.id);
          if (!live()) return;
          posts.push({
            ...post,
            authorName:
              post.author_name ||
              (post.author_type === "user" ? "我" : "作者名称未记录"),
            authorInitial: [...(post.author_name || "我")].at(-1),
            text: post.text,
            time: userInstant(post.created_at),
            likes: post.like_count || 0,
            liked: post.viewer_reaction === "like",
            comments: (comments.items || []).map((v) => ({
              name: v.author_type === "user" ? "我" : v.author_name || "作者",
              text: v.text,
              time: userInstant(v.created_at),
            })),
            _source: post,
          });
        }
        if (live()) current.value.posts = posts;
      } else if (page === "人物关系") {
        const rows = await npcClient.npcs(fid);
        if (live())
          current.value.people = rows.map((p) => ({
            id: p.id,
            name: p.name,
            identity: p.role,
            relation: p.relation?.summary || "",
            location: p.location || "",
            bio: p.bio,
            schedule:
              p.routine?.map((r) => r.start + " " + r.activity).join("；") ||
              "",
            tag: "",
            type: "世界中的人",
            status: p.archived ? "暂未联系" : "活跃",
            _source: p,
          }));
      } else if (page === "媒体") {
        const rows = await collectPages((cursor) =>
          mediaClient.list({
            fluctlightId: fid,
            scope: "fluctlight",
            cursor,
            hidden: false,
          }),
        );
        if (!live()) return;
        state.media = rows.map((a, index) => ({
          id: a.id,
          artIndex: index,
          assetId: a.id,
          character: current.value.name,
          type:
            { image: "图片", audio: "语音", video: "视频" }[a.kind] || "图片",
          tag: a.metadata?.tags?.[0] || "",
          title: a.title || a.source_filename || "未命名素材",
          time: userInstant(a.created_at),
          url: mediaClient.url(a.id),
          _source: a,
        }));
      } else if (page === "通知与待处理") {
        const [notices, pending] = await Promise.all([
          operationsClient.notifications(),
          operationsClient.pending(),
        ]);
        if (!live()) return;
        state.notifications = notices.items;
        state.pending = pending.groups.flatMap((g) =>
          (g.items || []).map((item) => ({ ...item, kind: g.kind })),
        );
      } else if (page === "创作中心") {
        const rows = await studioClient.list();
        if (live()) state.resources = rows.items;
      }
    });
  }
  const navigatePath = (path) => (location.hash = "#" + path);
  const observations = new Map();
  const receiptKey = (owner, fid) =>
    "fluctlight:approved-receipts:" + JSON.stringify([owner, fid]);
  function pointers(owner, fid) {
    try {
      return JSON.parse(sessionStorage.getItem(receiptKey(owner, fid)) || "[]");
    } catch {
      return [];
    }
  }
  function checkpoint(owner, fid, request, label, operation) {
    const key = receiptKey(owner, fid),
      rows = pointers(owner, fid).filter((r) => r.id !== request.id &&
        !(operation?.startsWith("wear-outfit:") && r.operation === operation));
    rows.push({ id: request.id, label, operation });
    sessionStorage.setItem(key, JSON.stringify(rows));
  }
  function updateCheckpoint(owner, fid, id, patch) {
    const rows = pointers(owner, fid).map((row) => row.id === id ? {...row, ...patch} : row);
    const latest = new Map();
    rows.forEach((row, index) => {
      if (row.operation?.startsWith("wear-outfit:")) latest.set(row.operation, index);
    });
    // Keep submission order when old checkpoints discover their outfit out of order.
    sessionStorage.setItem(receiptKey(owner, fid), JSON.stringify(rows.filter((row, index) =>
      !row.operation?.startsWith("wear-outfit:") || latest.get(row.operation) === index)));
  }
  function command(
    label,
    operation,
    value,
    run,
    {
      fid = state.selectedId,
      onSaved,
      success = "已保存",
      multiStage = false,
      preAdmissionConflicts,
    } = {},
  ) {
    const owner = state.session?.actorId,
      scope = JSON.stringify([owner, fid, operation]);
    const unresolved = state.operations.find(
      (record) => record.fid === fid && record.operation === operation &&
        (record.requestUnresolved || (operation.startsWith("wear-outfit:") && record.status === "pending")),
    );
    if (unresolved) return unresolved;
    let viewGeneration = generation;
    const live = () => isCurrent(owner, fid) && generation === viewGeneration;
    let channel = channels.get(scope);
    if (!channel) {
      channel = persistentCommand({
        key: "fluctlight:approved-command:" + scope,
        storage: () => sessionStorage,
        preAdmissionConflicts,
        execute: async (body, key) => {
          if (!isCurrent(owner, fid))
            throw Error("账号或角色已变化，请回到原范围核对提交");
          let result;
          try {
            result = await run(body, key, () => isCurrent(owner, fid));
          } catch (error) {
            if (multiStage)
              throw new Error(errorMessage(error), { cause: error });
            throw error;
          }
          if (result?.request?.id && state.session?.actorId === owner)
            checkpoint(owner, fid, result.request, label, operation);
          return result;
        },
      });
      channels.set(scope, channel);
    }
    const record = reactive({
        id: randomId(),
        fid,
        label,
        operation,
        character: current.value.name,
        input: safeText(value),
        status: "pending",
        reason: "",
        response: "",
        run: null,
        actionText: "重试",
      }),
      frozen = JSON.parse(JSON.stringify(value));
    const execute = async (recover = false) => {
      viewGeneration = generation;
      record.status = "pending";
      record.reason = "";
      try {
        const pending = channel.pending();
        if (
          pending &&
          !recover &&
          JSON.stringify(pending.value) !== JSON.stringify(frozen)
        ) {
          record.originalInput = safeText(pending.value);
          record.actionText = "重试原提交";
          record.run = () => execute(true).catch(() => {});
          throw Error(
            "原提交结果尚未确认，新草稿已保留。请核对下方原内容，再重试原提交。",
          );
        }
        const result = pending
          ? await channel.retry()
          : await channel.submit(frozen);
        channels.delete(scope);
        if (!live()) return result;
        const request = result?.request;
        record.response = request ? "请求已受理，等待处理" : success;
        record.status =
          request &&
          ["pending", "processing", "awaiting_goal"].includes(request.status)
            ? "pending"
            : "success";
        if (request) {
          record.requestId = request.id;
          record.requestUnresolved = true;
          void observeRequest(owner, fid, request.id, record, onSaved);
        } else {
          await onSaved?.(result);
          if (live()) notify(success);
        }
        return result;
      } catch (error) {
        channels.delete(scope);
        if (live()) {
          record.status = "error";
          record.reason = errorMessage(error);
          if (channel.pending()?.rejection) {
            record.reason += "；请核对最新版本后重新提出修改。当前草稿已保留。";
            record.actionText = "核对版本";
            record.run = () => {
              const before = channel.pending();
              if (!before?.rejection) return;
              record.review = true;
            };
          }
        }
        throw error;
      }
    };
    record.run = () => execute().catch(() => {});
    record.release = async () => {
      if (!live()) return;
      await load(state.page);
      if (!live()) return;
      channel.releaseRejected();
      channels.delete(scope);
      record.review = false;
      record.actionText = "查看原对象";
      record.run = () => {
        const base = "/c/" + encodeURIComponent(fid);
        if (
          operation.startsWith("memory:") ||
          operation.startsWith("forget-memory:")
        )
          navigatePath(
            base + "/growth/memory/" + encodeURIComponent(frozen.source.id),
          );
        else if (
          operation.includes("world:") ||
          operation.startsWith("expand:")
        )
          navigatePath(
            base +
              "/worlds/" +
              encodeURIComponent(operation.split(":").slice(1).join(":")) +
              "/edit",
          );
        else navigatePath(base + "/governance");
      };
      record.reason =
        "已读取最新版本并释放明确拒绝的原提交。请从原对象核对当前草稿，再明确保存。";
    };
    state.operations.push(record);
    record.completion = execute();
    record.completion.catch(() => {});
    return record;
  }
  async function observeRequest(owner, fid, id, record, onSaved) {
    const key = JSON.stringify([owner, fid, id]),
      observation = observations.get(key) || {
        owner,
        fid,
        id,
        record,
        onSaved,
      };
    if (observation.running) return;
    if (observation.timer) {
      clearTimeout(observation.timer);
      timers.delete(observation.timer);
    }
    observations.set(key, observation);
    if (!isCurrent(owner, fid) || document.hidden) return;
    observation.running = true;
    try {
      const request = await lifeClient.request(fid, id);
      if (!isCurrent(owner, fid)) return;
      record.requestStatus = request.status;
      if (!record.operation && request.kind === 'inventory_outfit' && typeof request.payload?.outfit_id === 'string')
        record.operation = 'wear-outfit:' + request.payload.outfit_id;
      if (record.operation?.startsWith("wear-outfit:"))
        updateCheckpoint(owner, fid, id, {operation: record.operation});
      if (["pending", "processing", "awaiting_goal"].includes(request.status)) {
        record.requestUnresolved = true;
        record.status = "pending";
        const timer = setTimeout(() => {
          timers.delete(timer);
          void observeRequest(owner, fid, id, record, onSaved);
        }, 3000);
        observation.timer = timer;
        timers.add(timer);
        return;
      }
      observations.delete(key);
      record.requestUnresolved = false;
      const retainResult = record.operation?.startsWith("wear-outfit:");
      if (retainResult) updateCheckpoint(owner, fid, id, {terminal: true});
      else sessionStorage.setItem(receiptKey(owner, fid), JSON.stringify(pointers(owner, fid).filter((r) => r.id !== id)));
      record.status = ["completed", "accepted"].includes(request.status)
        ? "success"
        : "error";
      record.reason =
        record.status === "success"
          ? ""
          : requestDecisionText(request);
      record.response = record.status === 'success'
        ? request.status === 'accepted' ? '请求已接受，实际结果将继续更新。'
          : request.result?.disposition === 'no_change'
            ? retainResult ? '已经穿着这套搭配，本次无需换装。' : '目标状态已经满足，本次无需新增变更。'
            : '请求已完成，可核对当前记录。'
        : '';
      record.actionText = "查看原请求";
      record.run = () => {
        location.hash = "#/diag/request/" + encodeURIComponent(id);
      };
      if (onSaved) {
        record.refreshResult = async () => {
          if (!isCurrent(owner, fid)) return;
          record.readbackError = "";
          try {
            await onSaved(request);
          } catch (error) {
            if (isCurrent(owner, fid))
              record.readbackError = "请求状态已确认，但页面结果读回失败：" + errorMessage(error);
          }
        };
        await record.refreshResult();
      }
    } catch (error) {
      if (isCurrent(owner, fid)) {
        record.status = "error";
        record.reason = "请求已受理，处理状态暂未确认，请恢复查询原请求：" + errorMessage(error);
        record.actionText = "恢复查询原请求";
        record.run = () => observeRequest(owner, fid, id, record, onSaved);
      }
    } finally {
      observation.running = false;
    }
  }
  function resumeRequests() {
    if (document.hidden) return;
    const owner = state.session?.actorId,
      fid = state.selectedId;
    if (!owner || !fid) return;
    for (const pointer of pointers(owner, fid)) {
      const key = JSON.stringify([owner, fid, pointer.id]);
      if (observations.has(key)) {
        const o = observations.get(key);
        if (!state.operations.some((record) => record.fid === fid && record.requestId === pointer.id))
          state.operations.push(o.record);
        void observeRequest(owner, fid, pointer.id, o.record, o.onSaved);
        continue;
      }
      if (state.operations.some((record) => record.fid === fid && record.requestId === pointer.id)) continue;
      const onSaved = () => pointer.label === "换装" ? loadWardrobe() : load(state.page);
      const record = reactive({
        id: randomId(),
        requestId: pointer.id,
        fid,
        label: pointer.label,
        operation: pointer.operation,
        requestUnresolved: !pointer.terminal,
        status: "pending",
        reason: "",
        run: () =>
          observeRequest(owner, fid, pointer.id, record, onSaved),
      });
      state.operations.push(record);
      void observeRequest(owner, fid, pointer.id, record, onSaved);
    }
  }
  document.addEventListener("visibilitychange", resumeRequests);
  async function load(page = state.page) {
    state.page = page;
    if (
      !state.selectedId &&
      ![
        "管理中心",
        "角色库",
        "通知与待处理",
        "创作中心",
        "初始化角色",
      ].includes(page)
    )
      return;
    resumeRequests();
    try {
      if (page === "聊天") {
        await detail();
        await loadWardrobe();
      } else if (page === "此刻")
        await Promise.allSettled([loadLife(),loadWardrobe()]);
      else if (["日程", "目标", "日记", "记忆"].includes(page))
        await loadLife();
      else if (page === "角色详情") await detail();
      else if (page === "衣柜") await loadWardrobe();
      else if (page === "世界") await loadWorlds();
      else if (page === "角色库")
        await read("library", async (live) => {
          for (const c of state.characters) {
            const raw = await client.detail(c.id);
            if (!live()) return;
            const identity = raw.identity || raw.core_persona?.identity || {};
            Object.assign(c, {
              bio: profileSnapshotFromDetail(raw).bio,
              job: identity.occupation || identity.profession || "",
              hobby:
                raw.life_profile?.preferences?.likes?.join?.("、") ||
                identity.interests?.join?.("、") ||
                "",
              tags: identity.tags || [],
              detail: raw,
            });
          }
        });
      else if (page !== "管理中心") await loadBrowse(page);
    } catch {
      /* Domain error stays on the current work surface. */
    }
  }
  function select(id) {
    if (state.selectedId === id || !state.characters.some((c) => c.id === id))
      return;
    generation++;
    state.loading = {};
    state.selectedId = id;
    setUi({ charId: id });
  }
  return {
    state,
    current,
    client,
    conversation,
    openConversation,
    bootstrap,
    refreshDirectory: () => refreshProduct({ initialOnly: true }),
    detail,
    load,
    loadLife,
    loadWardrobe,
    loadWorlds,
    loadBrowse,
    command,
    notify,
    select,
    isCurrent,
    currentScope,
    navigatePath,
    day,
    clock,
    slots,
    services: {
      lifeClient,
      diaryClient,
      memoryClient,
      inventoryClient,
      momentsClient,
      mediaClient,
      operationsClient,
      worldClient,
      npcClient,
      configurationClient,
      diagnosticsClient,
      studioClient,
    },
    dispose() {
      active = false;
      generation++;
      unsubscribe();
      document.removeEventListener("visibilitychange", resumeRequests);
      observations.clear();
      clearTimeout(toastTimer);
      for (const timer of timers) clearTimeout(timer);
      timers.clear();
    },
  };
}
