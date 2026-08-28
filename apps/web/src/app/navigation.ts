export type WorkspaceView = "chat" | "moments" | "instances" | "diagnostics" | "settings";

export type WorkspaceRoute =
  | { view: "chat"; fluctlightId?: string }
  | { view: "moments"; scope?: "global" | "fluctlight"; fluctlightId?: string }
  | { view: "instances"; mode?: "list" | "create" }
  | { view: "diagnostics"; correlationId?: string }
  | { view: "settings"; section?: "models" | "media" | "operations" | "owner" };

export const primaryNavigation = [
  { id: "instances", label: "聊天", icon: "◉" },
  { id: "moments", label: "动态", icon: "✦" },
  { id: "settings", label: "设置", icon: "⌘" },
  { id: "diagnostics", label: "诊断中心", icon: "⌁" },
] as const satisfies ReadonlyArray<{ id: WorkspaceView; label: string; icon: string }>;
