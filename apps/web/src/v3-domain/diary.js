import {errorMessage} from '../v3/domain-presentation.mjs';
import {openPromise} from '../v3/proto/components/promise-catalogue.js';
import {sourceHref,sourceReturn} from '../v3/proto/components/source-navigation.js';
import {mountCalendarReading,localDate,reconcileReadingRecord} from '../v3/proto/components/calendar-reading.js';
import {diaryClient} from '../v3/proto/services.js';
import * as store from '../v3/proto/store.js';
import {asyncPage,instant} from '../v3/proto/components/async-page.js';
import {CursorPager,bindCursorPager} from '../v3/proto/components/cursor-pager.js';
import {Field,Tabs,Toggle} from '../v3/proto/components/forms.js';
import {JsonDetails} from '../v3/proto/components/records.js';
import {ConflictPanel} from '../v3/proto/components/revisions.js';
import {beginDraftSubmission,settleDraftSubmission} from '../v3/proto/components/draft-submission.js';
import {esc,badge,confirm,drawer,toast} from '../v3/proto/ui.js';

const labels={pending:'正在等待日记生成',running:'正在生成',ready:'已生成',no_content:'本次没有日记',failed:'生成失败',inactive:'角色未处于运行状态'};
const entryHref=(fid,id)=>`#/c/${encodeURIComponent(fid)}/life/diary/${encodeURIComponent(id)}`;
const drafts=new Map(),generationDrafts=new Map();store.subscribe(()=>{if(!store.get('session')){drafts.clear();generationDrafts.clear();}});

