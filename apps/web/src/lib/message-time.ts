import type { BrowserMessage } from "@fluctlight/browser-client";
import { formatBeijingInstant } from "./instant.ts";

export function messageTimeLabel(message: BrowserMessage): string {
  // Sender clock fields remain original provenance; only the reading label changes.
  const sentAt = message.senderSentAt;
  return formatBeijingInstant(sentAt && Number.isFinite(Date.parse(sentAt)) ? sentAt : message.createdAt);
}
