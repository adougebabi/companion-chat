export type WorkspaceView = "chat" | "moments" | "instances" | "diagnostics" | "settings";
export type SettingsSection = "model-role" | "endpoint" | "binding" | "media" | "operations" | "owner";
export type DiagnosticsSection = "model-runs" | "events" | "workflows";
export type WorkspaceSection = SettingsSection | DiagnosticsSection;

export const settingsSections = [
  { id: "model-role", label: "模型角色绑定", description: "为不同认知角色选择模型和预算" },
  { id: "endpoint", label: "模型 Endpoint", description: "管理模型服务地址和协议" },
  { id: "binding", label: "当前角色绑定", description: "查看每个角色当前使用的模型" },
  { id: "media", label: "ComfyUI", description: "图片生成服务与工作流" },
  { id: "operations", label: "运行策略", description: "自治行为与诊断保留" },
  { id: "owner", label: "所有者", description: "密码和本地会话" },
] as const satisfies ReadonlyArray<{ id: SettingsSection; label: string; description: string }>;

export const diagnosticsSections = [
  { id: "model-runs", label: "模型运行", description: "最近 20 条模型调用记录" },
  { id: "events", label: "系统事件", description: "最近 20 条脱敏系统事件" },
  { id: "workflows", label: "工作流控制", description: "排查运行时工作流状态" },
] as const satisfies ReadonlyArray<{ id: DiagnosticsSection; label: string; description: string }>;

export type WorkspaceRoute =
  | { view: "chat"; fluctlightId?: string }
  | { view: "moments"; scope?: "global" | "fluctlight"; fluctlightId?: string }
  | { view: "instances"; mode?: "list" | "create" }
  | { view: "diagnostics"; correlationId?: string; section?: DiagnosticsSection }
  | { view: "settings"; section?: SettingsSection };

export const primaryNavigation = [
  { id: "instances", label: "聊天", icon: "◉" },
  { id: "moments", label: "动态", icon: "✦" },
  { id: "settings", label: "设置", icon: "⌘" },
  { id: "diagnostics", label: "诊断中心", icon: "⌁" },
] as const satisfies ReadonlyArray<{ id: WorkspaceView; label: string; icon: string }>;
