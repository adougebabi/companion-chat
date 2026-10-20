// 把契约响应投影成原型页面读取的视图模型，并提供反向的夹具投影。
// 浏览器包只调用 viewFromBundle；种子只存在于测试夹具。

const IDENTITY = { exchange: "交流", action_request: "行动请求", owner_edit: "所有者修改", system_ops: "系统运维" };
const IDENTITY_OUT = Object.fromEntries(Object.entries(IDENTITY).map(([key, value]) => [value, key]));
const BY = { 运行时: "runtime", 所有者修改: "owner_edit" };
const BY_IN = { runtime: "运行时", owner_edit: "所有者修改" };
const KIND = { 地点: "place", 物品: "item", 事件模板: "event_template", 角色: "character_template", 人格: "persona_fragment", 视觉: "visual_style" };
const KIND_IN = Object.fromEntries(Object.entries(KIND).map(([key, value]) => [value, key]));
const OUTREACH = { 低: "low", 适中: "medium", 高: "high" };
const OUTREACH_IN = { low: "低", medium: "适中", high: "高" };

export function assetUrl(value) {
  return typeof value === "string" ? value.replace(/^assets\//, "/v3/assets/") : value;
}

function clock(value) {
  const match = String(value ?? "").match(/T(\d{2}:\d{2})/);
  return match ? match[1] : String(value ?? "");
}

function postAt(value) {
  const match = String(value ?? "").match(/^\d{4}-(\d{2})-(\d{2})T(\d{2}:\d{2})/);
  return match ? `${match[1]}-${match[2]} ${match[3]}` : String(value ?? "");
}

function isoOn(monthDay, time) {
  const [month, day] = monthDay.split("-");
  return `2026-${month}-${day}T${time}:00+08:00`;
}

function messageKind(message) {
  if (message.from === "divider") return "divider";
  if (message.from === "sys") return "system";
  if (message.from === "action") return "action";
  if (message.kind === "image" || message.mediaId) return "image";
  return "text";
}

function copyDefined(target, key, value) {
  if (value !== undefined) target[key] = value;
}

export function bundleFromSeed(seed) {
  const characters = Object.values(seed.characters);
  const fluctlights = characters.map((character) => {
    const item = {
      id: character.id,
      identity: { name: character.name },
      status: character.status,
      status_note: character.statusNote ?? null,
      now_line: character.doing ?? null,
      group_id: character.groupId,
      avatar: character.avatar ? { asset_id: `av_${character.id}`, url: assetUrl(character.avatar), factual: false, label: "插画" } : null,
      world_id: character.worldId,
      bio: character.bio,
      traits: character.traits,
      location_id: character.locationId,
    };
    if (character.defVersion !== undefined) item.def_version = character.defVersion;
    if (Object.hasOwn(character, "harmIntercepts")) item.harm_intercepts = character.harmIntercepts;
    return item;
  });
  const details = {};
  const sidebars = {};
  const wardrobes = {};
  const media = {};
  const diaries = {};
  const nearby = {};
  const worlds = {};
  const templates = {};
  const desires = {};
  const reflections = {};
  const autonomy = {};
  const conversations = {};
  const moments = {};
  const goals = {};
  for (const character of characters) {
    const mood = character.mood ? { label: character.mood.value, state: character.mood.state, evidence: character.mood.evidence, updated_at: "2026-10-15T09:40:00+08:00" } : null;
    const momentum = character.energy ? { label: character.energy.value, state: character.energy.state, evidence: character.energy.evidence, updated_at: "2026-10-15T09:40:00+08:00" } : null;
    const place = character.locationId ? seed.places[character.locationId] : null;
    details[character.id] = {
      current_state: {
        ...(place ? { scene_image: { asset_id: `scene_${place.id}`, url: assetUrl(place.scene), captured_at: "2026-10-15T09:05:00+08:00", factual: false, label: "生成图·非事实" } } : { scene_image: null }),
        mood,
        momentum,
        doing: character.doing ? { text: character.doing, action_id: character.doingActionId ?? null } : null,
        location_id: character.locationId,
      },
      events: Object.values(seed.events).filter((event) => event.worldId === character.worldId).map((event) => ({
        id: event.id, world_id: event.worldId, title: event.title, at: event.at, place_id: event.placeId, state: event.state,
        impacts: event.impacts.map((impact) => ({ type: impact.type, id: impact.id, text: impact.text })),
      })),
      schedule_items: Object.values(seed.schedule).filter((item) => item.charId === character.id).map((item) => ({
        id: item.id, char_id: item.charId, time: item.time, title: item.title, place_id: item.placeId, state: item.state,
        fact: item.fact, fact_state: item.factState, action_id: item.actionId ?? null, goal_id: item.goalId ?? null,
        adjusted_by: item.adjustedBy ?? null, note: item.note ?? null,
      })),
      memories: Object.values(seed.memories).filter((item) => item.charId === character.id).map((item) => ({
        id: item.id, fluctlight_id: item.charId, content: item.content, state: item.state, kind: item.kind, version: item.version,
        source_message_id: item.sourceMsg ?? null, related_place_id: item.relatedPlace ?? null, related_npc_id: item.relatedNpc ?? null,
        source_event_id: item.sourceEvent ?? null,
        revisions: item.revisions.map((revision) => ({ v: revision.v, at: revision.at, text: revision.text, by: BY[revision.by] || "runtime", source_ref: revision.source ?? null, note: revision.note ?? null })),
      })),
      relationships: Object.values(seed.relations).filter((item) => item.charId === character.id).map((item) => ({
        id: item.id, fluctlight_id: item.charId, npc_id: item.npcId, stage: item.stage, version: item.version,
        intimacy: item.intimacy,
        revisions: item.revisions.map((revision) => ({ v: revision.v, at: revision.at, text: revision.text, by: BY[revision.by] || "runtime", source_ref: revision.source ?? null, note: revision.note ?? null })),
      })),
      actions: Object.values(seed.actions).filter((item) => item.charId === character.id),
      proposals: Object.values(seed.proposals).filter((item) => item.charId === character.id),
      actor_user: character.id === "c1" ? { background: seed.user.background } : undefined,
    };
    const next = Object.values(seed.schedule).find((item) => item.charId === character.id && item.state === "计划");
    const worn = seed.outfit[character.id]?.current ?? [];
    sidebars[character.id] = {
      location: place ? { place_id: place.id, name: place.name } : null,
      next_schedule: next ? { schedule_item_id: next.id, title: next.title, start_at: next.time } : null,
      worn_item_ids: worn,
      goal_ids: Object.values(seed.goals).filter((item) => item.charId === character.id).map((item) => item.id),
      memory_ids: Object.values(seed.memories).filter((item) => item.charId === character.id).map((item) => item.id),
      doing: character.doing ? { text: character.doing, action_id: character.doingActionId ?? null } : null,
    };
    const items = Object.values(seed.items).filter((item) => item.charId === character.id);
    const used = items.filter((item) => item.status === "使用").map((item) => ({ item_id: item.id, since: "09:00" }));
    const wornItems = (seed.outfit[character.id]?.current ?? []).map((id) => ({ item_id: id, slot: "worn", since: seed.outfit[character.id].since }));
    wardrobes[character.id] = {
      operation: "list", revision: 4, inventory_complete: true, has_more: false, next_cursor: null, searched_scope: "owner", can_conclude_absent: true,
      worn_items: wornItems, used_items: used,
      items: items.map((item) => ({
        id: item.id, name: item.name, category_label: item.cat, image_url: assetUrl(item.img),
        ownership_label: item.status === "借用" ? "借用" : item.status === "未拥有" ? "未拥有" : "拥有",
        source_label: item.source, inbound_status: item.status === "入库中" ? "pending" : "ready", failure_reason: null,
        source_action_id: item.actionId ?? null, source_message_id: item.sourceMsg ?? null, source_event_id: item.sourceEvent ?? null,
        goal_id: item.goalId ?? null, owner_actor_id: item.owner ?? null,
        category: item.cat, slot: "unspecified", description: item.name, ownership: item.status === "借用" ? "borrowed" : item.status === "未拥有" ? "absent" : "owned",
        availability: "available", source_kind: "record", revision: 1,
      })),
    };
    media[character.id] = {
      items: Object.values(seed.media).filter((item) => item.charId === character.id).map((item) => ({
        asset_id: item.id, kind: "image", url: assetUrl(item.img), title: item.title, created_at: "2026-10-15T09:30:00+08:00",
        outfit_snapshot_id: null, item_id: item.itemId ?? null, source_message_id: item.sourceMsg ?? null, factual: false, label: "生成图·非事实",
        place_id: item.placeId ?? null, outfit_snapshot: item.outfitSnapshot ?? null, overlay_url: item.overlay ? assetUrl(item.overlay) : null,
      })),
      next_cursor: null,
    };
    diaries[character.id] = {
      items: Object.values(seed.diary).filter((item) => item.charId === character.id).map((item) => ({
        id: item.id, date: item.date, body: item.text, occurred_at: isoOn(item.date, "21:00"), event_id: null, refs: item.refs, source: "agent", replies: item.replies, revision: 1,
      })),
      next_cursor: null,
    };
    nearby[character.id] = {
      place_id: character.locationId,
      actors: Object.values(seed.npcs).filter((npc) => npc.placeId && npc.placeId === character.locationId).map((npc) => ({ actor_id: npc.id, name: npc.name, npc: true, full_fluctlight: false, since: "09:00" })),
    };
    const world = seed.worlds[character.worldId];
    if (world) {
      worlds[character.id] = {
        world: { id: world.id, name: world.name, ...(world.defVersion ? { def_version: world.defVersion } : {}) },
        places: Object.values(seed.places).filter((item) => item.worldId === world.id).map((item) => ({
          id: item.id, name: item.name, desc: item.desc, scene_asset: { url: assetUrl(item.scene), label: "插画" }, resource_id: item.resourceId ?? null, override: null,
        })),
        scenes: [], map: null,
        npcs: Object.values(seed.npcs).filter((npc) => seed.places[npc.placeId]?.worldId === world.id).map((npc) => ({
          id: npc.id, name: npc.name, role: npc.role, place_id: npc.placeId, avatar_url: assetUrl(npc.avatar), full_fluctlight: npc.full === true, bio: npc.bio,
        })),
      };
    }
    const settings = seed.worldSettings[character.worldId];
    templates[character.id] = {
      revision: 1,
      items: settings ? Object.entries(settings.cats).map(([category, enabled]) => ({ template_id: `tpl_${category}`, name: category, category, enabled, permanent_harm: false, locked: false })) : [],
      random_events: settings ? settings.randomEvents : null,
      frequency: settings ? settings.freq : null,
    };
    desires[character.id] = { items: Object.values(seed.goals).filter((item) => item.charId === character.id && item.type === "意向").map((item) => ({ id: item.id, text: item.title, status: item.state, why: item.why ?? null, source_message_id: item.sourceMsg ?? null, schedule_id: item.scheduleId ?? null, action_id: item.actionId ?? null, item_id: item.itemId ?? null })) };
    goals[character.id] = { items: Object.values(seed.goals).filter((item) => item.charId === character.id && item.type !== "意向").map((item) => ({ id: item.id, desired_outcome: item.title, status: item.state, type: item.type, progress: item.progress ?? null, due: item.due ?? null, why: item.why ?? null })), next_cursor: null };
    reflections[character.id] = {
      items: Object.values(seed.reflections).filter((item) => item.charId === character.id).map((item) => ({
        id: item.id, title: item.title, body: item.text, state: item.state, growth: item.growth, at: item.at, responded_at: item.feedback ? item.at : null, claim_ids: [], source_refs: item.sources, feedback: item.feedback,
      })),
      next_cursor: null,
    };
    const pref = seed.prefs.autonomy[character.id];
    if (pref) {
      const [start, end] = String(pref.免打扰 || "").split("-");
      autonomy[character.id] = { level: pref.自治程度, quiet_hours: start && end ? { start, end } : null, spend_cap: null, outreach_limit: OUTREACH[pref.主动触达] ?? null, allow_moments: pref.允许发动态, confirm_action_types: [], revision: 1, runtime_effect: "preference_only" };
    }
    conversations[character.id] = {
      conversation: { id: `conv_${character.id}`, segment_id: `seg_${character.id}`, started_at: "2026-10-15T09:00:00+08:00", revision: 1 },
      participants: [],
      long_term_revision_unchanged: true,
      messages: Object.values(seed.messages).filter((item) => item.charId === character.id).map((item) => ({
        id: item.id, conversationId: `conv_${character.id}`, sequence: 0, authorActorId: item.from === "user" ? seed.user.id : character.id,
        text: item.text ?? "", attachmentRefs: item.mediaId ? [item.mediaId] : [], createdAt: item.at && item.at.includes(":") && item.at.length <= 5 ? `2026-10-15T${item.at}:00+08:00` : "2026-10-15T09:00:00+08:00",
        identity: item.identity ? IDENTITY_OUT[item.identity] : undefined, kind: messageKind(item), recall: Boolean(item.recall),
        memory_ref: item.memoryRef ?? null, goal_ref: item.goalRef ?? null, action_id: item.actionId ?? null, link: item.link ?? null, media_id: item.mediaId ?? null,
        display_at: item.at,
      })),
      nextBeforeSequence: null,
    };
    moments[character.id] = Object.values(seed.posts).filter((post) => post.author === character.id || post.worldId === character.worldId).map((post) => ({
      id: post.id, text: post.text, author_actor_id: post.author, author_type: post.authorType === "char" ? "fluctlight" : post.authorType === "user" ? "user" : "npc",
      author_name: post.authorType === "user" ? seed.user.name : seed.characters[post.author]?.name || post.author,
      author_avatar_url: assetUrl(post.authorType === "user" ? seed.user.avatar : seed.characters[post.author]?.avatar || seed.npcs[post.author]?.avatar || ""),
      source_action_id: post.sourceAction ?? null, created_at: post.at, world_id: post.worldId, media_id: post.mediaId ?? null,
      reaction_count: post.likes, viewer_reaction: post.liked ? "like" : null,
      comments: post.comments.map((comment) => ({ id: comment.id, author_actor_id: comment.author, text: comment.text, created_at: comment.at })),
      refs: post.refs ?? null, status: "visible",
    }));
  }
  const seenPosts = new Map();
  for (const list of Object.values(moments)) for (const post of list) seenPosts.set(post.id, post);
  return {
    fluctlights, groups: Object.values(seed.groups), details, sidebars, wardrobes, media, diaries, nearby, worlds, templates, desires, reflections, autonomy, conversations, goals,
    moments: [...seenPosts.values()],
    notifications: { items: seed.notices.map((notice) => ({ id: notice.id, title: notice.text, body: notice.text, link: notice.link, read_at: notice.read ? "2026-10-15T09:00:00+08:00" : null, created_at: "2026-10-15T09:00:00+08:00", source_kind: "notice", source_id: notice.id })), next_cursor: null },
    unread: { count: seed.notices.filter((notice) => !notice.read).length },
    resources: { items: Object.values(seed.resources).map((resource) => ({
      id: resource.id, kind: KIND[resource.type] || "place", name: resource.name, published_version: resource.published, draft_version: resource.draft,
      versions: Object.entries(resource.versions).map(([version, body]) => ({ version, created_at: "2026-10-01T00:00:00+08:00", body })),
      referenced_by: Object.entries(seed.instances).flatMap(([fid, applied]) => applied[resource.id] ? [{ fluctlight_id: fid, running_version: applied[resource.id].applied, overrides: applied[resource.id].overrides }] : []),
    })), next_cursor: null },
    previews: { items: seed.studioDrafts.map((draft) => ({ preview_id: draft.id, text: draft.text, label: "创作预览·不进入生活", life_write: false, created_at: draft.at, resource_id: draft.resourceId ?? null })), next_cursor: null },
    harm: seed.worldSettings.w1 ? { count: seed.characters.c1.harmIntercepts, window: "7d", latest_reason: seed.worldSettings.w1.intercepts.at(-1)?.rule ?? null, items: seed.worldSettings.w1.intercepts } : undefined,
    pendingCount: { count: Object.values(seed.proposals).filter((item) => item.charId === "c1" && item.state === "候选").length + Object.values(seed.resources).filter((resource) => Object.values(seed.instances).some((applied) => applied[resource.id] && applied[resource.id].applied !== resource.published)).length + Object.values(seed.tasks).filter((task) => task.charId === "c1" && task.state === "失败").length, partial: false },
    settings: { values: seed.prefs.global, configuredSecrets: [], runtime_effect: "preference_only" },
    owner: { id: seed.user.id, name: seed.user.name, avatar_url: assetUrl(seed.user.avatar) },
    tasks: Object.fromEntries(Object.values(seed.tasks).map((task) => [task.id, { task_id: task.id, type: "action", title: task.title, initiator: "runtime", state: task.state, runs: task.runs, correlation_id: task.id, fluctlight_id: task.charId, action_id: task.actionId, steps: task.steps.map((step) => ({ n: step.n, name: step.name, status: step.state, idempotency_key: `${task.id}-${step.n}`, error_code: step.err ? "timeout" : null, error_message: step.err ?? null })) }])),
  };
}

function itemStatus(item, used) {
  if (item.inbound_status === "pending" || item.inbound_status === "failed") return "入库中";
  if (used.has(item.id)) return "使用";
  if (item.ownership_label === "借用" || item.ownership === "borrowed") return "借用";
  if (item.ownership_label === "未拥有" || item.ownership === "absent") return "未拥有";
  return "拥有";
}

export function directoryGroupProjection(characters,rows) {
  const projected=Object.fromEntries(Object.entries(characters).map(([id,character])=>[id,{...character,groupId:'ungrouped'}]));
  const groups={};
  for(const group of rows){
    groups[group.id]={id:group.id,name:group.name};
    for(const actorId of group.actor_ids||group.members||[])if(projected[actorId])projected[actorId].groupId=group.id;
  }
  if(Object.values(projected).some(c=>c.groupId==='ungrouped'))groups.ungrouped={id:'ungrouped',name:'未分组'};
  return {characters:projected,groups};
}

export function viewFromBundle(bundle) {
  const data = {
    meta: { seedVersion: 4, seq: 100, unwired: {}, wardrobeRevisions: {}, ownerRevisions: {}, studioApplyNotes: {} },
    ui: { charId: bundle.fluctlights?.[0]?.id || "", identity: "交流", notes: false },
    groups: {}, characters: {}, worlds: {}, user: { id: bundle.session?.actorId || bundle.owner?.id || "", name: bundle.owner?.name || "我", avatar: bundle.owner?.avatar_url || "/v3/assets/avatars/user.svg", background: {} },
    places: {}, npcs: {}, relations: {}, messages: {}, memories: {}, goals: {}, schedule: {}, actions: {}, items: {}, outfit: {}, media: {}, events: {}, diary: {}, tasks: {}, proposals: {}, resources: {}, instances: {}, studioDrafts: [], posts: {}, postSuggestions: {}, reflections: {}, worldSettings: {}, prefs: { global: {}, autonomy: {} }, session: null, notices: [], audit: [], ownerRequests: {},
  };
  const mark = (key) => { data.meta.unwired[key] = true; };
  if (!bundle.fluctlights) mark("characters");
  for (const item of bundle.fluctlights || []) {
    const character = { id: item.id, name: item.identity?.name || item.name || item.id, avatar: item.avatar?.url || "", status: ({active:"正常",paused:"已暂停",retired:"已退休",created:"未启动",failed:"异常"}[item.status] || item.status), groupId: item.group_id || "ungrouped", worldId: item.world_id, bio: item.bio, traits: item.traits || [], locationId: item.location_id };
    if (item.def_version !== undefined) character.defVersion = item.def_version;
    if (Object.hasOwn(item, "harm_intercepts")) character.harmIntercepts = item.harm_intercepts;
    if (item.status_note) character.statusNote = item.status_note;
    if (item.now_line) character.doing = item.now_line;
    const detail = bundle.details?.[item.id];
    const state = detail?.current_state;
    if (state?.doing?.action_id) character.doingActionId = state.doing.action_id;
    else if (item.now_line) character.doingActionId = state?.doing?.action_id ?? null;
    if (state && Object.hasOwn(state, "mood") && state.mood) character.mood = { value: state.mood.label, state: state.mood.state, evidence: state.mood.evidence };
    if (state && Object.hasOwn(state, "momentum") && state.momentum) character.energy = { value: state.momentum.label, state: state.momentum.state, evidence: state.momentum.evidence };
    if (detail?.context) {
      character.locationText = detail.context.location;
      character.doing = detail.context.activity;
      character.contextSource = detail.context.source;
    }
    if (detail?.avatar?.url) character.avatar = detail.avatar.url;
    if (detail?.mood) character.mood = { value: detail.mood.label, state: detail.mood.source, ...(detail.mood.evidence ? { evidence: detail.mood.evidence } : {}) };
    if (detail?.momentum) character.energy = { value: detail.momentum.label ?? detail.momentum.value, state: detail.momentum.source, ...(detail.momentum.evidence ? { evidence: detail.momentum.evidence } : {}) };
    data.characters[item.id] = character;
    for (const event of detail?.events || []) {
      const row = { id: event.id, worldId: event.world_id, title: event.title || event.activity || event.kind, at: event.at || event.start_at, placeId: event.place_id, state: event.state || event.status, impacts: event.impacts || [] };
      if (!event.world_id) row.charId = item.id;
      if (event.location !== undefined) row.location = event.location;
      data.events[event.id] = row;
    }
    for (const entry of detail?.schedule_items || detail?.schedule?.items || []) {
      const row = { id: entry.id, charId: entry.char_id || item.id, time: entry.time || [entry.start_at, entry.end_at].filter(Boolean).map(clock).join("–"), title: entry.title || entry.activity, placeId: entry.place_id, state: entry.state || entry.status, fact: entry.fact, factState: entry.fact_state };
      copyDefined(row, "actionId", entry.action_id || undefined);
      copyDefined(row, "goalId", entry.goal_id || undefined);
      copyDefined(row, "adjustedBy", entry.adjusted_by || undefined);
      copyDefined(row, "note", entry.note || undefined);
      data.schedule[entry.id] = row;
    }
    for (const memory of detail?.memories || []) {
      const row = { id: memory.id, charId: memory.owner_fluctlight_id || memory.fluctlight_id || item.id, content: memory.content, state: memory.status || memory.state, kind: memory.type || memory.kind, version: memory.revision ?? memory.version, revisions: (memory.revisions || []).map((revision) => {
        const itemRevision = { v: revision.v, at: revision.at, text: revision.text, by: BY_IN[revision.by] || revision.by };
        if (revision.source_ref) itemRevision.source = revision.source_ref;
        if (revision.note) itemRevision.note = revision.note;
        return itemRevision;
      }) };
      if (memory.source_message_id) row.sourceMsg = memory.source_message_id;
      if (memory.related_place_id) row.relatedPlace = memory.related_place_id;
      if (memory.related_npc_id) row.relatedNpc = memory.related_npc_id;
      if (memory.source_event_id) row.sourceEvent = memory.source_event_id;
      data.memories[memory.id] = row;
    }
    for (const relation of detail?.relationships || []) data.relations[relation.id] = {
      id: relation.id, charId: relation.fluctlight_id || item.id, npcId: relation.npc_id || relation.target_actor_id, stage: relation.stage || relation.summary || relation.target_actor_type, intimacy: relation.intimacy, version: relation.revision ?? relation.version,
      ...(relation.target_actor_id ? { targetActorId: relation.target_actor_id, targetActorType: relation.target_actor_type, role: relation.role, metrics: relation.metrics, provenance: relation.provenance } : {}),
      revisions: (relation.revisions || []).map((revision) => {
        const itemRevision = { v: revision.v, at: revision.at, text: revision.text, by: BY_IN[revision.by] || revision.by };
        if (revision.source_ref) itemRevision.source = revision.source_ref;
        if (revision.note) itemRevision.note = revision.note;
        return itemRevision;
      }),
    };
    for (const action of detail?.actions || []) data.actions[action.id] = action;
    for (const proposal of detail?.proposals || []) data.proposals[proposal.id] = proposal;
    const wardrobe = bundle.wardrobes?.[item.id];
    if (wardrobe) {
      if (typeof wardrobe.revision === "number") data.meta.wardrobeRevisions[item.id] = wardrobe.revision;
      if (typeof wardrobe.owner_revision === "number") data.meta.ownerRevisions[item.id] = wardrobe.owner_revision;
      const used = new Set((wardrobe.used_items || []).map((entry) => entry.item_id));
      for (const entry of wardrobe.items || []) {
        const row = { id: entry.id, charId: item.id, name: entry.name || entry.description, cat: entry.category_label || entry.category, img: entry.image_url, status: itemStatus(entry, used), source: entry.source_label || "" };
        if (entry.source_message_id) row.sourceMsg = entry.source_message_id;
        if (entry.owner_actor_id) row.owner = entry.owner_actor_id;
        if (entry.source_event_id) row.sourceEvent = entry.source_event_id;
        if (entry.source_action_id) row.actionId = entry.source_action_id;
        if (entry.goal_id) row.goalId = entry.goal_id;
        data.items[entry.id] = row;
      }
      if (Array.isArray(wardrobe.worn_items)) {
        const outfit = { current: wardrobe.worn_items.map((entry) => entry.item_id), since: wardrobe.worn_items[0]?.since, history: [] };
        const recordedBy = Object.fromEntries(wardrobe.worn_items.filter((entry) => entry.recorded_by === "owner" || entry.recorded_by === "fluctlight").map((entry) => [entry.item_id, entry.recorded_by]));
        if (Object.keys(recordedBy).length) outfit.recordedBy = recordedBy;
        data.outfit[item.id] = outfit;
      }
      for (const entry of wardrobe.used_items || []) {
        if ((entry.recorded_by === "owner" || entry.recorded_by === "fluctlight") && data.items[entry.item_id]) data.items[entry.item_id].recordedBy = entry.recorded_by;
      }
    }
    for (const entry of bundle.media?.[item.id]?.items || []) {
      const row = { id: entry.asset_id || entry.id, charId: item.id, img: entry.url, title: entry.title, nonFact: entry.factual === false, sourceMsg: entry.source_message_id || undefined };
      if (!row.sourceMsg) delete row.sourceMsg;
      if (entry.place_id) row.placeId = entry.place_id;
      if (entry.item_id) row.itemId = entry.item_id;
      if (entry.outfit_snapshot) row.outfitSnapshot = entry.outfit_snapshot;
      if (entry.overlay_url) row.overlay = entry.overlay_url;
      if (!row.id) throw new Error("媒体响应缺少稳定 ID");
      data.media[row.id] = row;
    }
    for (const entry of bundle.diaries?.[item.id]?.items || []) data.diary[entry.id] = { id: entry.id, charId: item.id, date: entry.date, text: entry.body, refs: entry.refs || [], replies: entry.replies || [] };
    const world = bundle.worlds?.[item.id];
    if (world?.world) {
      data.worlds[world.world.id] = { id: world.world.id, name: world.world.name, ...(world.world.def_version ? { defVersion: world.world.def_version } : {}) };
      for (const place of world.places || []) {
        const row = { id: place.id, name: place.name, scene: place.scene_asset?.url || "", desc: place.desc, worldId: world.world.id };
        if (place.resource_id) row.resourceId = place.resource_id;
        data.places[place.id] = row;
      }
      for (const npc of world.npcs || []) data.npcs[npc.id] = { id: npc.id, full: npc.full_fluctlight === true, bio: npc.bio, name: npc.name, avatar: npc.avatar_url, role: npc.role, placeId: npc.place_id };
    }
    const template = bundle.templates?.[item.id];
    const harmReady = bundle.harm && bundle.harm.available !== false && bundle.harm.status !== "not_implemented" && typeof bundle.harm.count === "number";
    if (template?.frequency && data.worlds[item.world_id] && harmReady) {
      data.worldSettings[item.world_id] = { randomEvents: template.random_events, freq: template.frequency, cats: Object.fromEntries((template.items || []).map((entry) => [entry.category, entry.enabled])), harmLocked: true, intercepts: bundle.harm.items || [] };
    }
    for (const entry of bundle.desires?.[item.id]?.items || []) {
      const row = { id: entry.id, charId: item.id, type: "意向", title: entry.text, state: entry.status };
      if (entry.why) row.why = entry.why;
      if (entry.schedule_id) row.scheduleId = entry.schedule_id;
      if (entry.action_id) row.actionId = entry.action_id;
      if (entry.item_id) row.itemId = entry.item_id;
      if (entry.source_message_id) row.sourceMsg = entry.source_message_id;
      data.goals[entry.id] = row;
    }
    for (const entry of bundle.goals?.[item.id]?.items || []) {
      const row = { id: entry.id, charId: item.id, type: entry.type || "目标", title: entry.desired_outcome || entry.title, state: entry.status };
      if (entry.progress) row.progress = entry.progress;
      if (entry.due) row.due = entry.due;
      if (entry.why) row.why = entry.why;
      data.goals[entry.id] = row;
    }
    for (const entry of bundle.reflections?.[item.id]?.items || []) data.reflections[entry.id] = { id: entry.id, charId: item.id, at: entry.at, title: entry.title, text: entry.body, state: entry.state, growth: entry.growth, sources: entry.source_refs || [], feedback: entry.feedback ?? null };
    const policy = bundle.autonomy?.[item.id];
    if (policy) data.prefs.autonomy[item.id] = { 自治程度: policy.level, 主动触达: OUTREACH_IN[policy.outreach_limit] || policy.outreach_limit, 免打扰: policy.quiet_hours ? `${policy.quiet_hours.start}-${policy.quiet_hours.end}` : "", 允许发动态: policy.allow_moments };
    const conversation = bundle.conversations?.[item.id];
    for (const message of conversation?.messages || []) {
      const row = { id: message.id, charId: item.id, from: message.kind === "divider" ? "divider" : message.kind === "system" ? "sys" : message.kind === "action" ? "action" : message.authorActorId === (bundle.session?.actorId || conversation.conversation?.createdByActorId || bundle.owner?.id) ? "user" : "char", at: message.display_at || clock(message.createdAt) };
      if (message.text && message.kind !== "action") row.text = message.text;
      if (message.identity) row.identity = IDENTITY[message.identity] || message.identity;
      if (message.recall) row.recall = true;
      if (message.memory_ref) row.memoryRef = message.memory_ref;
      if (message.goal_ref) row.goalRef = message.goal_ref;
      if (message.action_id) row.actionId = message.action_id;
      if (message.link) row.link = message.link;
      if (message.kind === "image") row.kind = "image";
      if (message.media_id) row.mediaId = message.media_id;
      data.messages[message.id] = row;
    }
  }
  if(bundle.groups)Object.assign(data,directoryGroupProjection(data.characters,bundle.groups));
  if (Object.values(data.characters).some(c => c.groupId === "ungrouped")) data.groups.ungrouped = { id: "ungrouped", name: "未分组" };
  for (const post of bundle.moments || []) {
    const row = { id: post.id, worldId: post.world_id, author: post.author_actor_id, authorType: post.author_type === "fluctlight" ? "char" : post.author_type, text: post.text, at: post.created_at, likes: post.reaction_count ?? 0, liked: Boolean(post.viewer_reaction), comments: (post.comments || []).map((comment) => ({ id: comment.id, author: comment.author_actor_id, text: comment.text, at: comment.created_at })) };
    if (post.media_id) row.mediaId = post.media_id;
    if (post.source_action_id) row.sourceAction = post.source_action_id;
    if (post.refs) row.refs = post.refs;
    data.posts[post.id] = row;
  }
  const notes = bundle.notifications;
  if (!notes || notes.status === "not_implemented" || notes.available === false) mark("notices");
  else data.notices = (notes.items || []).map((notice) => ({ id: notice.id, text: notice.body || notice.title, link: notice.link, read: Boolean(notice.read_at) }));
  if (bundle.settings?.values) data.prefs.global = { ...bundle.settings.values };
  for (const resource of bundle.resources?.items || []) {
    data.resources[resource.id] = { id: resource.id, type: KIND_IN[resource.kind] || resource.kind, name: resource.name, published: resource.published_version, draft: resource.draft_version, versions: Object.fromEntries((resource.versions || []).map((version) => [version.version, version.body || {}])) };
    for (const applied of resource.referenced_by || []) {
      if (applied?.status === "recorded_not_applied" || applied?.applied === false) {
        data.meta.studioApplyNotes[resource.id] = "未完成·已记录选择，尚未写入实例";
        continue;
      }
      if (!applied?.running_version) continue;
      data.instances[applied.fluctlight_id] = data.instances[applied.fluctlight_id] || {};
      const instance = { applied: applied.running_version, overrides: applied.overrides || {} };
      if (typeof applied.instance_revision === "number") instance.revision = applied.instance_revision;
      data.instances[applied.fluctlight_id][resource.id] = instance;
    }
  }
  data.studioDrafts = (bundle.previews?.items || []).map((preview) => ({ id: preview.preview_id, resourceId: preview.resource_id, at: preview.created_at, text: preview.text, label: preview.label }));
  for (const [id, task] of Object.entries(bundle.tasks || {})) data.tasks[id] = { id, charId: task.fluctlight_id, actionId: task.action_id, title: task.title, state: task.state, runs: task.runs, steps: (task.steps || []).map((step) => ({ n: step.n, name: step.name, state: step.status, ...(step.error_message ? { err: step.error_message } : {}) })) };
  if (!bundle.harm || bundle.harm.available === false || bundle.harm.status === "not_implemented" || typeof bundle.harm.count !== "number") mark("harm");
  data.meta.unreadCount = bundle.unread?.count;
  data.meta.alertCount = bundle.unread?.alert_count;
  data.meta.pendingGroups = bundle.pending?.groups?.map(group=>({...group,items:group.items?.map(item=>({...item,charId:item.fluctlight_id,text:item.title,link:item.href}))}));
  data.meta.pendingCount = bundle.pendingCount?.count;
  data.meta.pendingPartial = bundle.pendingCount?.partial;
  data.meta.conversations = Object.fromEntries(Object.entries(bundle.conversations || {}).map(([id, page]) => [id, page.conversation?.id]).filter(([, id]) => id));
  return data;
}

export function rewriteSeedAssets(value) {
  if (typeof value === "string") return assetUrl(value);
  if (Array.isArray(value)) return value.map((item) => rewriteSeedAssets(item));
  if (value && typeof value === "object") return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, rewriteSeedAssets(item)]));
  return value;
}
