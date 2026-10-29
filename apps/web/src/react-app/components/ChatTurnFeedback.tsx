import {RefreshCwIcon} from 'lucide-react';
import {useEffect,useRef,useState} from 'react';
import {Feedback} from "@/react-app/shared/Presentation";
import { RotateCcwIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { chatFailureText } from "@/v3/chat-feedback.mjs";
import {errorMessage,stateLabel,feedbackDiagnostic} from '@/v3/domain-presentation.mjs';
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
  const [checking,setChecking]=useState(false),[checkNote,setCheckNote]=useState<{text:string;version:number}|null>(null),[checkError,setCheckError]=useState(''),[checkFailure,setCheckFailure]=useState<any>(null),readBusy=useRef(false),readEpoch=useRef(0);
  useEffect(()=>{setCheckNote(null);setCheckError('');setChecking(false);setCheckFailure(null);readBusy.current=false;return()=>{readEpoch.current++;readBusy.current=false;};},[message.id]);
  useEffect(()=>{if(!checkNote)return;const timer=setTimeout(()=>setCheckNote(current=>current===checkNote?null:current),6000);return()=>clearTimeout(timer);},[checkNote]);
  async function readStatus(){if(readBusy.current||!message.turnId)return;readBusy.current=true;const token=++readEpoch.current;setChecking(true);setCheckNote(null);setCheckError('');setCheckFailure(null);try{const status=await store.refreshActiveTurn(true,message.turnId);if(token===readEpoch.current)setCheckNote({text:status?'已核对：'+stateLabel(status)+'，原消息已保留。':'暂未读到本条回应的新状态，原消息会保留。',version:token});}catch(cause){if(token===readEpoch.current){setCheckError(errorMessage(cause));setCheckFailure(feedbackDiagnostic(cause));}}finally{if(token===readEpoch.current){readBusy.current=false;setChecking(false);}}}
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
    <Feedback kind={checking?'progress':checkError?'warning':settling?'progress':message.turnErrorCode==='agent_partial_committed'?'warning':message.turnStatus==='cancelled'?'information':'error'} className="mt-2" details={(message.turnErrorCode||checkFailure)&&<pre>{JSON.stringify({code:message.turnErrorCode,turnId:message.turnId,readFailure:checkFailure},null,2)}</pre>}>
      {reason}
      {checking&&<p role="status">正在核对这次回应的实际状态…</p>}{checkNote&&<p role="status">{checkNote.text}</p>}{checkError&&<p>{checkError}</p>}
      {store.retrying && <p className="m-0 text-sm" role="status">正在提交重试请求并等待回复</p>}
      {(message.turnRetryable||message.turnId)?<Button
        variant="outline"
        className="ml-2 w-fit"
        disabled={store.sending || store.retrying||checking}
        onClick={() =>
          message.turnRetryable
            ? store.retryMessage(message.id)
            : void readStatus()
        }
      >
        {message.turnRetryable?<RotateCcwIcon size={14}/>:<RefreshCwIcon size={14}/>}
        {checking?'核对中…':store.retrying
          ? "重试中…"
          : message.turnRetryable
            ? "重试原回应"
            : "查看状态"}
      </Button>:<p>这条历史回应暂时无法单独核对状态，原消息仍保留。</p>}
    </Feedback>
  );
}
