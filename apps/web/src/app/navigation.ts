export type WorkspaceView = "chat" | "moments" | "instances" | "diagnostics" | "settings" | "studio" | "inbox" | "life" | "world" | "people" | "growth" | "media" | "profile" | "autonomy" | "pending" | "data" | "rollback" | "showcase" | "lifeShowcase";
export type LifeSection = "now" | "schedule" | "goals" | "wardrobe" | "diary";
export type SettingsSection = "model-role" | "endpoint" | "binding" | "media" | "operations" | "kev" | "owner" | "reply" | "vision";
export type DiagnosticsSection = "lifecycle" | "agent-runs" | "model-runs" | "media-prompts" | "events" | "kev-decisions" | "workflows";
export type WorkspaceSection = SettingsSection | DiagnosticsSection;

export const settingsSections = [
  { id:"kev",label:"Kev 决策",description:"开关、服务地址和判定策略" },
  { id: "model-role", label: "模型角色绑定", description: "为认知、回复、视觉和 Embedding 选择模型" },
  { id: "reply", label: "私聊回复模型", description: "Fact-only 模式与独立采样参数" },
  { id: "vision", label: "视觉理解能力", description: "可选看图与设定卡校验，不控制媒体生成" },
  { id: "endpoint", label: "模型 Endpoint", description: "管理模型服务地址和协议" },
  { id: "binding", label: "当前角色绑定", description: "查看各个绑定目标当前使用的模型" },
  { id: "media", label: "ComfyUI", description: "图片生成服务与工作流" },
  { id: "operations", label: "运行策略", description: "自治行为与诊断保留" },
  { id: "owner", label: "所有者", description: "密码和本地会话" },
] as const satisfies ReadonlyArray<{ id: SettingsSection; label: string; description: string }>;

export const diagnosticsSections = [
  { id:"kev-decisions",label:"Kev 决策",description:"模型结论、实际应用与原始响应" },
  { id: "lifecycle", label: "生命周期", description: "按关联标识追踪触发、工作流、模型调用与下个周期" },
  { id: "agent-runs", label: "Agent 运行", description: "按发起时间查看逻辑 Agent 与终止记录" },
  { id: "model-runs", label: "模型运行", description: "查看逻辑 Agent、Tool 与各次模型调用的状态及失败原因" },
  { id: "media-prompts", label: "媒体提示词", description: "最近 20 条媒体生成提示词" },
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

export function isSettingsSection(value: unknown): value is SettingsSection { return settingsSections.some(section=>section.id===value); }
export function isDiagnosticsSection(value: unknown): value is DiagnosticsSection { return diagnosticsSections.some(section=>section.id===value); }

export const lifeSections = [
  { id: "now", label: "此刻" },
  { id: "schedule", label: "日程" },
  { id: "goals", label: "目标" },
  { id: "wardrobe", label: "衣柜" },
  { id: "diary", label: "日记" },
] as const satisfies ReadonlyArray<{ id: LifeSection; label: string }>;

export function isLifeSection(value: unknown): value is LifeSection {
  return lifeSections.some((section) => section.id === value);
}

/** Character workspace entries. `key` avoids treating chat as a legacy primary tab id. */
export const workspaceNavigation = [
  { key: "chat", label: "对话", view: "chat" },
  { key: "life", label: "生活", view: "life" },
  { key: "world", label: "世界", view: "world" },
  { key: "people", label: "人物关系", view: "people" },
  { key: "growth", label: "记忆成长", view: "growth" },
  { key: "media", label: "媒体", view: "media" },
  { key: "profile", label: "角色资料", view: "profile" },
  { key: "autonomy", label: "自治权限", view: "autonomy" },
] as const satisfies ReadonlyArray<{ key: string; label: string; view: WorkspaceView }>;

export const globalNavigation = [
  { key: "library", label: "角色库", view: "instances" },
  { key: "feed", label: "动态", view: "moments" },
  { key: "studio", label: "创作中心", view: "studio" },
  { key: "pending", label: "待处理", view: "pending" },
  { key: "data", label: "数据工具", view: "data" },
  { key: "settings", label: "系统设置", view: "settings" },
  { key: "diagnostics", label: "运行诊断", view: "diagnostics" },
  { key: "inbox", label: "通知", view: "inbox" },
] as const satisfies ReadonlyArray<{ key: string; label: string; view: WorkspaceView }>;

export const mobileNavigation: ReadonlyArray<{ key: string; label: string; view: WorkspaceView; life?: LifeSection }> = [
  { key: "chat", label: "对话", view: "chat" },
  { key: "now", label: "此刻", view: "life", life: "now" },
  { key: "wardrobe", label: "衣柜", view: "life", life: "wardrobe" },
  { key: "growth", label: "记忆", view: "growth" },
];
