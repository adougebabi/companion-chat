import {lifeClient,diaryClient,momentsClient,mediaClient} from '../v3/proto/services.js';
import {get,subscribe} from '../v3/proto/store.js';
import {esc} from '../v3/proto/ui.js';
import {instant} from '../v3/proto/components/async-page.js';
import {lifeTimelineRows} from '../v3/life-timeline.mjs';
const readings=new Map();subscribe(()=>{if(!get('session'))readings.clear();});
export function renderLifeTimeline(el,ctx){
 const fid=ctx.charId,owner=get('session')?.actorId,key=JSON.stringify([owner,fid]);
 if(!readings.has(key))readings.set(key,{sources:{},cursors:{},errors:{},loaded:new Set(),follow:true});
 const state=readings.get(key);let active=true,filter='',busy=false,timer=null,epoch=0,rendered='';
 const alive=()=>active&&el.isConnected&&get('session')?.actorId===owner;
 const base='#/c/'+encodeURIComponent(fid);
 const readers={schedule:()=>lifeClient.schedules(fid).then(v=>({items:v.days})),diary:cursor=>diaryClient.list(fid,cursor,false),moments:cursor=>momentsClient.list({fluctlightId:fid,cursor}),media:cursor=>mediaClient.list({fluctlightId:fid,scope:'fluctlight',kind:'image',hidden:false,cursor}),goals:cursor=>lifeClient.goals(fid,cursor)};
 el.innerHTML=`<div class="life-journal"><header class="reading-page-heading"><h1 class="reading-business-title">她的生活</h1><p class="muted">她的日常、心事和分享，都在这里。</p><nav class="row" aria-label="生活阅读"><a href="${base}/life/schedule">日历</a><a href="${base}/life/diary">日记</a><a href="#/feed?fid=${encodeURIComponent(fid)}">动态与分享</a><a href="${base}/media">相册</a><a href="${base}/life/goals">她想做的事</a></nav></header><div data-domain-ui="toggle-group" class="row life-journal-filters" role="group" aria-label="生活内容">${[['','全部'],['diary','日记'],['moments','动态'],['media','图片'],['schedule','安排']].map(([value,label])=>`<button data-domain-ui="button" class="chip ${value?'':'on'}" data-life-filter="${value}" aria-pressed="${!value}">${label}</button>`).join('')}</div><p role="status" class="small muted" data-life-status></p><div data-life-items></div><button data-domain-ui="button" class="btn ghost" data-life-more>更早的生活</button><details data-domain-ui="disclosure" class="reading-secondary"><summary>阅读选项</summary><button data-domain-ui="button" class="btn ghost" data-life-refresh>回到最新近况</button><a href="${base}/life/now">查看此刻</a><a href="#/diag">服务状态</a></details></div>`;
 function paint(){if(!alive())return;const rows=lifeTimelineRows(fid,state.sources).filter(row=>!filter||row.kind===filter);
  const html=rows.map(row=>`<article class="life-journal-entry"><p class="small muted">${esc(row.label)}${row.at?' · '+esc(row.localDay?row.at:instant(row.at)):''}</p><h3><a href="${esc(row.href)}">${esc(row.title)}</a></h3>${row.text?`<p class="reading-prose">${esc(row.text)}</p>`:''}${row.original?`<details data-domain-ui="disclosure"><summary>阅读版已略去尾部异常字符 · 查看原文</summary><p class="reading-prose">${esc(row.original)}</p></details>`:''}${row.imageId?`<a href="${esc(row.href)}"><img class="life-journal-image" src="${esc(mediaClient.url(row.imageId))}" alt="${esc(row.title)}" loading="lazy"></a>`:''}</article>`).join('')||`<div data-domain-ui="empty" class="statebox"><p>${busy&&!state.loaded.size?'正在读她的近况…':Object.keys(state.errors).length&&!Object.keys(state.sources).length?'近况暂时还没读到，稍后再来看看。':'她的生活还在慢慢展开。先和她聊聊吧。'}</p><a data-domain-ui="button" class="btn" href="${base}/chat">和她聊聊</a></div>`;
  if(html!==rendered){el.querySelector('[data-life-items]').innerHTML=html;rendered=html;}
  el.querySelector('[data-life-status]').textContent=Object.keys(state.errors).length?'有些近况暂时没读到，已读内容保留。':state.follow===false?'正在阅读更早的生活。':'';
  el.querySelector('[data-life-more]').hidden=!Object.values(state.cursors).some(Boolean);el.querySelector('[data-life-more]').disabled=busy;
 }
 async function load(more=false,force=false){if(!alive()||busy||document.hidden||!more&&!force&&state.follow===false)return;if(more)state.follow=false;else if(force)state.follow=true;busy=true;const version=++epoch;paint();
  await Promise.allSettled(Object.entries(readers).filter(([kind])=>!more||state.cursors[kind]).map(async([kind,read])=>{try{const page=await read(more?state.cursors[kind]:'');if(!alive()||version!==epoch)return;const merged=new Map((more?state.sources[kind]||[]:[]).map(row=>[row.id||row.local_date,row]));for(const row of page.items||[])merged.set(row.id||row.local_date,row);state.sources[kind]=[...merged.values()];state.cursors[kind]=page.next_cursor||'';state.loaded.add(kind);delete state.errors[kind];}catch{if(alive()&&version===epoch)state.errors[kind]=true;}paint();}));
  if(!alive()||version!==epoch)return;busy=false;paint();clearTimeout(timer);if(state.follow!==false)timer=setTimeout(()=>void load(),30000);
 }
 el.querySelectorAll('[data-life-filter]').forEach(button=>button.onclick=()=>{filter=button.dataset.lifeFilter;el.querySelectorAll('[data-life-filter]').forEach(b=>{b.classList.toggle('on',b===button);b.setAttribute('aria-pressed',String(b===button));});paint();});
 el.querySelector('[data-life-more]').onclick=()=>void load(true);el.querySelector('[data-life-refresh]').onclick=()=>void load(false,true);
 const visible=()=>{clearTimeout(timer);if(!document.hidden)void load();};document.addEventListener('visibilitychange',visible);paint();void load();
 return()=>{active=false;epoch++;clearTimeout(timer);document.removeEventListener('visibilitychange',visible);};
}
