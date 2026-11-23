import {requireWritableCharacter} from '@/state/domain-guards.mjs';
import {useState} from 'react';
import {Button} from '@/components/ui/button';
import {Card,CardHeader,CardTitle,CardContent} from '@/components/ui/card';
import {Badge} from '@/components/ui/badge';
import {Feedback} from '../shared/Presentation';
import {Label,FieldInput,ChoiceSelect} from '../controls';
import {useWorkspace} from '../page';
import {useCursorPage} from '../hooks/use-cursor-page';
import {useDomainRead} from '../hooks/use-domain-read';
import RecordShell from '../shared/RecordShell';
import CursorControls from '../shared/CursorControls';
import CommandDialog from '../shared/CommandDialog';
import {
 ClaimReadingSummary, ClaimValueSummary, ProfileDiagnostic, ProfileExtensionsSummary, ProfileFieldsSummary,
 ProfileNote, ProfileTime, WorkingPortraitSummary, personalityShape, profileText,
} from '../shared/ProfileRecordSummary';
import {GovernanceNavigation} from './GovernancePage';
import {canRestoreSelfRevision,selfCategoryLabels} from '@/v3/proto/components/self-claims.js';
import {stateLabel} from '@/v3/domain-presentation.mjs';
import ExpressionLearningPanel from './ExpressionLearningPanel';
export default function SelfPage({route,mode}:any){return mode==='catalogue'?<SelfCatalogue route={route}/>:route.feature.params.claimId?<SelfClaim route={route}/>:<SelfSummary route={route}/>;}
function useSelfAction(fid:string,reload:()=>unknown){const w=useWorkspace(),owner=w.state.session.actorId,service=w.services.selfClient,[action,setAction]=useState<any>(null),retired=['已退休','已归档','retired','archived'].includes(w.state.characters.find((row:any)=>row.id===fid)?.status);return {action,setAction,retired,operate(claim:any,operation:string,targetRevision?:number){if(retired)return;setAction({identity:JSON.stringify([owner,fid,claim.id,operation,targetRevision]),scope:[fid,claim.id,operation,targetRevision],storageKey:'fluctlight-studio:'+owner+':self-command:'+JSON.stringify([owner,fid,claim.id,operation,targetRevision]),title:operation==='review'?'复核这条认识':operation==='forget'?'遗忘这条认识':'恢复此版认识内容',description:operation==='review'?'记录你已阅读并认可这个版本，原始证据、置信度与人格各自保持。':operation==='forget'?'从当前消费中移除，证据与修订历史保留。':'追加新的认识修订，历史来源仍需有效。',initial:{base:claim.revision,reason:''},validate:()=>requireWritableCharacter(w,fid),build:(draft:any)=>{if(!draft.reason.trim())throw Error('请填写操作理由');return {expectedRevision:draft.base,reason:draft.reason,...(operation==='restore'?{targetRevision}:{})};},execute:(body:any,key:string)=>service.command(fid,claim.id,operation,body,key),readLatest:()=>service.read(fid,claim.id),adoptLatest:(draft:any,fresh:any)=>({...draft,base:fresh.revision}),onSaved:reload,snapshot:claim});}};}
const categoryLabel = (value: keyof typeof selfCategoryLabels) => selfCategoryLabels[value];
function ClaimCard({claim,onOpen,onAction,retired}:any){
 const category = categoryLabel(claim.category) || (claim.category ? '其他认识' : '');
 return <Card>
  <CardHeader>
   <CardTitle>{profileText(claim.claim) || '成长认识'}</CardTitle>
   {claim.status && <Badge variant="secondary">{stateLabel(claim.status)}</Badge>}
  </CardHeader>
  <CardContent className="grid gap-3">
   {claim.source_valid===false&&<Feedback kind="warning">依赖的事实来源已变化，这条认识不作为当前有效上下文。</Feedback>}
   {claim.expired&&<p className="text-sm text-muted-foreground">已超过有效期限。</p>}
   <ClaimReadingSummary claim={claim} category={category} />
   <ProfileExtensionsSummary value={claim} />
   {claim.owner_review && <p className="text-sm">所有者已复核这条认识。复核不提高证据或置信度。</p>}
   <div className="flex flex-wrap gap-3">
    {onOpen&&<Button type="button" variant="outline" onClick={onOpen}>详情与修订</Button>}
    <Button type="button" variant="outline" disabled={retired} onClick={()=>onAction(claim,'review')}>复核认可</Button>
    <Button type="button" variant="outline" disabled={retired||claim.status==='forgotten'} onClick={()=>onAction(claim,'forget')}>遗忘此认识</Button>
   </div>
   <ProfileDiagnostic value={claim} title="原始认识记录（诊断）" />
  </CardContent>
 </Card>;
}
function SelfCatalogue({route}:any){const w=useWorkspace(),fid=route.fid,owner=w.state.session.actorId,service=w.services.selfClient,[q,setQ]=useState(route.query.get('q')||''),[status,setStatus]=useState(route.query.get('status')||'all'),[category,setCategory]=useState(route.query.get('category')||''),[applied,setApplied]=useState({q,status,category});const page=useCursorPage(cursor=>service.list(fid,{...applied,cursor}),JSON.stringify([owner,fid,applied]));const actions=useSelfAction(fid,page.reload);return <RecordShell eyebrow="身份与认识" description="查阅逐渐形成的认识，保留每条认识的证据与修订。" read={{loading:page.loading,reload:page.reload}} title="发展中的自我" back={'/c/'+encodeURIComponent(fid)+'/growth/self'} backLabel="查看成长认识"><GovernanceNavigation fid={fid} current="developing"/><Card><CardContent className="grid gap-3 p-5"><p className="text-sm text-muted-foreground">认识来自真实证据与反思。复核、遗忘和恢复会保留原始来源与修订。</p><form className="flex flex-wrap gap-3" role="search" onSubmit={event=>{event.preventDefault();setApplied({q:q.trim(),status,category});if(q.trim()===applied.q&&status===applied.status&&category===applied.category)void page.reload();}}><Label>搜索认识<FieldInput value={q} onChange={(event:any)=>setQ(event.target.value)}/></Label><Label>状态<ChoiceSelect value={status} onChange={(event:any)=>setStatus(event.target.value)}>{['all','active','uncertain','forgotten','superseded'].map(value=><option key={value} value={value}>{value==='all'?'全部（含遗忘）':stateLabel(value)}</option>)}</ChoiceSelect></Label><Label>类别<ChoiceSelect value={category} onChange={(event:any)=>setCategory(event.target.value)}><option value="">全部类别</option>{Object.entries(selfCategoryLabels).map(([value,label])=><option key={value} value={value}>{label}</option>)}</ChoiceSelect></Label><Button type="submit" variant="outline">查询</Button></form></CardContent></Card>{page.items.map((claim:any)=><ClaimCard key={claim.id} claim={claim} retired={actions.retired} onAction={actions.operate} onOpen={()=>w.navigatePath('/c/'+encodeURIComponent(fid)+'/growth/self/claim/'+encodeURIComponent(claim.id))}/>)}{!page.items.length&&!page.loading&&<p>当前筛选没有认识记录。</p>}<CursorControls page={page} label="认识"/>{actions.action&&<CommandDialog key={actions.action.identity} action={actions.action} onClose={()=>actions.setAction(null)}/>}</RecordShell>;}
const revisionLabels: Record<string,string> = {
 create:'形成认识', update:'修订认识', forget:'遗忘认识', restore:'恢复认识',
};

