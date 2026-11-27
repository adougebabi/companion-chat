import {RefreshCwIcon} from 'lucide-react';
import {useEffect,useRef,useState} from 'react';
import {Feedback} from "@/react-app/shared/Presentation";
import { RotateCcwIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { DetailDrawer } from "../components";
import { Disclosure } from "../controls";
import { DetailFields } from "@/components/patterns";
import { chatFailureText, chatFailureStage } from "@/v3/chat-feedback.mjs";
import {errorMessage,stateLabel,feedbackDiagnostic} from '@/v3/domain-presentation.mjs';
import type { BrowserMessage } from "@fluctlight/browser-client";
import type { ConversationController } from "@/state/conversation";
import TurnDetailsDrawer from "../shared/TurnTimeline";
export default function ChatTurnFeedback({
  message,
  messages,
  store,
}: {
  message: Readonly<
    Pick<BrowserMessage, "id" | "turnErrorCode" | "turnId" | "turnRetryable" | "turnReplyRetryable" | "turnStatus">
  >;
  messages: readonly Readonly<Pick<BrowserMessage, "kind" | "turnId">>[];
  store: ConversationController;
}) {
  const [checking,setChecking]=useState(false),[checkNote,setCheckNote]=useState<{text:string;version:number}|null>(null),[checkError,setCheckError]=useState(''),[checkFailure,setCheckFailure]=useState<any>(null),readBusy=useRef(false),readEpoch=useRef(0);
  const [detailsOpen,setDetailsOpen]=useState(false);
  useEffect(()=>{setCheckNote(null);setCheckError('');setChecking(false);setCheckFailure(null);setDetailsOpen(false);readBusy.current=false;return()=>{readEpoch.current++;readBusy.current=false;};},[message.id]);
  useEffect(()=>{if(!checkNote)return;const timer=setTimeout(()=>setCheckNote(current=>current===checkNote?null:current),6000);return()=>clearTimeout(timer);},[checkNote]);
  async function readStatus(){if(readBusy.current||!message.turnId)return;readBusy.current=true;const token=++readEpoch.current;setChecking(true);setCheckNote(null);setCheckError('');setCheckFailure(null);try{const status=await store.refreshActiveTurn(true,message.turnId);if(token===readEpoch.current)setCheckNote({text:status?'已核对：'+stateLabel(status)+'，原消息已保留。':'暂未读到本条回应的新状态，原消息会保留。',version:token});}catch(cause){if(token===readEpoch.current){setCheckError(errorMessage(cause));setCheckFailure(feedbackDiagnostic(cause));}}finally{if(token===readEpoch.current){readBusy.current=false;setChecking(false);}}}
  const replied=messages.some(row=>row.kind==='assistant'&&row.turnId===message.turnId);
  const settling=message.turnRetryable===false&&message.turnErrorCode!=='agent_partial_committed';
  const reason = settling?message.turnStatus==='cancelled'
    ?'这次回应已停止，原消息已保留，暂时不能重试。'
    :chatFailureText(message.turnErrorCode||store.error, messages.some(row=>row.kind==='assistant'&&row.turnId===message.turnId))+' 当前暂时不能重试，可先重新核对状态。':
    message.turnReplyRetryable
      ? "图片任务已受理，可以单独重试文字回复。"
      : message.turnErrorCode === "agent_partial_committed"
      ? "部分操作已执行，不能直接重试。请先查看实际结果。"
      : chatFailureText(
          message.turnErrorCode || store.error,
          messages.some(
            (row) => row.kind === "assistant" && row.turnId === message.turnId,
          ),
        );
  return (
    <><Feedback kind={checking?'progress':checkError?'warning':settling?'warning':message.turnErrorCode==='agent_partial_committed'?'warning':message.turnStatus==='cancelled'?'information':'error'} className="mt-2" actions={<div className="ml-auto flex flex-nowrap items-center justify-end gap-3 whitespace-nowrap">
      {(message.turnRetryable||message.turnReplyRetryable||message.turnId)?<Button
        variant="outline"
        className="w-fit max-[700px]:min-h-[44px]"
        disabled={store.sending || store.retrying||checking}
        onClick={() => {
          if (message.turnReplyRetryable) {
            setCheckError('');
            void store.retryReply(message.id).catch((cause) => setCheckError(errorMessage(cause)));
          } else if (message.turnRetryable) {
            void store.retryMessage(message.id);
          } else {
            void readStatus();
          }
        }}
      >
        {message.turnRetryable||message.turnReplyRetryable?<RotateCcwIcon size={14}/>:<RefreshCwIcon size={14}/>}
        {checking?'核对中…':store.retrying
          ? "重试中…"
          : message.turnReplyRetryable ? "仅重试回复" : message.turnRetryable
            ? "重试原回应"
            : "查看状态"}
      </Button>:null}
      {(message.turnId||message.turnErrorCode||checkFailure)&&<Button type="button" variant="ghost" size="sm" className="max-[700px]:min-h-[44px]" onClick={()=>setDetailsOpen(true)}>{message.turnId?'本轮详情':'查看技术详情'}</Button>}
      </div>}>
      {reason}
      {!message.turnRetryable&&!message.turnId&&<p>这条历史回应暂时无法单独核对状态，原消息仍保留。</p>}
      {checking&&<p role="status">正在核对这次回应的实际状态…</p>}{checkNote&&<p role="status">{checkNote.text}</p>}{checkError&&<p>{checkError}</p>}
      {store.retrying && <p className="m-0 text-sm" role="status">正在提交重试请求并等待回复</p>}
    </Feedback>
    {detailsOpen&&message.turnId&&<TurnDetailsDrawer turnId={message.turnId} onClose={()=>setDetailsOpen(false)}
      technical={{code:message.turnErrorCode,readFailure:checkFailure}}>
      <section className="grid min-w-0 gap-3" aria-label="本次回应">
        <div className="flex items-center justify-between gap-3"><h3 className="font-medium">本次回应</h3><Badge variant={message.turnStatus==='failed'?'destructive':'secondary'}>{stateLabel(message.turnStatus)}</Badge></div>
        <p className="m-0">{reason}</p>
        <DetailFields items={[
          {label:'消息结果',value:replied?'回复已发送，后续处理未完成':'本次回复未发送'},
          ...(message.turnStatus==='failed'?[{label:'失败环节',value:chatFailureStage(message.turnErrorCode||'')}]:[]),
          ...(checkError?[{label:'最近状态读取',value:checkError}]:[]),
        ]}/>
      </section>
    </TurnDetailsDrawer>}
    {detailsOpen&&!message.turnId&&<DetailDrawer title="回应检查结果" testId="chat-turn-details" onClose={()=>setDetailsOpen(false)}>
      <div className="grid min-w-0 gap-5">
        <div className="flex items-center justify-between gap-3"><h3 className="font-medium">本次回应</h3><Badge variant={message.turnStatus==='failed'?'destructive':'secondary'}>{stateLabel(message.turnStatus)}</Badge></div>
        <Card><CardContent className="grid gap-4 pt-5">
          <p>{reason}</p>
          <dl className="grid min-w-0 gap-3 text-sm">
            <div><dt className="text-muted-foreground">消息结果</dt><dd>{replied?'回复已发送，后续处理未完成':'本次回复未发送'}</dd></div>
            {message.turnStatus==='failed'&&<div><dt className="text-muted-foreground">失败环节</dt><dd>{chatFailureStage(message.turnErrorCode||'')}</dd></div>}
            <div><dt className="text-muted-foreground">错误标识</dt><dd className="break-all font-mono">{message.turnErrorCode||'未记录'}</dd></div>
            {checkError&&<div><dt className="text-muted-foreground">最近状态读取</dt><dd>{checkError}</dd></div>}
          </dl>
        </CardContent></Card>
        <Disclosure title="原始 JSON"><pre className="approved-json whitespace-pre-wrap break-all">{JSON.stringify({code:message.turnErrorCode,turnId:message.turnId,readFailure:checkFailure},null,2)}</pre></Disclosure>
      </div>
    </DetailDrawer>}</>
  );
}