export function renderRealDiaryList(el,ctx){
 const fid=ctx.charId,favorite=ctx.query.favorite==='true',zone=store.get('characters',fid)?.timezone;
 const key=JSON.stringify([store.get('session')?.actorId,fid]);
 if(!generationDrafts.has(key))generationDrafts.set(key,{localDate:'',timezone:zone||'Asia/Shanghai',pending:false,error:'',command:null,result:null});
 const draft=generationDrafts.get(key);
 let active=true,reader,generationDrawer;
 el.innerHTML=`<div class="stack reading-workspace"><div class="row reading-page-heading"><h1 class="reading-business-title grow">她的日记</h1><button class="btn" data-open-generation>补写日记</button></div><div class="row diary-reading-toolbar">${Tabs({attrs:{role:'group','aria-label':'日记筛选'},value:favorite?'favorite':'all',items:[{value:'all',label:'全部日记',href:`#/c/${fid}/life/diary`},{value:'favorite',label:'收藏',href:`#/c/${fid}/life/diary?favorite=true`}]})}</div><div data-diary-reading></div></div>`;
 reader=mountCalendarReading(el.querySelector('[data-diary-reading]'),{scope:[fid,'diary',favorite],zone,initialDate:ctx.query.date||'',initialMonth:ctx.query.month||'',initialMode:ctx.query.mode||'list',visible:d=>typeof d.body==='string'&&!!d.body.trim(),emptyText:'还没有读到她写下的日记。',label:'日记',renderHidden:d=>`<article class="card diary-generation-state"><b>${esc(d.date)} · ${esc(labels[d.status]||'没有正文')}</b>${['pending','running'].includes(d.status)?'<progress aria-label="日记生成中"></progress><p>正在整理这一天的经历，可以稍后回来查看。</p>':d.status==='failed'?'<p>这次日记生成失败，可查看原因并从原记录恢复。</p>':'<p>这一天还没有形成日记正文。</p>'}<a class="btn" href="${esc(entryHref(fid,d.id))}">${d.status==='failed'?'查看原因与恢复':'查看生成进度'}</a></article>`,onView:view=>{ctx.raw=ctx.path+'?'+new URLSearchParams({...view,...(favorite?{favorite:'true'}:{})});history.replaceState(null,'','#'+ctx.raw);},read:cursor=>diaryClient.list(fid,cursor,favorite),dateOf:d=>d.date,
  toolbar:'',
  render:d=>`<article class="reading-paper life-reading-paper"><div class="row"><h3 class="grow">${esc(d.date)}</h3>${d.annotation.favorite?badge('已收藏'):''}${badge(labels[d.status]||d.status,'plain')}</div><p class="paper-body">${esc(d.body||(d.date===localDate(new Date().toISOString(),zone||'Asia/Shanghai')?'今晚她会写下今天的日记，有合适素材时才会形成正文。':['pending','running'].includes(d.status)?'正在整理这一天的经历，打开可查看生成进度。':d.status==='failed'?'这篇日记没有生成，打开可查看原因并重试。':'还没有正文，打开查看这次回顾的结果。'))}</p><a class="btn" href="${esc(sourceHref(entryHref(fid,d.id),ctx,'返回日记'))}">阅读这篇日记</a></article>`});
 function openGeneration(){
  generationDrawer=drawer({title:'生成日记',body:`<form class="stack" data-generate>${Field({label:'日期',name:'localDate',type:'date',value:draft.localDate,attrs:{required:true}})}${Field({label:'时区',name:'timezone',value:draft.timezone,attrs:{required:true}})}<p class="small muted">选择已结束的本地日期。已有日记会保留；提交后查看这次处理的状态。</p><p data-disabled-reason role="status"></p><p data-feedback role="alert">${esc(draft.error)}</p><button class="btn primary" type="submit">生成这一天的日记</button>${draft.result?`<a class="btn" href="${esc(entryHref(fid,draft.result.diary_id))}" data-result>查看原处理结果</a>`:''}</form>`,onClose:()=>{generationDrawer=null;delete draft.update;}});
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

function sourceRows(fid,entry){
 const wanted=new Set(entry.refs);
 return (entry.source_snapshot?.sources||[]).filter(s=>wanted.has(s.ref)).map(source=>{
  const id=source.ref.replace(/^[^:]+:/,'');
  const link=source.kind==='remote_communication'?`#/c/${encodeURIComponent(fid)}/chat?${new URLSearchParams({conversation:source.conversation_id,message:id})}`:`#/c/${encodeURIComponent(fid)}/world/event/${encodeURIComponent(id)}`;
  return `<div class="lrow"><a class="grow" href="${esc(link)}">${esc(source.kind==='remote_communication'?'原交流消息':'已确认生活事件')} ${esc(id)}</a>${source.kind==='remote_communication'?badge('远程交流','plain'):''}${JsonDetails(source,'查看原来源快照')}</div>`;
 }).join('')||'<p class="small muted">还没有已提交正文的引用来源；生成尝试和没有产出的原因见下方。</p>';
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
  el.innerHTML=`<div class="stack reading-workspace"><div class="row reading-page-heading"><h1 class="reading-business-title grow">${esc(entry.date)} · 日记</h1><button class="btn" data-open-note>收藏与注记${draft.dirty?' · 有草稿':''}</button></div><div class="reading-paper life-reading-paper"><div class="row diary-entry-head">${badge('角色主观记录','plain')}${badge(labels[entry.status]||entry.status)}<button class="btn" data-reload>重新读取</button></div><p class="small muted">按日记原日期归档 · 正文来源：独立人物表达 · v${esc(entry.revision)}</p>${entry.body?`<p class="memory-content">${esc(entry.body)}</p>`:`<p>${entry.status==='no_content'?'这次没有足够或适合的素材形成日记。可以继续交流、生活，之后回顾相应日期。':'尚无已提交正文。请查看实际任务和状态，受理不代表已经生成。'}</p>`}${entry.last_error_code?`<p class="banner err" role="alert">${esc(errorMessage(entry.last_error_code,'这篇日记没有生成，请查看任务原因后重试。'))}</p>`:''}${['failed','inactive'].includes(entry.status)?'<button class="btn" data-retry>按当前状态重新生成</button>':''}<p data-generation-feedback role="alert"></p></div><section class="card reading-annotation"><div class="row"><h3 class="grow">你的收藏与注记</h3><span data-saved-favorite>${entry.annotation.favorite?badge('已收藏','plain'):''}</span></div><p class="memory-content" data-saved-note>${esc(entry.annotation.note||'还没有已保存的注记。')}</p><p class="small muted" data-note-receipt>${draft.pending.note?'原注记正在保存，期间的新草稿将保留。':draft.dirty?'你有尚未保存的注记草稿。':'注记属于你，不修改她的主观正文。'}</p></section><details class="card reading-secondary"><summary>素材与来源</summary>${sourceRows(fid,entry)}${JsonDetails(entry.persona_provenance,'本次表达所用人格版本')}${JsonDetails(entry.model_provenance,'模型来源与关联标识')}</details><details class="card reading-secondary"><summary>生成尝试与实际任务</summary>${(entry.attempts||[]).map(a=>`<div class="lrow"><div class="grow"><b>尝试 ${esc(a.attempt)} · ${esc(labels[a.status]||a.status)}</b><p class="small muted">${esc(instant(a.created_at))} · ${esc(a.error_code||a.plan?.reason||'')}</p></div>${a.task_id?`<a class="btn sm" href="#/diag/task/${encodeURIComponent(a.task_id)}">实际任务</a>`:''}</div>`).join('')}${entry.has_more_attempts?'<p class="small muted">显示最近20次尝试；更早执行记录仍保留在任务诊断中。</p>':''}</details><div data-note-host hidden><form class="stack" data-note><h3>你的收藏与注记</h3><p class="small muted">这些字段属于你，不修改她的主观正文。</p>${conflict?ConflictPanel({baseVersion:draft.base,currentVersion:entry.annotation.revision}):''}${Toggle({label:'收藏这篇日记',name:'favorite',checked:draft.favorite})}${Field({label:'你的注记',name:'note',type:'textarea',value:draft.note,attrs:{maxlength:5000}})}<div class="row"><button type="button" class="btn" data-reset ${draft.pending.note?'disabled':''}>撤销草稿</button><button class="btn primary" ${conflict||draft.pending.note?'disabled':''}>保存注记</button></div><p role="alert" data-note-error></p>${conflict?JsonDetails(entry.annotation,'最新已保存注记（草稿保留）'):''}</form></div><a class="btn" href="${esc(sourceReturn(ctx,`#/c/${fid}/life/diary`))}">${esc(ctx.query.returnLabel||'返回日记')}</a></div>`;
  el.querySelector('[data-reload]').onclick=reload;
  el.querySelector('[data-retry]')?.addEventListener('click',async event=>{event.target.disabled=true;try{await diaryClient.generate(fid,{localDate:entry.date,timezone:entry.timezone,expectedRevision:entry.revision,reason:'Owner在失败或未运行后明确重试日记'});if(alive())await reload();}catch(error){if(alive()){el.querySelector('[data-generation-feedback]').textContent=errorMessage(error);event.target.disabled=false;}}});
  const note=el.querySelector('[data-note]'),host=el.querySelector('[data-note-host]');let noteDrawer;
  el.querySelector('[data-open-note]').onclick=()=>{noteDrawer=drawer({title:'你的收藏与注记',onClose:()=>{host.append(note);noteDrawer=null;}});noteDrawer.body.append(note);};note.oninput=note.onchange=()=>{draft.note=note.note.value;draft.favorite=note.favorite.checked;draft.dirty=true;el.querySelector('[data-open-note]').textContent='收藏与注记 · 有草稿';if(!draft.pending.note)el.querySelector('[data-note-receipt]').textContent='你有尚未保存的注记草稿。';};
  el.querySelector('[data-reset]').onclick=()=>{if(draft.pending.note)return;draft.dirty=false;void reload();};
  el.querySelector('[data-keep]')?.addEventListener('click',()=>{note.querySelector('[data-note-error]').textContent='注记草稿已保留。';});
  el.querySelector('[data-refresh]')?.addEventListener('click',async()=>{if(await confirm({title:'核对最新注记后采用新基线',text:'保留当前草稿。请核对上方最新已保存注记，后续保存会基于它的版本。',identity:'所有者修改'})&&alive()){draft.base=entry.annotation.revision;void reload();}});
  const settled=(channel,error,saved)=>{
   if(!alive())return;
   if(channel==='note'){
    note.querySelector('.primary').disabled=draft.base<entry.annotation.revision;
    note.querySelector('[data-reset]').disabled=false;
    if(saved){el.querySelector('[data-saved-note]').textContent=saved.note||'还没有已保存的注记。';el.querySelector('[data-saved-favorite]').innerHTML=saved.favorite?badge('已收藏','plain'):'';}
    el.querySelector('[data-note-receipt]').textContent=(error?errorMessage(error):'')||(draft.dirty?'本次提交已保存，期间的新草稿保留。':'注记已保存');
    note.querySelector('[data-note-error]').textContent=(error?errorMessage(error):'')||(draft.dirty?'本次提交已保存，期间的新草稿保留。':'注记已保存');
    if(!error)toast(draft.dirty?'注记已保存，新草稿已保留':'注记已保存');if(error?.status===409||(!error&&!draft.dirty))void reload();

   }
  };
  draft.onSettled=settled;
  note.onsubmit=async event=>{
   event.preventDefault();if(draft.pending.note||draft.base<entry.annotation.revision)return;
   const submission=beginDraftSubmission(draft,'note',{favorite:draft.favorite,note:draft.note});
   note.querySelector('.primary').disabled=true;note.querySelector('[data-reset]').disabled=true;
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
export function renderRealShared(el,ctx){
 const fid=ctx.charId;
 if(ctx.query.view==='records'){
  const cursors=[''];let at=0;
  return asyncPage(el,()=>diaryClient.shared(fid,cursors[at]),(page,reload)=>{
   el.innerHTML=`<div class="stack story-page"><h1 class="reading-business-title">交流记录</h1><p class="small muted">保留双方实际参与时段内的原交流；远程交流不表示物理同场。</p><div class="reading-paper life-reading-paper">${page.items.map(r=>`<div class="story-footprint"><p class="small muted">${esc(instant(r.occurred_at))}</p><p>${esc(r.title)}</p><a class="btn ghost" href="${esc(sourceHref(r.href,ctx,'返回共同交流'))}">查看原交流</a></div>`).join('')||'<p>当前读取范围没有交流记录。</p>'}</div>${CursorPager({more:!!page.next_cursor,previous:at>0,label:'交流'})}<button class="btn" data-story-back>返回共同经历</button></div>`;
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
  const sourceHrefFromStory=row=>sourceHref(targetHref(row),ctx,'返回共同经历');
  const rows=[...(snapshot?.footprints||[])].sort((a,b)=>{const time=row=>row.at?Date.parse(row.at):row.local_date?Date.parse(row.local_date+'T00:00:00Z'):null;return (time(b)??-Infinity)-(time(a)??-Infinity);});
  el.innerHTML=`<div class="stack story-page"><section class="story-overview"><h1 class="reading-business-title">我们</h1><p>${esc(relation?.summary||relation?.role?.label||'从每一次真实交流，慢慢积累共同经历。')}</p><p class="small muted">${snapshot?'截至 '+esc(instant(snapshot.as_of,snapshot.timezone)):'正在读取已有记录…'}</p></section><nav class="row" aria-label="我们的事"><a href="#/c/${encodeURIComponent(fid)}/growth?aboutOwner=true">她记得的事</a><a href="#/c/${encodeURIComponent(fid)}/life/schedule?section=promises">我们的约定</a></nav><details class="reading-secondary"><summary>共同经历详情</summary><div class="story-metrics">${cards.map(([key,label,unit])=>{let text='—';if(known(key))text=key==='relationship'?(relation?.role?.label||relation?.summary||'尚无关系记录'):String(metric(key).value)+' '+unit;return `<button class="story-metric" data-story-metric="${key}" ${known(key)?'':'disabled'}><span>${label}</span><strong class="${key==='relationship'?'story-relationship-value':''}">${esc(text)}</strong></button>`;}).join('')}</div></details>${failure?`<p class="banner info" role="status">部分概览暂不可读取，已知内容保留：${esc(failure)}</p>`:''}<div class="row"><h3 class="grow">共同经历与原交流</h3><details><summary>阅读选项</summary><button class="btn ghost" data-basis ${snapshot?'':'disabled'}>回顾依据</button></details></div><section class="reading-paper life-reading-paper">${rows.map(row=>`<article class="story-footprint"><p class="small muted">${esc(({memory:'她记下的事',diary:'她的主观日记',remote_communication:'远程交流',relationship:'关系记录',promise:'约定'})[row.kind]||'来源记录')} · ${row.local_date?esc(row.local_date):row.at?esc(instant(row.at,snapshot.timezone)):'时间未记录'}${row.revision?' · v'+esc(row.revision):''}</p><p class="story-source-excerpt">${esc(row.title)}</p>${row.kind==='promise'?`<p class="small muted">${esc(row.status==='awaiting_goal'?'等待规划，尚未接受':row.status==='deferred'?'暂缓决定':'已接受，履行待证据')}</p><button class="btn ghost" data-story-promise="${esc(row.id)}">查看约定</button>`:`<a class="btn ghost" href="${esc(sourceHrefFromStory(row))}">查看记录</a>`}</article>`).join('')||`<p class="muted">${snapshot?'还没有可回顾的足印，可以从真实交流开始。':'已有资料暂未读取。'}</p>`}</section><div class="row"><a class="btn" href="#/c/${fid}/life/shared?view=records">查看全部交流</a><button class="btn ghost" data-story-refresh ${busy?'disabled':''}>重新读取概览</button></div></div>`;
  el.querySelectorAll('[data-story-promise]').forEach(button=>button.onclick=()=>openPromise(fid,button.dataset.storyPromise,()=>void load(),live));
  el.querySelector('[data-story-refresh]').onclick=()=>void load();
  el.querySelector('[data-basis]').onclick=()=>drawer({title:'这次回顾的依据',body:`<p>读取时间：${esc(instant(snapshot.as_of,snapshot.timezone))}</p><p>范围：${esc(store.get('characters',fid)?.name||fid)}与当前所有者，当前人格 ${esc(snapshot.profile_id)}。统计使用同一次只读快照。足印展示最近一部分来源，不代表完整历史。</p>${JsonDetails(snapshot,'统计口径、实际对象与版本')}`});
  el.querySelectorAll('[data-story-metric]').forEach(button=>button.onclick=()=>{const key=button.dataset.storyMetric;if(key==='diaries')ctx.navigate(`#/c/${fid}/life/diary`);else if(key==='memories')ctx.navigate(`#/c/${fid}/growth?aboutOwner=true&status=active`);else if(key==='relationship')ctx.navigate(`#/c/${fid}/people/owner`);else el.querySelector('[data-basis]').click();});
 }
 async function load(){if(busy||!live())return;busy=true;const token=++epoch;paint();try{const value=await diaryClient.overview(fid);if(!live()||token!==epoch)return;if(value.fluctlight_id!==fid||value.actor_id!==owner)throw new Error('共同经历读取范围已经变化，请重新打开');snapshot=value;failure='';}catch(error){if(live()&&token===epoch)failure=errorMessage(error);}finally{busy=false;paint();}}
 paint();void load();return()=>{active=false;epoch++;};
}
