import {useState} from 'react';
import {Button} from '@/components/ui/button';
import {Card,CardContent,CardHeader,CardTitle} from '@/components/ui/card';
import {requireWritableCharacter} from '@/state/domain-guards.mjs';
import {useWorkspace} from '../page';
import {useDomainRead} from '../hooks/use-domain-read';
import {useCursorPage} from '../hooks/use-cursor-page';
import {Feedback} from '../shared/Presentation';
import CursorControls from '../shared/CursorControls';
import CommandDialog from '../shared/CommandDialog';

const labels:Record<string,string>={pending:'待审核',enabled:'使用中',disabled:'已停用',revoked:'已撤销'};

export default function ExpressionLearningPanel({fid}:{fid:string}){
 const w=useWorkspace(),owner=w.state.session.actorId,service=w.services.selfClient;
 const read=useDomainRead(()=>service.expressionLearning(fid),JSON.stringify([owner,fid,'expression-learning']));
 const sources=useCursorPage((cursor:string)=>service.expressionSources(fid,cursor),JSON.stringify([owner,fid,'expression-sources']));
 const [action,setAction]=useState<any>(null);
 const retired=['已退休','已归档','retired','archived'].includes(w.state.characters.find((row:any)=>row.id===fid)?.status);
 const reload=async()=>{await read.reload();await sources.reload();};
 const settings=read.data?.settings,items=Array.isArray(read.data?.items)?read.data.items:[];
 const identity=(...parts:any[])=>JSON.stringify([owner,fid,'expression-learning',...parts]);
 const storage=(...parts:any[])=>'fluctlight-studio:'+identity(...parts);
 function settingsAction(field:'learningEnabled'|'useEnabled'){
  if(!settings||retired||!read.fresh)return;
  const current=field==='learningEnabled'?settings.learning_enabled:settings.use_enabled,target=!current;
  setAction({identity:identity('settings',field,target),scope:[fid,'expression-settings',field,target],storageKey:storage('settings',field,target),title:(target?'开启':'关闭')+(field==='learningEnabled'?'表达学习':'表达参考'),description:field==='learningEnabled'?'学习从你选择的真实消息提取待审核候选。':'使用时只参考已审核、来源有效且符合当前语境的表达。',initial:{base:settings.revision,learningEnabled:settings.learning_enabled,useEnabled:settings.use_enabled,[field]:target},editable:[],validate:()=>requireWritableCharacter(w,fid),build:(draft:any)=>({learningEnabled:draft.learningEnabled,useEnabled:draft.useEnabled,expectedRevision:draft.base}),execute:(body:any,key:string)=>service.updateExpressionLearning(fid,body,key),readLatest:async()=> (await service.expressionLearning(fid)).settings,adoptLatest:(draft:any,fresh:any)=>({...draft,base:fresh.revision,learningEnabled:field==='learningEnabled'?target:fresh.learning_enabled,useEnabled:field==='useEnabled'?target:fresh.use_enabled}),onSaved:reload,snapshot:settings});
 }
 function extract(source:any){
  if(retired||!read.fresh||!settings?.learning_enabled)return;
  setAction({identity:identity('extract',source.id),scope:[fid,'expression-extract',source.id],storageKey:storage('extract',source.id),title:'从这条消息提取候选',description:'只分析这条真实消息。候选默认待审核，不会自动改变人格或使用。',initial:{},editable:[],validate:()=>requireWritableCharacter(w,fid),build:()=>({}),execute:(_:any,key:string)=>service.extractExpressionLearning(fid,source.id,key),preAdmissionConflicts:[],reconcilePending:(key:string)=>service.expressionExtractionStatus(fid,key),readLatest:()=>service.expressionSource(fid,source.id),adoptLatest:(draft:any)=>draft,onSaved:reload,snapshot:{text:source.text,created_at:source.created_at}});
 }
 function review(candidate:any,operation:'enable'|'disable'|'revoke'){
  if(retired||!read.fresh)return;
  setAction({identity:identity('review',candidate.id,operation),scope:[fid,'expression-review',candidate.id,operation],storageKey:storage('review',candidate.id,operation),title:operation==='enable'?'审核并启用表达':operation==='disable'?'停用表达':'撤销表达候选',description:operation==='enable'?'填写适用触发词。当前消息包含该词时，角色才可能参考这条表达；她仍可自行决定是否使用。':'停止后旧草稿会重新核验，已经发送的消息和审核记录保留。',initial:{base:candidate.revision,reason:'',triggerText:candidate.trigger_text||''},editable:operation==='enable'?['triggerText','reason']:['reason'],labels:{triggerText:'适用触发词',reason:'审核理由'},validate:()=>requireWritableCharacter(w,fid),build:(draft:any)=>{if(!draft.reason.trim()||(operation==='enable'&&!draft.triggerText.trim()))throw Error('请填写审核理由和适用触发词');return {expectedRevision:draft.base,reason:draft.reason,...(operation==='enable'?{triggerText:draft.triggerText}: {})};},execute:(body:any,key:string)=>service.reviewExpressionLearning(fid,candidate.id,operation,body,key),readLatest:async()=>{const fresh=await service.expressionLearning(fid);const item=fresh.items.find((entry:any)=>entry.id===candidate.id);if(!item)throw Error('候选已不可用');return item;},adoptLatest:(draft:any,fresh:any)=>({...draft,base:fresh.revision}),onSaved:reload,snapshot:{excerpt:candidate.excerpt,situation:candidate.situation,source_text:candidate.source_text,source_valid:candidate.source_valid}});
 }
 return <section className="grid min-w-0 gap-4" aria-labelledby="expression-learning-title">
  <Card><CardHeader><CardTitle id="expression-learning-title">表达习惯</CardTitle></CardHeader><CardContent className="grid min-w-0 gap-3">
   <p className="text-sm text-muted-foreground">从你的真实消息中提取候选，审核后才可按语境参考。学习与使用分别控制，不改变人格或关系事实。</p>
   {read.error&&<Feedback kind="error">{read.error}</Feedback>}
   {settings&&<div className="flex flex-wrap gap-3"><Button type="button" variant="outline" disabled={retired||!read.fresh} onClick={()=>settingsAction('learningEnabled')}>学习：{settings.learning_enabled?'已开启':'已关闭'}</Button><Button type="button" variant="outline" disabled={retired||!read.fresh} onClick={()=>settingsAction('useEnabled')}>参考：{settings.use_enabled?'已开启':'已关闭'}</Button></div>}
   {!read.loading&&!items.length&&<p className="text-sm text-muted-foreground">还没有表达候选。开启学习后，可以从下方真实消息中选择一条。</p>}
   {items.map((candidate:any)=><article key={candidate.id} className="grid min-w-0 gap-2 border-t pt-3"><div className="flex flex-wrap items-baseline gap-2"><strong className="break-words">{candidate.excerpt}</strong><span className="text-sm text-muted-foreground">{labels[candidate.status]||candidate.status}</span></div><p className="text-sm">适用情境：{candidate.situation}</p>{candidate.trigger_text&&<p className="text-sm text-muted-foreground">触发词：{candidate.trigger_text}</p>}{!candidate.source_valid&&<Feedback kind="warning">来源已变化，不能用于新回复。</Feedback>}<details><summary>查看来源与审核记录</summary><p className="whitespace-pre-wrap break-words text-sm">{candidate.source_text||'来源已不可读取'}</p>{candidate.reviews?.map((entry:any)=><p className="text-sm text-muted-foreground" key={entry.revision}>版本 {entry.revision} · {entry.action} · {entry.reason}</p>)}</details><div className="flex flex-wrap gap-2">{candidate.status!=='enabled'&&candidate.status!=='revoked'&&<Button type="button" variant="outline" disabled={retired||!read.fresh||!candidate.source_valid} onClick={()=>review(candidate,'enable')}>审核并启用</Button>}{candidate.status==='enabled'&&<Button type="button" variant="outline" disabled={retired||!read.fresh} onClick={()=>review(candidate,'disable')}>停用</Button>}{candidate.status!=='revoked'&&<Button type="button" variant="outline" disabled={retired||!read.fresh} onClick={()=>review(candidate,'revoke')}>撤销</Button>}</div></article>)}
  </CardContent></Card>
  <details><summary>选择真实消息提取候选</summary><div className="grid min-w-0 gap-3 pt-3">{sources.items.map((source:any)=><article key={source.id} className="grid min-w-0 gap-2 border-b pb-3"><p className="whitespace-pre-wrap break-words text-sm">{source.text}</p><Button className="w-fit" type="button" variant="outline" disabled={retired||!read.fresh||!settings?.learning_enabled} onClick={()=>extract(source)}>从这条提取</Button></article>)}{!sources.loading&&!sources.items.length&&<p className="text-sm text-muted-foreground">没有可用于学习的真实消息。</p>}<CursorControls page={sources} label="来源消息"/></div></details>
  {action&&<CommandDialog key={action.identity} action={action} onClose={()=>setAction(null)}/>}
 </section>;
}
