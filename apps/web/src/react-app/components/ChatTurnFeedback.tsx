import {RefreshCwIcon} from 'lucide-react';
import {Feedback} from "@/react-app/shared/Presentation";
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
    Pick<BrowserMessage, "id" | "turnErrorCode" | "turnId" | "turnRetryable" | "turnStatus">
  >;
  messages: readonly Readonly<Pick<BrowserMessage, "kind" | "turnId">>[];
  store: ConversationController;
}) {
  const settling=message.turnRetryable===false&&message.turnErrorCode!=='agent_partial_committed';
  const reason = settling?'这次回应的结束状态仍在核对，原消息会保留。':
    message.turnErrorCode === "agent_partial_committed"
      ? "部分操作已执行，不能直接重试。请先查看实际结果。"
      : chatFailureText(
          message.turnErrorCode || store.error,
          messages.some(
            (row) => row.kind === "assistant" && row.turnId === message.turnId,
          ),
        );
  return (
    <Feedback kind={settling?'progress':message.turnErrorCode==='agent_partial_committed'?'warning':message.turnStatus==='cancelled'?'information':'error'} className="mt-2" details={message.turnErrorCode&&<pre>{JSON.stringify({code:message.turnErrorCode,turnId:message.turnId},null,2)}</pre>}>
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
    </Feedback>
  );
}
