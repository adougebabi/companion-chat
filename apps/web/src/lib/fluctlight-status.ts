const statusLabels: Record<string, string> = {
  active: "运行中",
  paused: "已暂停",
  retired: "已归档",
};

export function fluctlightStatusLabel(status: unknown): string {
  const normalized = String(status ?? "").trim();
  return statusLabels[normalized] ?? (normalized ? `状态：${normalized}` : "状态未知");
}
