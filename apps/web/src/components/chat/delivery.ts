import type { BrowserMessage } from "@fluctlight/browser-client";

export function deliveryStatus(message: BrowserMessage, context: { sending: boolean; queuedMessageId: string | null; canRetry: boolean; retryMessageId?: string; latestUserMessageId?: string }): "pending" | "failed" | "sent" | "none" {
  if (message.kind !== "user") return "none";
  if (message.turnStatus === "pending" || message.turnStatus === "running") return "pending";
  if (message.turnStatus === "failed" || message.turnStatus === "cancelled") return "failed";
  if (message.turnStatus === "completed") return "sent";
  if (context.queuedMessageId === message.id) return "pending";
  if (context.canRetry && context.retryMessageId === message.id) return "failed";
  return context.sending && context.latestUserMessageId === message.id ? "pending" : "sent";
}

export function deliveryLabel(message: BrowserMessage, status: "pending" | "failed" | "sent" | "none"): string {
  if (status === "pending") return "已接收，处理中";
  if (status === "failed") {
    if (message.turnErrorCode === "agent_partial_committed") return "部分操作已执行，不能直接重试";
    if (message.turnRetryable === false) return "本次生成正在结束";
    return message.turnStatus === "cancelled" ? "回复已取消，可重试" : "回复失败，可重试";
  }
  if (status === "sent") return "已回复";
  return "";
}

export function formatMessageText(text?: string | null): string {
  if (!text) return "";
  return text.replace(/\\r\\n/g, "\n").replace(/\\n/g, "\n");
}
