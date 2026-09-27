import type { BrowserMessage } from "@fluctlight/browser-client";

function offsetLabel(minutes: number): string {
  const sign = minutes < 0 ? "−" : "+";
  const absolute = Math.abs(minutes);
  return `UTC${sign}${String(Math.floor(absolute / 60)).padStart(2, "0")}:${String(absolute % 60).padStart(2, "0")}`;
}

export function messageTimeLabel(message: BrowserMessage): string {
  const offset = message.senderUtcOffsetMinutes;
  if (message.senderTimezone && typeof offset === "number" && message.senderSentAt) {
    const sentAt = Date.parse(message.senderSentAt);
    if (Number.isFinite(sentAt)) {
      // Freeze the original wall clock even if timezone rules change later.
      const wallClock = new Date(sentAt + offset * 60_000);
      const clock = new Intl.DateTimeFormat("zh-CN", { hour: "2-digit", minute: "2-digit", hourCycle: "h23", timeZone: "UTC" }).format(wallClock);
      return `${clock} · ${message.senderTimezone} (${offsetLabel(offset)})`;
    }
  }
  const clock = new Intl.DateTimeFormat("zh-CN", { hour: "2-digit", minute: "2-digit", hourCycle: "h23" }).format(new Date(message.createdAt));
  const viewerZone = Intl.DateTimeFormat().resolvedOptions().timeZone || "设备时区";
  return `${clock} · ${viewerZone}（旧消息，发送时区未知）`;
}
