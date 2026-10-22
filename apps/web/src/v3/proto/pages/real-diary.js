import {openPromise} from '../components/promise-catalogue.js';
import {sourceHref,sourceReturn} from '../components/source-navigation.js';
import {mountCalendarReading,localDate,reconcileReadingRecord} from '../components/calendar-reading.js';
import {LifeNavigation} from '../components/life-navigation.js';
import {diaryClient} from '../services.js';
import * as store from '../store.js';
import {asyncPage,instant} from '../components/async-page.js';
import {CursorPager,bindCursorPager} from '../components/cursor-pager.js';
import {Field,Tabs,Toggle} from '../components/forms.js';
import {JsonDetails} from '../components/records.js';
import {ConflictPanel} from '../components/revisions.js';
import {beginDraftSubmission,settleDraftSubmission} from '../components/draft-submission.js';
import {esc,badge,confirm,drawer} from '../ui.js';

const labels={pending:'等待日终或执行',running:'正在生成',ready:'已生成',no_content:'本次没有日记',failed:'生成失败',inactive:'角色未处于运行状态'};
const entryHref=(fid,id)=>`#/c/${encodeURIComponent(fid)}/life/diary/${encodeURIComponent(id)}`;
const drafts=new Map();store.subscribe(()=>{if(!store.get('session'))drafts.clear();});

export function renderRealDiaryList(el,ctx){
 const fid=ctx.charId,favorite=ctx.query.favorite==='true',zone=store.get('characters',fid)?.timezone;
 let active=true,reader;
 el.innerHTML=`${LifeNavigation(ctx)}<div class="row" style="margin:16px 0">${Tabs({value:favorite?'favorite':'all',items:[{value:'all',label:'全部日记',href:`#/c/${fid}/life/diary`},{value:'favorite',label:'收藏',href:`#/c/${fid}/life/diary?favorite=true`}]})}<a class="btn ghost" href="#/c/${fid}/life/shared">我们的故事</a></div><div data-diary-reading></div><details class="card" style="margin-top:20px"><summary>补记已结束日期的日记</summary><form class="stack" data-generate>${Field({label:'日期',name:'localDate',type:'date',attrs:{required:true}})}${Field({label:'时区',name:'timezone',value:zone||'',attrs:{required:true}})}<p class="small muted">选择已结束的本地日期。已有日记会保留；提交后查看这次处理的状态。</p><button class="btn primary">生成这一天的日记</button><p data-feedback role="alert"></p></form></details>`;
 reader=mountCalendarReading(el.querySelector('[data-diary-reading]'),{scope:[fid,'diary',favorite],zone,initialDate:ctx.query.date||'',initialMonth:ctx.query.month||'',label:'日记',onView:view=>{ctx.raw=ctx.path+'?'+new URLSearchParams({...view,...(favorite?{favorite:'true'}:{})});history.replaceState(null,'','#'+ctx.raw);},read:cursor=>diaryClient.list(fid,cursor,favorite),dateOf:d=>d.date,
  toolbar:'<p class="small muted">日记保留她自己的表达，来源和你的注记分别记录。</p>',
  render:d=>`<article class="reading-paper"><div class="row"><h3 class="grow">${esc(d.date)}</h3>${d.annotation.favorite?badge('已收藏'):''}${badge(labels[d.status]||d.status,'plain')}</div><p class="paper-body">${esc(d.body||'尚无正文，打开查看处理进度与来源。')}</p><a class="btn" href="${esc(sourceHref(entryHref(fid,d.id),ctx,'返回日记日历'))}">阅读这篇日记</a></article>`});
 const form=el.querySelector('[data-generate]');
 const check=()=>{let today='';try{today=localDate(new Date().toISOString(),form.timezone.value);}catch{}form.querySelector('button').disabled=!today||!form.localDate.value||form.localDate.value>=today||store.get('characters',fid)?.status==='已退休';};
 form.oninput=check;check();
 form.onsubmit=async event=>{event.preventDefault();check();const button=form.querySelector('button');if(button.disabled)return;button.disabled=true;try{const result=await diaryClient.generate(fid,{localDate:form.localDate.value,timezone:form.timezone.value.trim(),reason:'Owner请求回顾已结束日期'});if(active)ctx.navigate(entryHref(fid,result.diary_id));}catch(error){if(active){form.querySelector('[data-feedback]').textContent=error.message;check();}}};
 return()=>{active=false;reader();};
}