function ClaimSnapshot({value,title}:any) {
 if (!value || typeof value !== 'object' || Array.isArray(value) || !('category' in value || 'claim' in value)) {
  return <ClaimValueSummary value={value} title={title} />;
 }
 const claim = profileText(value.claim);
 const category = categoryLabel(value.category);
 if (!claim && !category && value.value == null && !Number.isFinite(value.confidence)) return null;
 return <div className="grid gap-2">
  <h4 className="font-medium">{title}</h4>
  <ProfileNote value={claim} />
  <ClaimReadingSummary claim={value} category={category} />
 </div>;
}

function SelfClaim({route}:any) {
 const w=useWorkspace(),fid=route.fid,id=route.feature.params.claimId,owner=w.state.session.actorId,service=w.services.selfClient;
 const read=useDomainRead(()=>service.read(fid,id),JSON.stringify([owner,fid,id]));
 const revisions=useCursorPage(cursor=>service.history(fid,id,'revisions',cursor),JSON.stringify([owner,fid,id,'revisions']));
 const reviews=useCursorPage(cursor=>service.history(fid,id,'reviews',cursor),JSON.stringify([owner,fid,id,'reviews']));
 const reload=async()=>{await read.reload();await Promise.all([revisions.reload(),reviews.reload()]);};
 const actions=useSelfAction(fid,reload);
 return <RecordShell eyebrow="认识与修订" description="核对这条认识的来源、变化与复核记录。" title="认识详情与修订" back={'/c/'+encodeURIComponent(fid)+'/governance?tab=developing'} backLabel="全部认识" read={read}>
  {read.data && <ClaimCard claim={read.data} retired={actions.retired||!read.fresh} onAction={actions.operate} />}
  <Card>
   <CardHeader><CardTitle>认识修订与恢复</CardTitle></CardHeader>
   <CardContent className="grid gap-4">
    {revisions.items.map((entry:any,index:number)=><section key={entry.id||index} className="grid gap-3 border-b pb-4">
     <b>{revisionLabels[entry.change_type] || '认识修订'}</b>
     {entry.status && <Badge variant="secondary">{stateLabel(entry.status)}</Badge>}
     <ProfileTime value={entry.created_at} />
     <ProfileNote value={entry.reason || entry.candidate?.reason} label="修订理由" />
     <ClaimSnapshot value={entry.before_value} title="修改前的认识" />
     <ClaimSnapshot value={entry.after_value} title="修改后的认识" />
     <ProfileExtensionsSummary value={entry} />
     <ProfileDiagnostic value={entry} title="原始认识修订记录（诊断）" />
     {canRestoreSelfRevision(entry)&&read.data
      ? <Button type="button" variant="outline" disabled={actions.retired||!read.fresh} onClick={()=>actions.operate(read.data,'restore',entry.revision)}>恢复此版本内容</Button>
      : <p className="text-sm text-muted-foreground">此记录没有可恢复的完整有效快照。</p>}
    </section>)}
    <CursorControls page={revisions} label="认识修订" />
   </CardContent>
  </Card>
  <Card>
   <CardHeader><CardTitle>所有者复核记录</CardTitle></CardHeader>
   <CardContent className="grid gap-3">
    {reviews.items.map((entry:any,index:number)=><div key={entry.id||index} className="grid gap-2">
     <ProfileNote value={entry.reason} label="复核意见" />
     <ProfileTime value={entry.created_at} />
     <ProfileExtensionsSummary value={entry} />
     <ProfileDiagnostic value={entry} title="原始复核记录（诊断）" />
    </div>)}
    {!reviews.items.length&&!reviews.loading&&<p>尚无人工复核。正常反思不以人工审核为前提。</p>}
    <CursorControls page={reviews} label="复核" />
   </CardContent>
  </Card>
  {actions.action&&<CommandDialog key={actions.action.identity} action={actions.action} onClose={()=>actions.setAction(null)}/>}
 </RecordShell>;
}
function SelfSummary({route}:any){const w=useWorkspace(),fid=route.fid,owner=w.state.session.actorId,service=w.services.selfClient,read=useDomainRead(()=>service.summary(fid),JSON.stringify([owner,fid,'self-summary']));const claims=useCursorPage(cursor=>service.list(fid,{status:'active',cursor}),JSON.stringify([owner,fid,'active-claims'])),reflections=useCursorPage(cursor=>service.reflections(fid,cursor),JSON.stringify([owner,fid,'recent-reflections']));const reload=async()=>{await read.reload();await Promise.all([claims.reload(),reflections.reload()]);},actions=useSelfAction(fid,reload),[proposalAction,setProposalAction]=useState<any>(null),snapshot=route.query.get('view')==='snapshot',detail=read.data,proposals=Array.isArray(detail?.foundation_revisions)?detail.foundation_revisions.filter((entry:any)=>entry.status==='proposed'):null;
 function decide(proposal:any,accept:boolean){if(actions.retired||!detail)return;setProposalAction({identity:JSON.stringify([owner,fid,'foundation',proposal.id,accept]),scope:[fid,'foundation',proposal.id,accept?'accept':'reject'],storageKey:'fluctlight-studio:'+owner+':governance:'+JSON.stringify([owner,fid,'foundation',proposal.id,accept]),title:accept?'采纳身份提议':'暂不采纳提议',casOnly:true,description:'通过实际身份核心命令处理，生活状态和历史各自保留。',initial:{base:detail.current_revision,reason:''},validate:()=>requireWritableCharacter(w,fid),build:(draft:any)=>{if(!draft.reason.trim())throw Error('请填写操作理由');return {expectedRevision:draft.base,reason:draft.reason};},execute:(body:any)=>service.decideFoundation(fid,proposal.id,accept,body.expectedRevision,body.reason),readLatest:async()=>{const value=await service.summary(fid);return {...value,revision:value.current_revision};},adoptLatest:(draft:any,fresh:any)=>({...draft,base:fresh.revision}),onSaved:reload,snapshot:proposal});}
 const personality=detail?.personality??detail?.core_persona?.personality;
 const compilation=detail?.persona_compilation;
 return <RecordShell eyebrow="记忆与成长" description={snapshot?'查看稳定人格、工作画像与待确认的身份提议。':'回顾真实交流、经历与反思中逐渐形成的认识。'} title={snapshot?'人格快照':'成长认识'} back={'/c/'+encodeURIComponent(fid)+'/growth'} backLabel="查看记忆" read={{...read,reload}}>
  <div className="flex flex-wrap gap-3">
   <Button type="button" variant="outline" onClick={()=>w.navigatePath('/c/'+encodeURIComponent(fid)+'/governance?tab=developing')}>查阅全部认识与历史</Button>
   <Button type="button" variant="outline" onClick={()=>w.navigatePath('/c/'+encodeURIComponent(fid)+'/growth/self'+(snapshot?'':'?view=snapshot'))}>{snapshot?'成长认识':'人格快照'}</Button>
  </div>
  {!snapshot&&<>
   {claims.items.map((claim:any)=><ClaimCard key={claim.id} claim={claim} retired={actions.retired} onAction={actions.operate} onOpen={()=>w.navigatePath('/c/'+encodeURIComponent(fid)+'/growth/self/claim/'+encodeURIComponent(claim.id))}/>)}
   {!claims.items.length&&!claims.loading&&<p>还没有当前有效的认识记录。</p>}
   <CursorControls page={claims} label="当前认识" />
   <ExpressionLearningPanel fid={fid} />
  </>}
  <Card>
   <CardHeader><CardTitle>稳定人格与工作画像</CardTitle></CardHeader>
   <CardContent className="grid gap-4">
    <ProfileFieldsSummary value={personality} shape={personalityShape} title="稳定性格" />
    <WorkingPortraitSummary value={compilation} activeProfileId={detail?.personality_runtime?.active_profile_id} />
    <ProfileExtensionsSummary value={[personality, compilation]} />
    <ProfileDiagnostic value={personality != null || compilation != null ? {personality, persona_compilation:compilation} : null} title="原始人格快照（诊断）" />
    <h3 className="font-medium">最近的反思</h3>
    {reflections.items.map((entry:any)=><Button key={entry.id} type="button" variant="ghost" className="h-auto justify-start whitespace-normal text-left" onClick={()=>w.navigatePath('/c/'+encodeURIComponent(fid)+'/growth/reflection/'+encodeURIComponent(entry.id))}>
     {profileText(entry.summary) || profileText(entry.title) || '反思记录'}{entry.status && ' · '+stateLabel(entry.status)}
    </Button>)}
    <CursorControls page={reflections} label="最近反思" />
   </CardContent>
  </Card>
  <Card>
   <CardHeader><CardTitle>人格与身份提议</CardTitle></CardHeader>
   <CardContent className="grid gap-3">
    {!detail?null:proposals===null?<p>当前服务未提供身份修订记录，请重新读取后核对。</p>:proposals.length?proposals.map((proposal:any)=><div key={proposal.id} className="grid gap-2 rounded-lg border p-3">
     <b>身份核心修订提议</b>
     <ProfileNote value={proposal.reason} label="提议理由" />
     <ProfileTime value={proposal.created_at} />
     <ProfileFieldsSummary value={proposal.changes} title="提议变更" showClears />
     <ProfileExtensionsSummary value={proposal} />
     <div className="flex flex-wrap gap-3">
      <Button type="button" variant="outline" disabled={actions.retired||!read.fresh} onClick={()=>decide(proposal,false)}>暂不采纳</Button>
      <Button type="button" disabled={actions.retired||!read.fresh} onClick={()=>decide(proposal,true)}>采纳</Button>
     </div>
     <ProfileDiagnostic value={proposal} title="原始身份提议记录（诊断）" />
    </div>):<p>暂无待确认提议。</p>}
    <Button type="button" variant="link" onClick={()=>w.navigatePath('/c/'+encodeURIComponent(fid)+'/governance?tab=foundation')}>身份核心与完整修订</Button>
   </CardContent>
  </Card>
  {actions.action&&<CommandDialog key={actions.action.identity} action={actions.action} onClose={()=>actions.setAction(null)}/>}
  {proposalAction&&<CommandDialog key={proposalAction.identity} action={proposalAction} onClose={()=>setProposalAction(null)}/>}
 </RecordShell>;
}
