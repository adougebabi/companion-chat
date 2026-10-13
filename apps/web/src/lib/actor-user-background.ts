import type { BrowserActorUserSettings } from "@fluctlight/browser-client";

export const actorUserBackgroundFields = [
  { key: "name", label: "称呼" },
  { key: "occupation", label: "职业" },
  { key: "background", label: "简单背景" },
  { key: "interests", label: "稳定兴趣" },
  { key: "preferences", label: "偏好" },
  { key: "location_scope", label: "所在地概况" },
  { key: "location", label: "具体所在地" },
  { key: "timezone", label: "时区（如 Asia/Shanghai）" },
  { key: "relationship_distance", label: "双方距离或交流方式" },
  { key: "meeting_confirmed", label: "是否已约定线下见面" },
] as const;

export function backgroundFromDraft(draft: Record<string, string>): BrowserActorUserSettings["background"] {
  return Object.fromEntries(actorUserBackgroundFields.map(({ key }) => [key,
    key === "meeting_confirmed" ? draft[key] === "true" ? true : draft[key] === "false" ? false : null : draft[key]?.trim() || null,
  ]));
}

export function backgroundDraft(value: BrowserActorUserSettings["background"]): Record<string, string> {
  return Object.fromEntries(actorUserBackgroundFields.map(({ key }) => [key,
    key === "meeting_confirmed" ? value[key] === true ? "true" : value[key] === false ? "false" : "unknown" : value[key] ?? "",
  ]));
}

export function parseActorUserSettings(value: unknown): BrowserActorUserSettings | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  const settings = value as Record<string, unknown>;
  if (Object.keys(settings).length !== 1 || !settings.background || typeof settings.background !== "object" || Array.isArray(settings.background)) return null;
  const background = settings.background as Record<string, unknown>;
  for (const [key, child] of Object.entries(background)) {
    if (!actorUserBackgroundFields.some(field => field.key === key)) return null;
    if (child === null) continue;
    if (key === "meeting_confirmed") { if (typeof child !== "boolean") return null; }
    else if (typeof child !== "string" || !child.trim() || [...child].length > 1024) return null;
  }
  return settings as BrowserActorUserSettings;
}

export const actorUserInitializationExample = {
  actor_user: { background: { name: "你的称呼", occupation: null, background: null, location_scope: "在国外", location: null, timezone: null, relationship_distance: "异地交流", meeting_confirmed: false } },
};
