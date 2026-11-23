import {errorMessage} from '../v3/domain-presentation.mjs';
import {diaryAttemptStatus} from '../approved-ui/diary-generation.mjs';
import {openPromise} from '../v3/proto/components/promise-catalogue.js';
import {sourceHref,sourceReturn} from '../v3/proto/components/source-navigation.js';
import {mountCalendarReading,localDate,reconcileReadingRecord} from '../v3/proto/components/calendar-reading.js';
import {diaryClient} from '../v3/proto/services.js';
import * as store from '../v3/proto/store.js';
import {asyncPage,instant} from '../v3/proto/components/async-page.js';
import {CursorPager,bindCursorPager} from '../v3/proto/components/cursor-pager.js';
import {Field,Tabs,Toggle} from '../v3/proto/components/forms.js';
import {RawRecordDownload} from '../v3/proto/components/records.js';
import {beginDraftSubmission,settleDraftSubmission} from '../v3/proto/components/draft-submission.js';
import {esc,badge,confirm,drawer,toast} from '../v3/proto/ui.js';
import {isInternalId} from '../v3/internal-id.mjs';
import {userInstant} from '../v3/presentation-time.mjs';

const labels={pending:'正在等待日记生成',running:'正在生成',ready:'已生成',no_content:'本次没有日记',failed:'生成失败',inactive:'角色未处于运行状态'};
const entryHref=(fid,id)=>`#/c/${encodeURIComponent(fid)}/life/diary/${encodeURIComponent(id)}`;
const drafts=new Map(),generationDrafts=new Map();store.subscribe(()=>{if(!store.get('session')){drafts.clear();generationDrafts.clear();}});

export function diaryGenerationRecord(fid,entry){
 const pending=['pending','running'].includes(entry.status);
 const message=pending?'<p data-domain-ui="skeleton">正在整理这一天的经历，可以稍后回来查看。</p>':entry.status==='failed'?'<p data-domain-ui="alert" class="banner err" role="alert">这次日记生成失败，可查看原因并从原记录恢复。</p>':'<p>这一天还没有形成日记正文。</p>';
 return `<article data-domain-ui="card" class="card diary-generation-state"><b>${esc(entry.date)} · ${esc(labels[entry.status]||'没有正文')}</b>${message}<a data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" href="${esc(entryHref(fid,entry.id))}">${entry.status==='failed'?'查看原因与恢复':'查看生成进度'}</a></article>`;
}

