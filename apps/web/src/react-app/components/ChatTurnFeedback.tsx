import {RefreshCwIcon} from 'lucide-react';
import {useEffect,useRef,useState} from 'react';
import {Feedback} from "@/react-app/shared/Presentation";
import { RotateCcwIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { DetailDrawer } from "../components";
import { Disclosure } from "../controls";
import { chatFailureText } from "@/v3/chat-feedback.mjs";
import {errorMessage,stateLabel,feedbackDiagnostic} from '@/v3/domain-presentation.mjs';
import type { BrowserMessage } from "@fluctlight/browser-client";
import type { ConversationController } from "@/state/conversation";
export default function ChatTurnFeedback({
  message,
  messages,
  store,
  onOpenDiagnostic,
}: {
  message: Readonly<
    Pick<BrowserMessage, "id" | "turnErrorCode" | "turnId" | "turnRetryable" | "turnStatus">
  >;
  messages: readonly Readonly<Pick<BrowserMessage, "kind" | "turnId">>[];
  store: ConversationController;
  onOpenDiagnostic?: (turnId:string)=>void;
}) {
  const [checking,setChecking]=useState(false),[checkNote,setCheckNote]=useState<{text:string;version:number}|null>(null),[checkError,setCheckError]=useState(''),[checkFailure,setCheckFailure]=useState<any>(null),readBusy=useRef(false),readEpoch=useRef(0);
  const [detailsOpen,setDetailsOpen]=useState(false);
  useEffect(()=>{setCheckNote(null);setCheckError('');setChecking(false);setCheckFailure(null);setDetailsOpen(false);readBusy.current=false;return()=>{readEpoch.current++;readBusy.current=false;};},[message.id]);
  useEffect(()=>{if(!checkNote)return;const timer=setTimeout(()=>setCheckNote(current=>current===checkNote?null:current),6000);return()=>clearTimeout(timer);},[checkNote]);
  async function readStatus(){if(readBusy.current||!message.turnId)return;readBusy.current=true;const token=++readEpoch.current;setChecking(true);setCheckNote(null);setCheckError('');setCheckFailure(null);try{const status=await store.refreshActiveTurn(true,message.turnId);if(token===readEpoch.current)setCheckNote({text:status?'已核对：'+stateLabel(status)+'，原消息已保留。':'暂未读到本条回应的新状态，原消息会保留。',version:token});}catch(cause){if(token===readEpoch.current){setCheckError(errorMessage(cause));setCheckFailure(feedbackDiagnostic(cause));}}finally{if(token===readEpoch.current){readBusy.current=false;setChecking(false);}}}
  const settling=message.turnRetryable===false&&message.turnErrorCode!=='agent_partial_committed';
  const reason = settling?message.turnStatus==='cancelled'
    ?'这次回应已停止，原消息已保留，暂时不能重试。'
    :'这次回应未完成，原消息已保留，暂时不能重试。':
    message.turnErrorCode === "agent_partial_committed"
      ? "部分操作已执行，不能直接重试。请先查看实际结果。"
      : chatFailureText(
          message.turnErrorCode || store.error,
          messages.some(
            (row) => row.kind === "assistant" && row.turnId === message.turnId,
          ),
        );
  return (
    <><Feedback kind={checking?'progress':checkError?'warning':settling?'warning':message.turnErrorCode==='agent_partial_committed'?'warning':message.turnStatus==='cancelled'?'information':'error'} className="mt-2" actions={<div className="ml-auto flex flex-nowrap items-center justify-end gap-3 whitespace-nowrap">
      {(message.turnRetryable||message.turnId)?<Button
        variant="outline"
        className="w-fit max-[700px]:min-h-[44px]"
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
      </Button>:null}
      {(message.turnErrorCode||checkFailure)&&<Button type="button" variant="ghost" size="sm" className="max-[700px]:min-h-[44px]" onClick={()=>setDetailsOpen(true)}>查看技术详情</Button>}
      </div>}>
      {reason}
      {!message.turnRetryable&&!message.turnId&&<p>这条历史回应暂时无法单独核对状态，原消息仍保留。</p>}
      {checking&&<p role="status">正在核对这次回应的实际状态…</p>}{checkNote&&<p role="status">{checkNote.text}</p>}{checkError&&<p>{checkError}</p>}
      {store.retrying && <p className="m-0 text-sm" role="status">正在提交重试请求并等待回复</p>}
    </Feedback>
    {detailsOpen&&<DetailDrawer title="回应检查结果" testId="chat-turn-details" onClose={()=>setDetailsOpen(false)}>
      <div className="grid min-w-0 gap-5">
        <div className="flex items-center justify-between gap-3"><h3 className="font-medium">本次回应</h3><Badge variant={message.turnStatus==='failed'?'destructive':'secondary'}>{stateLabel(message.turnStatus)}</Badge></div>
        <Card><CardContent className="grid gap-4 pt-5">
          <p>{reason}</p>
          <dl className="grid min-w-0 gap-3 text-sm">
            <div><dt className="text-muted-foreground">错误标识</dt><dd className="break-all font-mono">{message.turnErrorCode||'未记录'}</dd></div>
            <div><dt className="text-muted-foreground">关联回应</dt><dd className="break-all font-mono">{message.turnId||'未记录'}</dd></div>
            {checkError&&<div><dt className="text-muted-foreground">最近状态读取</dt><dd>{checkError}</dd></div>}
          </dl>
        </CardContent></Card>
        {message.turnId&&<Button variant="outline" asChild><a href={'#/diag/turns?correlationId='+encodeURIComponent('turn:'+message.turnId)} onClick={event=>{if(onOpenDiagnostic&&message.turnId){event.preventDefault();setDetailsOpen(false);onOpenDiagnostic(message.turnId);}}}>查看关联运行记录</a></Button>}
        <Disclosure title="原始 JSON"><pre className="approved-json whitespace-pre-wrap break-all">{JSON.stringify({code:message.turnErrorCode,turnId:message.turnId,readFailure:checkFailure},null,2)}</pre></Disclosure>
      </div>
    </DetailDrawer>}</>
  );
}