function sourceRows(fid,entry){
 const wanted=new Set(entry.refs);
 return (entry.source_snapshot?.sources||[]).filter(s=>wanted.has(s.ref)).map(source=>{
  const id=source.ref.replace(/^[^:]+:/,'');
  const link=source.kind==='remote_communication'?`#/c/${encodeURIComponent(fid)}/chat?${new URLSearchParams({conversation:source.conversation_id,message:id})}`:`#/c/${encodeURIComponent(fid)}/world/event/${encodeURIComponent(id)}`;
  return `<div class="lrow"><a class="grow" href="${esc(link)}">${esc(source.kind==='remote_communication'?'原交流消息':'已确认生活事件')} ${esc(id)}</a>${source.kind==='remote_communication'?badge('远程交流','plain'):''}${JsonDetails(source,'查看原来源快照')}</div>`;
 }).join('')||'<p class="small muted">还没有已提交正文的引用来源；生成尝试和没有产出的原因见下方。</p>';
}
export function renderRealDiaryEntry(el,ctx){
 const fid=ctx.charId,id=ctx.params.entryId,key=JSON.stringify([store.get('session')?.actorId,fid,id]);
 return asyncPage(el,()=>diaryClient.read(fid,id),(entry,reload,alive)=>{
  reconcileReadingRecord(fid,'diary',entry,(scope,row)=>!scope[2]||row.annotation.favorite);
  if(!drafts.has(key))drafts.set(key,{note:entry.annotation.note,favorite:entry.annotation.favorite,base:entry.annotation.revision,reply:'',dirty:false,pending:{}});
  const draft=drafts.get(key);
  if(!draft.dirty&&!draft.pending.note&&entry.annotation.revision>=draft.base)Object.assign(draft,{note:entry.annotation.note,favorite:entry.annotation.favorite,base:entry.annotation.revision});
  const conflict=draft.base<entry.annotation.revision;
  el.innerHTML=`<div class="stack"><div class="reading-paper"><div class="row diary-entry-head"><h2 class="grow">${esc(entry.date)}</h2>${badge('角色主观记录','plain')}${badge(labels[entry.status]||entry.status)}<button class="btn" data-reload>重新读取</button></div><p class="small muted">${esc(entry.timezone)} · 正文来源：独立人物表达 · v${esc(entry.revision)}</p>${entry.body?`<p class="memory-content">${esc(entry.body)}</p>`:`<p>${entry.status==='no_content'?'这次没有足够或适合的素材形成日记。可以继续交流、生活，之后回顾相应日期。':'尚无已提交正文。请查看实际任务和状态，受理不代表已经生成。'}</p>`}${entry.last_error_code?`<p class="banner err" role="alert">${esc(entry.last_error_code)}</p>`:''}${['failed','inactive'].includes(entry.status)?'<button class="btn" data-retry>按当前状态重新生成</button>':''}<p data-generation-feedback role="alert"></p></div><div class="card"><h3>素材与来源</h3>${sourceRows(fid,entry)}${JsonDetails(entry.persona_provenance,'本次表达所用人格版本')}${JsonDetails(entry.model_provenance,'模型来源与关联标识')}</div><div class="card"><h3>生成尝试与实际任务</h3>${(entry.attempts||[]).map(a=>`<div class="lrow"><div class="grow"><b>尝试 ${esc(a.attempt)} · ${esc(labels[a.status]||a.status)}</b><p class="small muted">${esc(instant(a.created_at))} · ${esc(a.error_code||a.plan?.reason||'')}</p></div>${a.task_id?`<a class="btn sm" href="#/diag/task/${encodeURIComponent(a.task_id)}">实际任务</a>`:''}</div>`).join('')}${entry.has_more_attempts?'<p class="small muted">显示最近20次尝试；更早执行记录仍保留在任务诊断中。</p>':''}</div><div class="card"><form class="stack" data-note><h3>你的收藏与注记</h3><p class="small muted">这些字段属于你，不修改她的主观正文。</p>${conflict?ConflictPanel({baseVersion:draft.base,currentVersion:entry.annotation.revision}):''}${Toggle({label:'收藏这篇日记',name:'favorite',checked:draft.favorite})}${Field({label:'你的注记',name:'note',type:'textarea',value:draft.note,attrs:{maxlength:5000}})}<div class="row"><button type="button" class="btn" data-reset>撤销草稿</button><button class="btn primary" ${conflict||draft.pending.note?'disabled':''}>保存注记</button></div><p role="alert" data-note-error></p>${conflict?JsonDetails(entry.annotation,'最新已保存注记（草稿保留）'):''}</form></div><a class="btn" href="${esc(sourceReturn(ctx,`#/c/${fid}/life/diary`))}">${esc(ctx.query.returnLabel||'返回日记日历')}</a></div>`;
  el.querySelector('[data-reload]').onclick=reload;
  el.querySelector('[data-retry]')?.addEventListener('click',async event=>{event.target.disabled=true;try{await diaryClient.generate(fid,{localDate:entry.date,timezone:entry.timezone,expectedRevision:entry.revision,reason:'Owner在失败或未运行后明确重试日记'});if(alive())await reload();}catch(error){if(alive()){el.querySelector('[data-generation-feedback]').textContent=error.message;event.target.disabled=false;}}});
  const note=el.querySelector('[data-note]');note.oninput=note.onchange=()=>{draft.note=note.note.value;draft.favorite=note.favorite.checked;draft.dirty=true;};
  el.querySelector('[data-reset]').onclick=()=>{draft.dirty=false;void reload();};
  el.querySelector('[data-keep]')?.addEventListener('click',()=>{el.querySelector('[data-note-error]').textContent='注记草稿已保留。';});
  el.querySelector('[data-refresh]')?.addEventListener('click',async()=>{if(await confirm({title:'核对最新注记后采用新基线',text:'保留当前草稿。请核对上方最新已保存注记，后续保存会基于它的版本。',identity:'所有者修改'})&&alive()){draft.base=entry.annotation.revision;void reload();}});
  const settled=(channel,error)=>{
   if(!alive())return;
   if(channel==='note'){
    note.querySelector('.primary').disabled=false;
    note.querySelector('[data-note-error]').textContent=error?.message||(draft.dirty?'本次提交已保存，期间的新草稿保留。':'注记已保存');
    if(error?.status===409||(!error&&!draft.dirty))void reload();

   }
  };
  draft.onSettled=settled;
  note.onsubmit=async event=>{
   event.preventDefault();if(draft.pending.note)return;
   const submission=beginDraftSubmission(draft,'note',{favorite:draft.favorite,note:draft.note});
   note.querySelector('.primary').disabled=true;
   try{
    const saved=await diaryClient.annotate(fid,id,{...submission.value,expectedRevision:draft.base});
    if(drafts.get(key)!==draft)return;
    const result=settleDraftSubmission(draft,submission,{favorite:draft.favorite,note:draft.note});
    if(result){draft.base=Math.max(draft.base,saved.revision);draft.dirty=!result.unchanged;draft.onSettled?.('note');}
   }catch(error){if(drafts.get(key)!==draft)return;settleDraftSubmission(draft,submission,null);draft.dirty=true;draft.onSettled?.('note',error);}
  };
  return()=>{if(draft.onSettled===settled)delete draft.onSettled;};
 });
}
export function renderRealShared(el,ctx){
 const fid=ctx.charId;
 if(ctx.query.view==='records'){
  const cursors=[''];let at=0;
  return asyncPage(el,()=>diaryClient.shared(fid,cursors[at]),(page,reload)=>{
   el.innerHTML=`${LifeNavigation(ctx)}<div class="stack"><h2>交流记录</h2><p class="small muted">保留双方实际参与时段内的原交流；远程交流不表示物理同场。</p><div class="reading-paper">${page.items.map(r=>`<div class="story-footprint"><p class="small muted">${esc(instant(r.occurred_at))}</p><p>${esc(r.title)}</p><a class="btn ghost" href="${esc(sourceHref(r.href,ctx,'返回共同交流'))}">查看原交流</a></div>`).join('')||'<p>当前读取范围没有交流记录。</p>'}</div>${CursorPager({more:!!page.next_cursor,previous:at>0,label:'交流'})}<button class="btn" data-story-back>返回我们的故事</button></div>`;
   el.querySelector('[data-story-back]').onclick=ctx.goBack;
   bindCursorPager(el,{next:()=>{cursors[++at]=page.next_cursor;void reload();},previous:()=>{at--;void reload();}});
  });
 }
 const owner=store.get('session')?.actorId;let active=true,busy=false,snapshot=null,failure='',epoch=0;
 const live=()=>active&&el.isConnected&&store.get('session')?.actorId===owner;
 const metric=name=>snapshot?.metrics?.[name];
 const known=name=>metric(name)?.available&&metric(name)?.complete;
 function paint(){if(!live())return;
  const relation=known('relationship')?metric('relationship').value:null;
  const cards=[['companionship_days','陪伴天数','天'],['completed_turns','完成回复轮次','轮'],['relationship','现在的我们',''],['memories','关于你的记忆','条'],['diaries','她的日记','篇']];
  const targetHref=row=>row.kind==='memory'?`#/c/${fid}/growth/memory/${encodeURIComponent(row.id)}`:row.kind==='diary'?entryHref(fid,row.id):row.kind==='relationship'?`#/c/${fid}/people/owner`:`#/c/${fid}/chat?${new URLSearchParams({conversation:row.conversation_id,message:row.id})}`;
  const sourceHrefFromStory=row=>sourceHref(targetHref(row),ctx,'返回我们的故事');
  const rows=[...(snapshot?.footprints||[])].sort((a,b)=>{const time=row=>row.at?Date.parse(row.at):row.local_date?Date.parse(row.local_date+'T00:00:00Z'):null;return (time(b)??-Infinity)-(time(a)??-Infinity);});
  el.innerHTML=`${LifeNavigation(ctx)}<div class="stack story-page"><section class="story-overview"><h2>我们的故事</h2><p>${esc(relation?.summary||relation?.role?.label||'从每一次真实交流，慢慢积累我们的故事。')}</p><p class="small muted">${snapshot?'截至 '+esc(instant(snapshot.as_of,snapshot.timezone)):'正在读取已有记录…'}</p></section><div class="story-metrics">${cards.map(([key,label,unit])=>{let text='—';if(known(key))text=key==='relationship'?(relation?.role?.label||relation?.summary||'尚无关系记录'):String(metric(key).value)+' '+unit;return `<button class="story-metric" data-story-metric="${key}" ${known(key)?'':'disabled'}><span>${label}</span><strong>${esc(text)}</strong></button>`;}).join('')}</div>${failure?`<p class="banner info" role="status">部分概览暂不可读取，已知内容保留：${esc(failure)}</p>`:''}<div class="row"><h3 class="grow">关系足印</h3><button class="btn ghost" data-basis ${snapshot?'':'disabled'}>回顾依据</button></div><section class="reading-paper">${rows.map(row=>`<article class="story-footprint"><p class="small muted">${esc(({memory:'她记下的事',diary:'她的主观日记',remote_communication:'远程交流',relationship:'关系记录',promise:'约定'})[row.kind]||'来源记录')} · ${row.local_date?esc(row.local_date):row.at?esc(instant(row.at,snapshot.timezone)):'时间未记录'}${row.revision?' · v'+esc(row.revision):''}</p><p>${esc(row.title)}</p>${row.kind==='promise'?`<p class="small muted">${esc(row.status==='awaiting_goal'?'等待规划，尚未接受':row.status==='deferred'?'暂缓决定':'已接受，履行待证据')}</p><button class="btn ghost" data-story-promise="${esc(row.id)}">查看约定</button>`:`<a class="btn ghost" href="${esc(sourceHrefFromStory(row))}">查看记录</a>`}</article>`).join('')||`<p class="muted">${snapshot?'还没有可回顾的足印，可以从真实交流开始。':'已有资料暂未读取。'}</p>`}</section><div class="row"><a class="btn" href="#/c/${fid}/life/shared?view=records">查看全部交流</a><button class="btn ghost" data-story-refresh ${busy?'disabled':''}>重新读取概览</button></div></div>`;
  el.querySelectorAll('[data-story-promise]').forEach(button=>button.onclick=()=>openPromise(fid,button.dataset.storyPromise,()=>void load(),live));
  el.querySelector('[data-story-refresh]').onclick=()=>void load();
  el.querySelector('[data-basis]').onclick=()=>drawer({title:'这次回顾的依据',body:`<p>读取时间：${esc(instant(snapshot.as_of,snapshot.timezone))}</p><p>范围：${esc(store.get('characters',fid)?.name||fid)}与当前所有者，当前人格 ${esc(snapshot.profile_id)}。统计使用同一次只读快照。足印展示最近一部分来源，不代表完整历史。</p>${JsonDetails(snapshot,'统计口径、实际对象与版本')}`});
  el.querySelectorAll('[data-story-metric]').forEach(button=>button.onclick=()=>{const key=button.dataset.storyMetric;if(key==='diaries')ctx.navigate(`#/c/${fid}/life/diary`);else if(key==='memories')ctx.navigate(`#/c/${fid}/growth?aboutOwner=true&status=active`);else if(key==='relationship')ctx.navigate(`#/c/${fid}/people/owner`);else el.querySelector('[data-basis]').click();});
 }
 async function load(){if(busy||!live())return;busy=true;const token=++epoch;paint();try{const value=await diaryClient.overview(fid);if(!live()||token!==epoch)return;if(value.fluctlight_id!==fid||value.actor_id!==owner)throw new Error('故事读取范围已经变化，请重新打开');snapshot=value;failure='';}catch(error){if(live()&&token===epoch)failure=error.message;}finally{busy=false;paint();}}
 paint();void load();return()=>{active=false;epoch++;};
}