export function renderRealDiaryList(el,ctx){
 const fid=ctx.charId,favorite=ctx.query.favorite==='true',zone=store.get('characters',fid)?.timezone;
 const key=JSON.stringify([store.get('session')?.actorId,fid]);
 if(!generationDrafts.has(key))generationDrafts.set(key,{localDate:'',timezone:zone||'Asia/Shanghai',pending:false,error:'',command:null,result:null});
 const draft=generationDrafts.get(key);
 let active=true,reader,generationDrawer;
 el.innerHTML=`<div class="stack reading-workspace"><div class="row reading-page-heading"><h1 class="reading-business-title grow">她的日记</h1><button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" data-open-generation>补写日记</button></div><div class="row diary-reading-toolbar">${Tabs({attrs:{role:'group','aria-label':'日记筛选'},value:favorite?'favorite':'all',items:[{value:'all',label:'全部日记',href:`#/c/${fid}/life/diary`},{value:'favorite',label:'收藏',href:`#/c/${fid}/life/diary?favorite=true`}]})}</div><div data-diary-reading></div></div>`;
 reader=mountCalendarReading(el.querySelector('[data-diary-reading]'),{scope:[fid,'diary',favorite],zone,initialDate:ctx.query.date||'',initialMonth:ctx.query.month||'',initialMode:ctx.query.mode||'list',visible:d=>typeof d.body==='string'&&!!d.body.trim(),emptyText:'还没有读到她写下的日记。',label:'日记',renderHidden:d=>diaryGenerationRecord(fid,d),onView:view=>{ctx.raw=ctx.path+'?'+new URLSearchParams({...view,...(favorite?{favorite:'true'}:{})});history.replaceState(null,'','#'+ctx.raw);},read:cursor=>diaryClient.list(fid,cursor,favorite),dateOf:d=>d.date,
  toolbar:'',
  render:d=>`<article class="reading-paper life-reading-paper"><div class="row"><h3 class="grow">${esc(d.date)}</h3>${d.annotation.favorite?badge('已收藏'):''}${badge(labels[d.status]||d.status,'plain')}</div><p class="paper-body">${esc(d.body||(d.date===localDate(new Date().toISOString(),zone||'Asia/Shanghai')?'今晚她会写下今天的日记，有合适素材时才会形成正文。':['pending','running'].includes(d.status)?'正在整理这一天的经历，打开可查看生成进度。':d.status==='failed'?'这篇日记没有生成，打开可查看原因并重试。':'还没有正文，打开查看这次回顾的结果。'))}</p><a data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" href="${esc(sourceHref(entryHref(fid,d.id),ctx,'返回日记'))}">阅读这篇日记</a></article>`});
 function openGeneration(){
  generationDrawer=drawer({title:'生成日记',body:`<form class="stack" data-generate>${Field({label:'日期',name:'localDate',type:'date',value:draft.localDate,attrs:{required:true}})}${Field({label:'时区',name:'timezone',value:draft.timezone,attrs:{required:true}})}<p class="small muted">选择已结束的本地日期。已有日记会保留；提交后查看这次处理的状态。</p><p data-disabled-reason role="status"></p><p data-feedback role="alert">${esc(draft.error)}</p><button data-domain-ui="button" data-domain-variant="default" data-domain-size="default" type="submit">生成这一天的日记</button>${draft.result?`<a data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" href="${esc(entryHref(fid,draft.result.diary_id))}" data-result>查看原处理结果</a>`:''}</form>`,onClose:()=>{generationDrawer=null;delete draft.update;}});
  const dialog=generationDrawer,form=dialog.el.querySelector('[data-generate]');
  const check=()=>{let today='';const timezone=form.timezone.value.trim();try{if(timezone&&Intl.DateTimeFormat('en',{timeZone:timezone}))today=localDate(new Date().toISOString(),timezone);}catch{}const button=form.querySelector('button');button.disabled=draft.pending||store.get('characters',fid)?.status==='已退休'||(!draft.command&&(!today||!form.localDate.value||form.localDate.value>=today));button.textContent=draft.pending?'正在提交…':draft.command?'重试原生成请求':'生成这一天的日记';form.localDate.disabled=form.timezone.disabled=!!draft.command||draft.pending;form.querySelector('[data-feedback]').textContent=draft.error;form.querySelector('[data-disabled-reason]').textContent=draft.pending?'原生成请求正在提交，请等待回执。':store.get('characters',fid)?.status==='已退休'?'当前角色已退休，不能生成日记。':!today?'请填写有效的 IANA 时区，例如 Asia/Shanghai。':!form.localDate.value?'请选择已结束的日期。':form.localDate.value>=today?'只能回顾已结束的日期；今天的日记需要等这一天结束。':'';};
  draft.update=check;
  form.oninput=()=>{draft.localDate=form.localDate.value;draft.timezone=form.timezone.value;check();};check();
  form.querySelector('[data-result]')?.addEventListener('click',()=>dialog.close());
  form.onsubmit=async event=>{event.preventDefault();check();if(form.querySelector('button').disabled||!form.reportValidity())return;
   draft.command ||= {localDate:draft.localDate,timezone:draft.timezone.trim(),reason:'Owner请求回顾已结束日期'};
   draft.pending=true;draft.error='';check();
   try{const result=await diaryClient.generate(fid,draft.command);if(generationDrafts.get(key)!==draft)return;draft.result=result;draft.command=null;draft.error='';if(active){dialog.close();ctx.navigate(entryHref(fid,result.diary_id));}}
   catch(error){if(generationDrafts.get(key)===draft){draft.error=errorMessage(error);if(error.status>=400&&error.status<500)draft.command=null;if(dialog.el.isConnected)form.querySelector('[data-feedback]').textContent=errorMessage(error);}}
   finally{draft.pending=false;draft.update?.();}
  };
 }
 el.querySelector('[data-open-generation]').onclick=openGeneration;
 return()=>{active=false;generationDrawer?.close();reader();};
}

function diarySources(fid,entry){
 const refs=new Set(entry.refs||[]),sources=entry.source_snapshot?.sources||[];
 const communication=sources.filter(source=>source.kind==='remote_communication').length;
 const rows=sources.filter(source=>refs.has(source.ref)).map(source=>{
  const remote=source.kind==='remote_communication';
  const event=source.kind==='confirmed_life_event'||source.ref.startsWith('event:');
  if(!remote&&!event)return '';
  const id=source.ref.replace(/^[^:]+:/,'');
  const link=remote
   ?(id&&source.conversation_id?`#/c/${encodeURIComponent(fid)}/chat?${new URLSearchParams({conversation:source.conversation_id,message:id})}`:'')
   :(id?`#/c/${encodeURIComponent(fid)}/world/event/${encodeURIComponent(id)}`:'');
  const label=remote?'原交流消息':'已确认生活事件';
  const excerpt=remote?source.text:[source.activity,source.scene,source.location].filter(value=>typeof value==='string'&&value.trim()).join(' · ');
  const time=remote?source.at:source.start_at;
  return `<div data-domain-ui="item" class="lrow"><div class="grow">
   ${link?`<a href="${esc(link)}">${label}</a>`:`<b>${label}</b>`}
   ${time?`<p class="small muted"><time datetime="${esc(time)}">${esc(instant(time))}</time></p>`:''}
   ${typeof excerpt==='string'&&excerpt.trim()?`<p class="memory-content">${esc(excerpt)}</p>`:''}
  </div></div>`;
 }).join('');
 return `${sources.length?`<p class="small muted">可用素材 ${sources.length} 条${communication?`（远程交流 ${communication} 条）`:''}</p>`:''}
  ${refs.size?`<p class="small muted">正文引用来源 ${refs.size} 条</p>`:''}
  ${rows}
  ${RawRecordDownload(entry,'原始日记记录（诊断）')}`;
}

function diaryAttemptRow(attempt,index){
 const time=attempt.created_at;
 const reason=typeof attempt.plan?.reason==='string'?attempt.plan.reason.trim():'';
 const explanation=attempt.error_code
  ?errorMessage({code:attempt.error_code},'这次整理遇到问题，请查看原任务诊断。')
  :reason==='no_source_material'?'没有可用于整理的素材。':/^[a-z\d]+(?:[_:][a-z\d]+)+$/i.test(reason)?'':reason;
 return `<div data-domain-ui="item" class="lrow"><div class="grow">
  <p>第 ${esc(attempt.attempt??index+1)} 次整理 · ${diaryAttemptStatus(attempt.status)}${time?` · <time datetime="${esc(time)}">${esc(instant(time))}</time>`:''}</p>
  ${explanation?`<p class="small muted">${esc(explanation)}</p>`:''}
 </div>${attempt.task_id?`<a data-domain-ui="button" data-domain-variant="outline" data-domain-size="sm" href="#/diag/task/${encodeURIComponent(attempt.task_id)}">查看原任务</a>`:''}</div>`;
}

function diaryAnnotationConflict(annotation){
 return `<div data-domain-ui="alert" class="banner err" role="alert" data-test="conflict">已保存的注记或收藏状态已更新。你的草稿已保留。</div>
  <div class="stack">
   <b>最新已保存注记</b>
   <p>${annotation.favorite?'已收藏':'未收藏'}</p>
   ${annotation.note?`<p class="memory-content">${esc(annotation.note)}</p>`:''}
   ${RawRecordDownload(annotation,'原始注记记录（诊断）')}
  </div>
  <div class="row [margin:8px_0]">
   <button data-domain-ui="button" type="button" data-domain-variant="outline" data-domain-size="default" data-keep>保留草稿</button>
   <button data-domain-ui="button" type="button" data-domain-variant="default" data-domain-size="default" data-refresh>采用最新注记并保留草稿</button>
  </div>`;
}
export function renderRealDiaryEntry(el,ctx){
 const fid=ctx.charId,id=ctx.params.entryId,owner=store.get('session')?.actorId,key=JSON.stringify([owner,fid,id]);
 let cached=null;
 return asyncPage(el,async()=>{const value=cached||await diaryClient.read(fid,id);cached=null;return value;},(entry,reload,connected)=>{
  const alive=()=>connected()&&store.get('session')?.actorId===owner;
  let timer=null,observing=false,reads=0;
  async function observe(){clearTimeout(timer);if(!alive()||observing||document.hidden||!['pending','running'].includes(entry.status)||reads>=150)return;observing=true;reads++;try{const next=await diaryClient.read(fid,id);if(!alive())return;if(next.status!==entry.status||next.body!==entry.body||next.revision!==entry.revision){cached=next;void reload();return;}el.querySelector('[data-generation-feedback]').textContent='正在跟进生成进度，可以稍后再回来。';}catch(error){if(alive())el.querySelector('[data-generation-feedback]').textContent='更新失败，上次内容已保留：'+errorMessage(error);}finally{observing=false;if(alive()&&!document.hidden&&reads<150)timer=setTimeout(observe,3000);else if(alive()&&reads>=150)el.querySelector('[data-generation-feedback]').textContent='自动更新已暂停，可以点击重新读取继续查看。';}}
  const visible=()=>{clearTimeout(timer);if(!document.hidden)void observe();};
  reconcileReadingRecord(fid,'diary',entry,(scope,row)=>!scope[2]||row.annotation.favorite);
  if(!drafts.has(key))drafts.set(key,{note:entry.annotation.note,favorite:entry.annotation.favorite,base:entry.annotation.revision,reply:'',dirty:false,pending:{}});
  const draft=drafts.get(key);
  if(!draft.dirty&&!draft.pending.note&&entry.annotation.revision>=draft.base)Object.assign(draft,{note:entry.annotation.note,favorite:entry.annotation.favorite,base:entry.annotation.revision});
  const conflict=draft.base<entry.annotation.revision;
  el.innerHTML=`<div class="stack reading-workspace">
   <div class="row reading-page-heading"><h1 class="reading-business-title grow">${esc(entry.date)} · 日记</h1><button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" data-open-note>收藏与注记${draft.dirty?' · 有草稿':''}</button></div>
   <div class="reading-paper life-reading-paper"><div class="row diary-entry-head">${badge('角色主观记录','plain')}${badge(labels[entry.status]||entry.status)}<button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" data-reload>重新读取</button></div><p class="small muted">按日记原日期归档</p>${entry.body?`<p class="memory-content">${esc(entry.body)}</p>`:`<p>${entry.status==='no_content'?'这次没有足够或适合的素材形成日记。可以继续交流、生活，之后回顾相应日期。':'尚无已提交正文。请查看实际任务和状态，受理不代表已经生成。'}</p>`}${entry.last_error_code?`<p data-domain-ui="alert" class="banner err" role="alert">${esc(errorMessage(entry.last_error_code,'这篇日记没有生成，请查看任务原因后重试。'))}</p>`:''}${['failed','inactive'].includes(entry.status)?'<button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" data-retry>按当前状态重新生成</button>':''}<p data-generation-feedback role="alert"></p></div>
   <section data-domain-ui="card" class="card reading-annotation"><div class="row"><h3 class="grow">你的收藏与注记</h3><span data-saved-favorite>${entry.annotation.favorite?badge('已收藏','plain'):''}</span></div><p class="memory-content" data-saved-note ${entry.annotation.note?'':'hidden'}>${esc(entry.annotation.note)}</p><p class="small muted" data-note-receipt>${draft.pending.note?'原注记正在保存，期间的新草稿将保留。':draft.dirty?'你有尚未保存的注记草稿。':'注记属于你，不修改她的主观正文。'}</p></section>
   <details data-domain-ui="disclosure" class="card reading-secondary"><summary>素材与来源</summary>${diarySources(fid,entry)}</details>
   <details data-domain-ui="disclosure" class="card reading-secondary"><summary>生成尝试与实际任务</summary>${(entry.attempts||[]).map(diaryAttemptRow).join('')}${entry.has_more_attempts?'<p class="small muted">显示最近20次尝试；更早执行记录仍保留在任务诊断中。</p>':''}</details>
   <div data-note-host hidden><form class="stack" data-note><h3>你的收藏与注记</h3><p class="small muted">这些字段属于你，不修改她的主观正文。</p>${conflict?diaryAnnotationConflict(entry.annotation):''}${Toggle({label:'收藏这篇日记',name:'favorite',checked:draft.favorite})}${Field({label:'你的注记',name:'note',type:'textarea',value:draft.note,attrs:{maxlength:5000}})}<div class="row"><button data-domain-ui="button" type="button" data-domain-variant="outline" data-domain-size="default" data-reset ${draft.pending.note?'disabled':''}>撤销草稿</button><button data-domain-ui="button" data-domain-variant="default" data-domain-size="default" type="submit" ${conflict||draft.pending.note?'disabled':''}>保存注记</button></div><p data-domain-ui="alert" role="alert" data-note-error></p></form></div>
   <a data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" href="${esc(sourceReturn(ctx,`#/c/${fid}/life/diary`))}">${esc(ctx.query.returnLabel||'返回日记')}</a></div>`;
  el.querySelector('[data-reload]').onclick=reload;
  el.querySelector('[data-retry]')?.addEventListener('click',async event=>{event.target.disabled=true;try{await diaryClient.generate(fid,{localDate:entry.date,timezone:entry.timezone,expectedRevision:entry.revision,reason:'Owner在失败或未运行后明确重试日记'});if(alive())await reload();}catch(error){if(alive()){el.querySelector('[data-generation-feedback]').textContent=errorMessage(error);event.target.disabled=false;}}});
  const note=el.querySelector('[data-note]'),host=el.querySelector('[data-note-host]');let noteDrawer;
  el.querySelector('[data-open-note]').onclick=()=>{noteDrawer=drawer({title:'你的收藏与注记',onClose:()=>{host.append(note);noteDrawer=null;}});noteDrawer.body.append(note);};note.oninput=note.onchange=()=>{draft.note=note.note.value;draft.favorite=note.favorite.checked;draft.dirty=true;el.querySelector('[data-open-note]').textContent='收藏与注记 · 有草稿';if(!draft.pending.note)el.querySelector('[data-note-receipt]').textContent='你有尚未保存的注记草稿。';};
  el.querySelector('[data-reset]').onclick=()=>{if(draft.pending.note)return;draft.dirty=false;void reload();};
  el.querySelector('[data-keep]')?.addEventListener('click',()=>{note.querySelector('[data-note-error]').textContent='注记草稿已保留。';});
  el.querySelector('[data-refresh]')?.addEventListener('click',async()=>{if(await confirm({title:'采用最新注记并保留草稿',text:'请先核对最新已保存的注记和收藏状态。确认后仍可编辑并保存你的草稿。',identity:'所有者修改'})&&alive()){draft.base=entry.annotation.revision;void reload();}});
  const settled=(channel,error,saved)=>{
   if(!alive())return;
   if(channel==='note'){
    note.querySelector('button[type=submit]').disabled=draft.base<entry.annotation.revision;
    note.querySelector('[data-reset]').disabled=false;
    if(saved){const savedNote=el.querySelector('[data-saved-note]');savedNote.textContent=saved.note;savedNote.hidden=!saved.note;el.querySelector('[data-saved-favorite]').innerHTML=saved.favorite?badge('已收藏','plain'):'';}
    el.querySelector('[data-note-receipt]').textContent=(error?errorMessage(error):'')||(draft.dirty?'本次提交已保存，期间的新草稿保留。':'注记已保存');
    note.querySelector('[data-note-error]').textContent=(error?errorMessage(error):'')||(draft.dirty?'本次提交已保存，期间的新草稿保留。':'注记已保存');
    if(!error)toast(draft.dirty?'注记已保存，新草稿已保留':'注记已保存');if(error?.status===409||(!error&&!draft.dirty))void reload();

   }
  };
  draft.onSettled=settled;
  note.onsubmit=async event=>{
   event.preventDefault();if(draft.pending.note||draft.base<entry.annotation.revision)return;
   const submission=beginDraftSubmission(draft,'note',{favorite:draft.favorite,note:draft.note});
   note.querySelector('button[type=submit]').disabled=true;note.querySelector('[data-reset]').disabled=true;
   try{
    const saved=await diaryClient.annotate(fid,id,{...submission.value,expectedRevision:draft.base});
    if(drafts.get(key)!==draft)return;
    const result=settleDraftSubmission(draft,submission,{favorite:draft.favorite,note:draft.note});
    if(result){draft.base=Math.max(draft.base,saved.revision);draft.dirty=!result.unchanged;draft.onSettled?.('note',null,{...submission.value,...saved});}
   }catch(error){if(drafts.get(key)!==draft)return;settleDraftSubmission(draft,submission,null);draft.dirty=true;draft.onSettled?.('note',error);}
  };
  document.addEventListener('visibilitychange',visible);if(['pending','running'].includes(entry.status))timer=setTimeout(observe,3000);
  return()=>{clearTimeout(timer);document.removeEventListener('visibilitychange',visible);noteDrawer?.close();if(draft.onSettled===settled)delete draft.onSettled;};
 },{backHref:`#/c/${encodeURIComponent(ctx.charId)}/life/diary`,backLabel:'返回日记'});
}
const storyTime=value=>typeof value==='string'&&Number.isFinite(Date.parse(value))?userInstant(value):'';
const storyText=value=>typeof value==='string'&&value.trim()&&!isInternalId(value)?value:'';
const footprintLabels={memory:'她记下的事',diary:'她的主观日记',remote_communication:'远程交流',relationship:'关系记录',promise:'约定'};
const storyPromiseStatuses={awaiting_goal:'等待规划，尚未接受',deferred:'暂缓决定',accepted:'已接受，履行待证据'};
const storyCards=[['companionship_days','陪伴天数','天'],['completed_turns','完成回复轮次','轮'],['relationship','现在的我们',''],['memories','关于你的记忆','条'],['diaries','她的日记','篇']];

function storyBasis(snapshot,rows,recordsHref,characterName){
 const from=storyTime(snapshot.first_communication_at),to=storyTime(snapshot.as_of);
 const turns=snapshot.metrics?.completed_turns;
 const semantics=snapshot.semantics;
 const relationship= snapshot.metrics?.relationship?.value;
 const relationshipRole=storyText(typeof relationship?.role === "string" ? relationship.role : relationship?.role?.label);
 return `
  ${from&&to?`<p>交流统计范围：${esc(from)} 至 ${esc(to)}</p>`:to?`<p>统计截至：${esc(to)}</p>`:''}
  ${turns?.available&&turns.complete&&Number.isFinite(turns.value)?`<p>完成回复轮次：${turns.value} 轮</p>`:''}
  ${Array.isArray(snapshot.footprints)?`<p>本页回顾记录 ${rows.length} 条，其中远程交流 ${rows.filter(row=>row.kind==='remote_communication').length} 条。</p>`:''}
  ${relationshipRole?`<p>关系称谓：${esc(relationshipRole)}</p>`:''}
  ${characterName?`<p>回顾范围：你和${esc(characterName)}。</p>`:''}
  <p>统计来自同一次读取。${snapshot.footprints_complete===false?'本页只列出部分来源，完整交流可从下方入口查看。':''}</p>
  ${semantics?.communication==='actual_participation_intervals'?'<p>交流只纳入双方实际参与时段内的消息。</p>':''}
  ${semantics?.turns==='distinct_conversation_and_turn_with_persisted_owner_input_and_nonempty_character_reply'?'<p>每轮需有已保存的留言和角色的非空回复，同一交流中的同一轮只统计一次。</p>':''}
  ${semantics?.memories==='active_owner_actor_refs_and_current_source_liveness'?'<p>记忆只统计与您有关且来源仍有效的记录。</p>':''}
  ${semantics?.diaries==='ready_day_with_nonempty_active_attempt_body'?'<p>日记只统计已生成有效正文的日期。</p>':''}
  ${semantics?.copresent===false?'<p>远程交流不表示双方曾在同一地点。</p>':''}
  <p><a href="${esc(recordsHref)}">查看全部交流</a></p>
  ${RawRecordDownload(snapshot,'原始共同经历记录（诊断）')}`;
}

export function renderRealShared(el,ctx){
 const fid=ctx.charId;
 if(ctx.query.view==='records'){
  const cursors=[''];let at=0;
  return asyncPage(el,()=>diaryClient.shared(fid,cursors[at]),(page,reload)=>{
   el.innerHTML=`<div class="stack story-page"><h1 class="reading-business-title">交流记录</h1><p class="small muted">保留双方实际参与时段内的原交流；远程交流不表示物理同场。</p><div class="reading-paper life-reading-paper">${page.items.map(row=>{
    const time=storyTime(row.occurred_at),title=storyText(row.title);
    return `<div class="story-footprint">${time?`<p class="small muted">${esc(time)}</p>`:''}${title?`<p>${esc(title)}</p>`:''}<a data-domain-ui="button" data-domain-variant="ghost" data-domain-size="default" href="${esc(sourceHref(row.href,ctx,'返回共同交流'))}">查看原交流</a></div>`;
   }).join('')||'<p>当前读取范围没有交流记录。</p>'}</div>${CursorPager({more:!!page.next_cursor,previous:at>0,label:'交流'})}<button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" data-story-back>返回共同经历</button></div>`;
   el.querySelector('[data-story-back]').onclick=ctx.goBack;
   bindCursorPager(el,{next:()=>{cursors[++at]=page.next_cursor;void reload();},previous:()=>{at--;void reload();}});
  });
 }
 const owner=store.get('session')?.actorId;let active=true,busy=false,snapshot=null,failure='',epoch=0;
 const live=()=>active&&el.isConnected&&store.get('session')?.actorId===owner;
 const recordsHref=`#/c/${encodeURIComponent(fid)}/life/shared?view=records`;
 const metric=name=>snapshot?.metrics?.[name];
 const known=name=>metric(name)?.available&&metric(name)?.complete;
 function paint(){if(!live())return;
  const relation=known('relationship')?metric('relationship').value:null;
  const relationLabel=storyText(relation?.role?.label),relationSummary=storyText(relation?.summary);
  const cards=storyCards.map(([key,label,unit])=>{
   if(!known(key))return '';
   const value=metric(key).value,isRelationship=key==='relationship';
   if(!isRelationship&&!Number.isFinite(value))return '';
   const text=isRelationship?relationLabel||relationSummary:`${value} ${unit}`;
   return `<button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" class="story-metric" data-story-metric="${key}"><span>${label}</span>${text?`<strong class="${isRelationship?'story-relationship-value':''}">${esc(text)}</strong>`:''}</button>`;
  }).join('');
  const targetHref=row=>row.kind==='memory'?`#/c/${fid}/growth/memory/${encodeURIComponent(row.id)}`:row.kind==='diary'?entryHref(fid,row.id):row.kind==='relationship'?`#/c/${fid}/people/owner`:`#/c/${fid}/chat?${new URLSearchParams({conversation:row.conversation_id,message:row.id})}`;
  const sourceHrefFromStory=row=>sourceHref(targetHref(row),ctx,'返回共同经历');
  const rows=[...(snapshot?.footprints||[])].sort((a,b)=>{const time=row=>row.at?Date.parse(row.at):row.local_date?Date.parse(row.local_date+'T00:00:00Z'):null;return (time(b)??-Infinity)-(time(a)??-Infinity);});
  const footprints=rows.map(row=>{
   const label=Object.hasOwn(footprintLabels,row.kind)?footprintLabels[row.kind]:'来源记录';
   const time=storyTime(row.local_date)||storyTime(row.at),title=storyText(row.title);
   const promiseStatus=Object.hasOwn(storyPromiseStatuses,row.status)?storyPromiseStatuses[row.status]:'';
   return `<article class="story-footprint">
    <p class="small muted">${label}${time?' · '+esc(time):''}</p>
    ${title?`<p class="story-source-excerpt">${esc(title)}</p>`:''}
    ${row.kind==='promise'?`${promiseStatus?`<p class="small muted">${promiseStatus}</p>`:''}
     <button data-domain-ui="button" data-domain-variant="ghost" data-domain-size="default" data-story-promise="${esc(row.id)}">查看约定</button>`:
     `<a data-domain-ui="button" data-domain-variant="ghost" data-domain-size="default" href="${esc(sourceHrefFromStory(row))}">查看记录</a>`}
   </article>`;
  }).join('');
  const asOf=storyTime(snapshot?.as_of);
  el.innerHTML=`<div class="stack story-page">
   <section class="story-overview"><h1 class="reading-business-title">我们</h1>
    <p>${esc(relationSummary||relationLabel||'从每一次真实交流，慢慢积累共同经历。')}</p>
    ${asOf?`<p class="small muted">截至 ${esc(asOf)}</p>`:busy&&!snapshot?'<p class="small muted">正在读取已有记录…</p>':''}
   </section>
   <nav class="row" aria-label="我们的事"><a href="#/c/${encodeURIComponent(fid)}/growth?aboutOwner=true">她记得的事</a><a href="#/c/${encodeURIComponent(fid)}/life/schedule?section=promises">我们的约定</a></nav>
   ${cards?`<details data-domain-ui="disclosure" class="reading-secondary"><summary>共同经历详情</summary><div class="story-metrics">${cards}</div></details>`:''}
   ${failure?`<p data-domain-ui="alert" class="banner info" role="status">部分概览暂不可读取，已知内容保留：${esc(failure)}</p>`:''}
   <div class="row"><h3 class="grow">共同经历与原交流</h3><details data-domain-ui="disclosure"><summary>阅读选项</summary><button data-domain-ui="button" data-domain-variant="ghost" data-domain-size="default" data-basis ${snapshot?'':'disabled'}>回顾依据</button></details></div>
   <section class="reading-paper life-reading-paper">${footprints||`<p class="muted">${snapshot?'还没有可回顾的足印，可以从真实交流开始。':'已有资料暂未读取。'}</p>`}</section>
   <div class="row"><a data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" href="${esc(recordsHref)}">查看全部交流</a><button data-domain-ui="button" data-domain-variant="ghost" data-domain-size="default" data-story-refresh ${busy?'disabled':''}>重新读取概览</button></div>
  </div>`;
  el.querySelectorAll('[data-story-promise]').forEach(button=>button.onclick=()=>openPromise(fid,button.dataset.storyPromise,()=>void load(),live));
  el.querySelector('[data-story-refresh]').onclick=()=>void load();
  el.querySelector('[data-basis]').onclick=()=>drawer({title:'这次回顾的依据',body:storyBasis(snapshot,rows,recordsHref,storyText(store.get('characters',fid)?.name))});
  el.querySelectorAll('[data-story-metric]').forEach(button=>button.onclick=()=>{const key=button.dataset.storyMetric;if(key==='diaries')ctx.navigate(`#/c/${fid}/life/diary`);else if(key==='memories')ctx.navigate(`#/c/${fid}/growth?aboutOwner=true&status=active`);else if(key==='relationship')ctx.navigate(`#/c/${fid}/people/owner`);else el.querySelector('[data-basis]').click();});
 }
 async function load(){if(busy||!live())return;busy=true;const token=++epoch;paint();try{const value=await diaryClient.overview(fid);if(!live()||token!==epoch)return;if(value.fluctlight_id!==fid||value.actor_id!==owner)throw new Error('共同经历读取范围已经变化，请重新打开');snapshot=value;failure='';}catch(error){if(live()&&token===epoch)failure=errorMessage(error);}finally{busy=false;paint();}}
 paint();void load();return()=>{active=false;epoch++;};
}
