import {RefreshCwIcon} from 'lucide-react';
import {InlineFeedback} from "@/react-app/shared/Presentation";
import { RotateCcwIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { chatFailureText } from "@/v3/chat-feedback.mjs";
import type { BrowserMessage } from "@fluctlight/browser-client";
import type { ConversationController } from "@/state/conversation";
export default function ChatTurnFeedback({
  message,
  messages,
  store,
}: {
  message: Readonly<
    Pick<BrowserMessage, "id" | "turnErrorCode" | "turnId" | "turnRetryable">
  >;
  messages: readonly Readonly<Pick<BrowserMessage, "kind" | "turnId">>[];
  store: ConversationController;
}) {
  const reason =
    message.turnErrorCode === "agent_partial_committed"
      ? "部分操作已执行，不能直接重试。请先查看实际结果。"
      : chatFailureText(
          message.turnErrorCode || store.error,
          messages.some(
            (row) => row.kind === "assistant" && row.turnId === message.turnId,
          ),
        );
  return (
    <InlineFeedback variant="default" className="mt-2 border-border bg-muted/40 text-foreground" role="alert">
      {reason}
      {store.retrying && <p className="m-0 text-sm" role="status">正在提交重试请求并等待回复</p>}
      <Button
        variant="outline"
        className="ml-2 w-fit"
        disabled={store.sending || store.retrying}
        onClick={() =>
          message.turnRetryable
            ? store.retryMessage(message.id)
            : store.refreshActiveTurn(true)
        }
      >
        {message.turnRetryable?<RotateCcwIcon size={14}/>:<RefreshCwIcon size={14}/>}
        {store.retrying
          ? "重试中…"
          : message.turnRetryable
            ? "重试原回应"
            : "查看状态"}
      </Button>
    </InlineFeedback>
  );
}
