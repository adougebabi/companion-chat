export type LifeRecord = Record<string, unknown>;

export type WardrobeFilter = "all" | "wearing" | "owned" | "borrowed" | "using" | "inbound";

export const wardrobeFilters: ReadonlyArray<{ id: WardrobeFilter; label: string }> = [
  { id: "all", label: "全部" },
  { id: "wearing", label: "穿着" },
  { id: "owned", label: "拥有" },
  { id: "borrowed", label: "借用" },
  { id: "using", label: "使用" },
  { id: "inbound", label: "入库中" },
];

const sourceLabels: Record<string, string> = {
  initialization: "初始化登记",
  accepted_event: "已接受事件",
  purchase_result: "购买结果",
};

export function asRecord(value: unknown): LifeRecord | null {
  return value && typeof value === "object" && !Array.isArray(value) ? value as LifeRecord : null;
}

export function asRecords(value: unknown): LifeRecord[] {
  return Array.isArray(value) ? value.filter((item): item is LifeRecord => Boolean(asRecord(item))) : [];
}

export function textField(value: unknown): string {
  if (typeof value === "number" && Number.isFinite(value)) return String(value);
  return typeof value === "string" && value.trim() ? value.trim() : "";
}

export function relationshipDraftKey(relationship: LifeRecord): string {
  return `${String(relationship.target_actor_id ?? "")}::${String(relationship.profile_id ?? "shared")}`;
}

export function appearanceOf(detail: LifeRecord | null): LifeRecord | null {
  const current = asRecord(detail?.current_state);
  return asRecord(current?.appearance);
}

/** null means the payload has no wearing list. An empty array is a real empty wearing state. */
export function wornItems(detail: LifeRecord | null): LifeRecord[] | null {
  const appearance = appearanceOf(detail);
  if (!appearance || !Object.prototype.hasOwnProperty.call(appearance, "worn_items")) return null;
  return asRecords(appearance.worn_items);
}

export function usedItems(detail: LifeRecord | null): LifeRecord[] | null {
  const appearance = appearanceOf(detail);
  if (!appearance || !Object.prototype.hasOwnProperty.call(appearance, "used_items")) return null;
  return asRecords(appearance.used_items);
}

export function idSet(items: LifeRecord[] | null): Set<string> | null {
  if (!items) return null;
  return new Set(items.map((item) => textField(item.id)).filter(Boolean));
}

export function sourceLabel(kind: unknown): string {
  const text = textField(kind);
  if (!text) return "来源未记录";
  return sourceLabels[text] ?? text;
}

export function wardrobeFilterAvailability(filter: WardrobeFilter, detail: LifeRecord | null): "ready" | "unwired" {
  if (filter === "inbound") return "unwired";
  if (filter === "wearing" && wornItems(detail) === null) return "unwired";
  if (filter === "using" && usedItems(detail) === null) return "unwired";
  return "ready";
}

export function wardrobeItemVisible(filter: WardrobeFilter, item: { id?: string; ownership?: string }, detail: LifeRecord | null): boolean {
  if (wardrobeFilterAvailability(filter, detail) === "unwired") return false;
  const worn = idSet(wornItems(detail));
  const used = idSet(usedItems(detail));
  const id = textField(item.id);
  if (filter === "all") return true;
  if (filter === "owned") return item.ownership === "owned";
  if (filter === "borrowed") return item.ownership === "borrowed";
  if (filter === "wearing") return Boolean(id && worn?.has(id));
  if (filter === "using") return Boolean(id && used?.has(id));
  return false;
}

export type MoodRead = { state: "missing" } | { state: "recorded"; label: string; evidence: string[] | null };

export function moodRead(detail: LifeRecord | null): MoodRead {
  const mood = asRecord(asRecord(detail?.inner_state)?.mood);
  const label = textField(mood?.label ?? mood?.value);
  if (!mood || !label) return { state: "missing" };
  if (!Object.prototype.hasOwnProperty.call(mood, "evidence")) return { state: "recorded", label, evidence: null };
  const evidence = Array.isArray(mood.evidence) ? mood.evidence.map((item) => textField(item)).filter(Boolean) : [];
  return { state: "recorded", label, evidence };
}

export function momentumRead(detail: LifeRecord | null): { state: "missing" } | { state: "recorded"; label: string } {
  const momentum = asRecord(asRecord(detail?.inner_state)?.momentum);
  const label = textField(momentum?.label ?? momentum?.value);
  if (!momentum || !label) return { state: "missing" };
  return { state: "recorded", label };
}

export function sceneImageUrl(detail: LifeRecord | null): string | null {
  const context = asRecord(detail?.context);
  for (const key of ["scene_image", "scene_url", "image_url"]) {
    const value = textField(context?.[key]);
    if (value) return value;
  }
  return null;
}

export function nowActivity(detail: LifeRecord | null): { activity: string; place: string; confirmed: boolean } {
  const context = asRecord(detail?.context);
  const presence = asRecord(context?.presence);
  const activity = textField(presence?.current_task) || textField(context?.activity);
  const place = textField(context?.location) || textField(context?.scene);
  return { activity, place, confirmed: Boolean(activity || place) };
}

export type ScheduleRead =
  | { state: "missing" }
  | { state: "empty" }
  | { state: "recorded"; id: string; revision: number | null; timezone: string; localDate: string; items: LifeRecord[] };

export function scheduleRead(detail: LifeRecord | null): ScheduleRead {
  if (!detail || !Object.prototype.hasOwnProperty.call(detail, "schedule")) return { state: "missing" };
  const schedule = asRecord(detail.schedule);
  if (!schedule) return { state: "empty" };
  const revision = typeof schedule.revision === "number" ? schedule.revision : null;
  return {
    state: "recorded",
    id: textField(schedule.id),
    revision,
    timezone: textField(schedule.timezone),
    localDate: textField(schedule.local_date ?? schedule.localDate),
    items: asRecords(schedule.items),
  };
}

export function recordedCommitments(detail: LifeRecord | null): { state: "missing" } | { state: "recorded"; items: LifeRecord[] } {
  if (!detail || !Object.prototype.hasOwnProperty.call(detail, "goals")) return { state: "missing" };
  const items: LifeRecord[] = [];
  for (const goal of asRecords(detail.goals)) {
    for (const commitment of asRecords(goal.commitments)) items.push({ ...commitment, goal_id: goal.id, goal_outcome: goal.desired_outcome });
  }
  return { state: "recorded", items };
}

export function goalProfileOptions(detail: LifeRecord | null): Array<{ id: string; label: string }> {
  const core = asRecord(detail?.core_persona);
  const system = asRecord(core?.personality_system) ?? asRecord(detail?.personality_system);
  return asRecords(system?.profiles).filter((profile) => textField(profile.id)).map((profile) => ({
    id: textField(profile.id),
    label: textField(profile.name) || textField(profile.id),
  }));
}

export function editableActorOptions(detail: LifeRecord | null, names: Array<{ id: string; name: string }>, ownerActorId: string | null): Array<{ id: string; label: string }> {
  const ids = Array.isArray(detail?.editable_actor_ids) ? detail.editable_actor_ids.map(String) : [];
  for (const relation of asRecords(detail?.relationships)) ids.push(String(relation.target_actor_id ?? ""));
  return [...new Set(ids.filter(Boolean))].map((id) => ({
    id,
    label: id === ownerActorId ? "你（Owner）" : (names.find((item) => item.id === id)?.name || id),
  }));
}
