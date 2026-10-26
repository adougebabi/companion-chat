import {InlineFeedback as OperationStatus} from "@/react-app/shared/Presentation";
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
    <div className="error" role="alert">
      {reason}
      {store.retrying && <OperationStatus variant="default" role="status">正在提交重试请求并等待回复</OperationStatus>}
      <Button
        variant="ghost"
        disabled={store.sending || store.retrying}
        onClick={() =>
          message.turnRetryable
            ? store.retryMessage(message.id)
            : store.refreshActiveTurn(true)
        }
      >
        <RotateCcwIcon size={12} />
        {store.retrying
          ? "重试中…"
          : message.turnRetryable
            ? "重试原回应"
            : "查看状态"}
      </Button>
    </div>
  );
}
