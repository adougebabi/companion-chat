import {errorMessage} from '../../domain-presentation.mjs';
import {esc} from '../ui.js';
import {Field} from './forms.js';
import {displayTimezone} from '../../presentation-time.mjs';
import * as store from '../store.js';

const views=new Map();
store.subscribe(()=>{if(!store.get('session'))views.clear();});
export function localDate(value,zone){
 if(!value||!Number.isFinite(Date.parse(value)))return '';
 const parts=new Intl.DateTimeFormat('en-CA',{timeZone:displayTimezone(zone),year:'numeric',month:'2-digit',day:'2-digit'}).formatToParts(new Date(value));
 const part=type=>parts.find(p=>p.type===type)?.value;
 return `${part('year')}-${part('month')}-${part('day')}`;
}
const validDate=value=>/^\d{4}-\d{2}-\d{2}$/.test(value)&&Number.isFinite(Date.parse(value+'T00:00:00Z'))&&new Date(value+'T00:00:00Z').toISOString().slice(0,10)===value;
// Bounded, explicit cursor aggregation. Calendar dots mean loaded records only.
// An uncovered date stays unknown; a partial page never proves an empty month.
export function mountCalendarReading(el,{scope,zone,read,dateOf,render,label='记录',initialDate='',initialMonth='',initialMode='calendar',toolbar='',bindToolbar,visible=()=>true,renderHidden=null,emptyText='',filtered=false,onView=()=>{}}){
 const actor=store.get('session')?.actorId;
 const key=JSON.stringify([actor,scope]);
 let state=views.get(key);
 if(!state){state={scope,rows:[],cursor:'',started:false,complete:false,month:validDate(initialDate)?initialDate.slice(0,7):/^\d{4}-(0[1-9]|1[0-2])$/.test(initialMonth)?initialMonth:localDate(new Date().toISOString(),zone).slice(0,7),date:validDate(initialDate)?initialDate:'',dateDraft:initialDate,following:true,rangeSelected:validDate(initialDate)||/^\d{4}-(0[1-9]|1[0-2])$/.test(initialMonth),mode:initialMode==='list'?'list':'calendar'};views.set(key,state);}
 if(validDate(initialDate)){state.rangeSelected=true;state.date=initialDate;state.dateDraft=initialDate;state.month=initialDate.slice(0,7);}else if(/^\d{4}-(0[1-9]|1[0-2])$/.test(initialMonth)){state.rangeSelected=true;state.month=initialMonth;state.date='';state.dateDraft='';}
 let timer=null;const current=()=>active&&el.isConnected&&store.get('session')?.actorId===actor;
 let active=true,busy=false,error='',notice='',epoch=0;
 el.innerHTML=`<div class="calendar-reading"><details class="reading-date-options" ${state.rangeSelected?'open':''}><summary>按日期查看</summary><aside class="reading-sidebar" data-calendar></aside></details><section class="reading-flow"><div data-toolbar>${toolbar}</div><div data-reading-status role="status" aria-live="polite"></div><div class="stack" data-papers></div><div class="row"><button class="btn" data-reading-more>加载更多${esc(label)}</button><details class="reading-options"><summary>阅读选项</summary><button class="btn ghost" data-reading-refresh>刷新内容</button></details></div></section></div>`;
 const calendar=el.querySelector('[data-calendar]'),papers=el.querySelector('[data-papers]'),status=el.querySelector('[data-reading-status]');
 const dated=()=>state.rows.map(row=>dateOf(row)).filter(Boolean).sort();
 function covered(){if(state.mode==='list'&&!state.rangeSelected)return state.started;const dates=dated(),boundary=state.date||state.month+'-01';return state.complete||(dates.length>0&&dates[0]<boundary);}
 function paint(){
  if(!current())return;
  onView({month:state.rangeSelected||state.mode==='calendar'?state.month:'',date:state.date,mode:state.mode});
  const [year,month]=state.month.split('-').map(Number),first=new Date(Date.UTC(year,month-1,1)),days=new Date(Date.UTC(year,month,0)).getUTCDate(),offset=(first.getUTCDay()+6)%7;
  const readable=state.rows.filter(visible);const counts=new Map();for(const row of readable){const day=dateOf(row);if(day)counts.set(day,(counts.get(day)||0)+1);}
  calendar.innerHTML=`<div class="card stack"><div class="row"><button class="btn ghost" data-month="-1" aria-label="上个月">‹</button><b class="grow">${esc(state.month)}</b><button class="btn ghost" data-month="1" aria-label="下个月">›</button></div><div class="calendar-grid" >${['一','二','三','四','五','六','日'].map(d=>`<span class="calendar-weekday">${d}</span>`).join('')}${'<span></span>'.repeat(offset)}${Array.from({length:days},(_,i)=>{const day=`${state.month}-${String(i+1).padStart(2,'0')}`;return `<button class="calendar-day ${state.date===day?'selected':''}" data-day="${day}" aria-pressed="${state.date===day}" aria-label="${day}${counts.has(day)?'，已有载入记录':''}">${i+1}${counts.has(day)?'<span class="calendar-dot" aria-hidden="true"></span>':''}</button>`;}).join('')}</div><button class="btn ghost" data-reading-mode>${state.mode==='list'?'返回日历':'按月份列表'}</button><form class="stack" data-reading-find>${Field({label:`查找${label}日期`,name:'date',type:'date',value:state.dateDraft,attrs:{required:true}})}<div class="row"><button class="btn">查找</button><button type="button" class="btn ghost" data-reading-clear>清除日期</button></div></form><p class="small muted">本月已载入 ${readable.filter(row=>dateOf(row)?.startsWith(state.month)).length} 条${esc(label)}；共已载入 ${readable.length} 条可阅读内容${state.rows.length>readable.length?`；另有 ${state.rows.length-readable.length} 条生成记录`: ""}。日期标记只代表已载入记录。</p></div>`;
  calendar.querySelector('[name=date]').oninput=e=>state.dateDraft=e.target.value;
  calendar.querySelectorAll('[data-month]').forEach(button=>button.onclick=()=>{const next=new Date(Date.UTC(year,month-1+Number(button.dataset.month),1));state.rangeSelected=true;state.month=next.toISOString().slice(0,7);state.date='';state.dateDraft='';notice='已切换到 '+state.month;paint();void load({seek:true});});
  calendar.querySelectorAll('[data-day]').forEach(button=>button.onclick=()=>selectDate(button.dataset.day));
  calendar.querySelector('[data-reading-find]').onsubmit=e=>{e.preventDefault();if(e.target.reportValidity())selectDate(e.target.elements.date.value);};
  calendar.querySelector('[data-reading-clear]').onclick=()=>{state.date='';state.dateDraft='';state.rangeSelected=false;state.mode='list';notice='';paint();};
  calendar.querySelector('[data-reading-mode]').onclick=()=>{state.mode=state.mode==='list'?'calendar':'list';state.rangeSelected=state.mode==='calendar';paint();};
  const rows=state.rows.filter(visible).filter(row=>{const date=dateOf(row);return state.mode==='list'&&!state.rangeSelected||!date||date.startsWith(state.date||state.month);});
  const empty=!state.started?'<p class="muted reading-empty">正在读取…</p>':emptyText&&state.complete&&!filtered&&state.mode==='list'&&!state.rangeSelected?`<p class="muted reading-empty">${esc(emptyText)}</p>`:`<div class="statebox"><h3>${!covered()?'尚未读到这个日期范围':state.mode==='list'&&!state.rangeSelected?'当前筛选没有已载入的'+esc(label):(state.date?'这一天':'本月')+'没有已载入的'+esc(label)}</h3><p>${covered()?'可以选择其它日期或调整筛选。':'继续读取更早记录后才能确认，当前内容已保留。'}</p></div>`;
  const hidden=renderHidden?state.rows.filter(row=>!visible(row)).filter(row=>!state.rangeSelected||String(dateOf(row)||'').startsWith(state.date||state.month)).map(renderHidden).join(''):'';
  const paperHtml=(rows.map(row=>render(row,dateOf(row))).join('')||empty)+(hidden?`<section class="diary-generation-records stack"><h3>日记生成记录</h3>${hidden}</section>`:'');
  if(papers.innerHTML!==paperHtml)papers.innerHTML=paperHtml;
  status.innerHTML=error?`<p class="banner err">读取失败：${esc(error)}，已载入内容保留。</p>`:busy&&!state.started?'<p class="small muted">正在读取所选范围…</p>':notice?`<p class="small muted">${esc(notice)}</p>`:!covered()?'<p class="small muted">这个日期范围尚未完整读取。</p>':'';
  el.querySelector('[data-reading-more]').hidden=state.complete;
  el.querySelector('[data-reading-more]').disabled=busy;
  el.querySelector('[data-reading-refresh]').disabled=busy;
 }
 function selectDate(date){if(!validDate(date))return;state.rangeSelected=true;state.date=date;state.dateDraft=date;state.month=date.slice(0,7);notice='已查找 '+date;paint();if(!covered())void load({seek:true});}
 async function load({seek=false,reset=false}={}){
  if(!current()||busy)return;busy=true;error='';notice='';const version=++epoch;paint();
  try{
   let reads=0,cursor=reset?'':state.cursor;
   do{const page=await read(cursor);if(!current()||version!==epoch)return;
    if(reset&&reads===0)state.rows=[];
    const merged=new Map(state.rows.map(row=>[row.id,row]));for(const row of page.items)merged.set(row.id,row);state.rows=[...merged.values()];
    state.cursor=page.next_cursor||'';state.complete=!state.cursor;state.started=true;cursor=state.cursor;reads++;
   }while(seek&&!covered()&&!state.complete&&reads<3);
  }catch(failure){if(current()&&version===epoch)error=errorMessage(failure,'暂时无法读取记录');}
  finally{if(current()&&version===epoch){busy=false;if(!error)notice=state.mode==='list'&&!state.rangeSelected?'':reset?'已重新读取所选范围':seek?'已更新所选日期范围':'已载入更多记录';paint();}}
 }
 el.querySelector('[data-reading-more]').onclick=()=>{state.following=false;clearTimeout(timer);void load({seek:true});};
 el.querySelector('[data-reading-refresh]').onclick=()=>{state.following=true;void load({reset:true,seek:true});};
 const cleanup=bindToolbar?.(el.querySelector('[data-toolbar]'));
 const follow=()=>{clearTimeout(timer);if(!current()||!globalThis.document||document.hidden||state.following===false||state.mode!=='list'||state.rangeSelected)return;timer=setTimeout(async()=>{const editing=el.contains?.(document.activeElement)&&['INPUT','TEXTAREA','SELECT'].includes(document.activeElement?.tagName);if(!editing)await load({reset:true});follow();},30000);};
 const visibility=()=>{if(document.hidden)clearTimeout(timer);else follow();};globalThis.document?.addEventListener('visibilitychange',visibility);
 paint();if(!state.started)void load({seek:true});else if(state.following!==false&&!state.rangeSelected)void load({reset:true});follow();
 return()=>{active=false;epoch++;clearTimeout(timer);globalThis.document?.removeEventListener('visibilitychange',visibility);cleanup?.();};
}

// Detail reads reconcile already loaded records without moving calendar anchors.
export function reconcileReadingRecord(fid,kind,row,matches=()=>true){
 for(const state of views.values())if(state.scope?.[0]===fid&&state.scope?.[1]===kind&&state.rows.some(old=>old.id===row.id)){
  state.rows=state.rows.flatMap(old=>old.id!==row.id?[old]:matches(state.scope,row)?[row]:[]);
 }
}
