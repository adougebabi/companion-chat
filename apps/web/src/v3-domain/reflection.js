import {errorMessage} from '../v3/domain-presentation.mjs';
import {ReflectionProgress} from '../v3/proto/components/reflection-progress.js';
import {reflectionClient} from '../v3/proto/services.js';
import {asyncPage,instant} from '../v3/proto/components/async-page.js';
import {CursorPager,bindCursorPager} from '../v3/proto/components/cursor-pager.js';
import {esc,badge} from '../v3/proto/ui.js';

const growthNavigation=(fid,current)=>`<nav class="reading-module-nav" aria-label="记忆成长">${[['memory','记忆',`#/c/${fid}/growth`],['reflection','反思',`#/c/${fid}/growth/reflection`],['self','成长认识',`#/c/${fid}/growth/self`],['snapshot','人格快照',`#/c/${fid}/growth/self?view=snapshot`]].map(([key,label,href])=>`<a href="${href}" ${current===key?'aria-current="page"':''}>${label}</a>`).join('')}</nav>`;

const href=(fid,id)=>`#/c/${encodeURIComponent(fid)}/growth/reflection/${encodeURIComponent(id)}`;
const stateLabel={applied:'已应用',no_change:'无变化',proposed:'历史提案'};

export function renderRealReflections(el,ctx){
 const cursors=[''];let at=0;
 return asyncPage(el,()=>reflectionClient.list(ctx.charId,cursors[at]),(initial,reload,alive)=>{
  let page=initial,timer,reads=0,busy=false,failure='';const taskId=page.latest_task?.intent_id;
  el.innerHTML=`<div class="stack reading-workspace"><h1 class="reading-business-title">反思与成长</h1>${growthNavigation(ctx.charId,'reflection')}<section class="stack" data-reflection-results></section><details data-domain-ui="disclosure" class="card" data-progress><summary>这次反思的处理进度</summary><div data-reflection-progress></div></details><div data-reflection-pager></div><p data-domain-ui="alert" data-observe-error role="status"></p><div class="row"><button data-domain-ui="button" data-domain-variant="ghost" data-domain-size="default" data-reload>重新读取</button><a data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" href="#/c/${ctx.charId}/growth/self">查看成长认识</a><a data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" href="#/c/${ctx.charId}/growth">她的记忆</a></div></div>`;
  function paint(){if(!alive())return;
   el.querySelector('[data-reflection-results]').innerHTML=page.items.map(r=>`<a data-domain-ui="card" class="card reflection-result" href="${href(ctx.charId,r.id)}"><div class="row"><h3 class="grow">${esc(r.title)}</h3>${badge(stateLabel[r.status]||r.status,'plain')}</div><p>${esc(r.summary??'这条旧记录没有摘要。')}</p><span class="small muted">${esc(instant(r.at))}</span></a>`).join('')||'<div data-domain-ui="empty" class="statebox"><h3>还没有反思结果</h3><p>她会结合新的交流和经历反思；一次反思也可能没有新的变化。</p></div>';
   const progress=el.querySelector('[data-reflection-progress]');
   progress.innerHTML=ReflectionProgress(page);el.querySelector('[data-progress]').open ||= !!page.latest_task?.error_code||['failed','dead_letter','superseded'].includes(page.latest_task?.status);
   el.querySelector('[data-reflection-pager]').innerHTML=CursorPager({more:!!page.next_cursor,previous:at>0,label:'反思'});
   bindCursorPager(el.querySelector('[data-reflection-pager]'),{next:()=>{cursors[++at]=page.next_cursor;void reload();},previous:()=>{at--;void reload();}});
   el.querySelector('[data-observe-error]').textContent=failure;
  }
  const pending=()=>['pending','queued','dispatched','running','retry','retry_wait'].includes(page.latest_task?.status)||page.window?.status==='running';
  async function query(){if(!alive()||document.hidden||busy||reads>=60||!pending())return;busy=true;try{const next=await reflectionClient.list(ctx.charId,cursors[at]);if(!alive())return;if(taskId&&next.latest_task?.intent_id!==taskId){failure='已有处理已结束或有新的反思任务，可重新读取。';reads=60;}else{page=next;failure='';}paint();}catch(error){if(alive()){failure='更新失败，已读取的反思保留：'+errorMessage(error);paint();}}finally{busy=false;reads++;schedule();}}
  function schedule(){clearTimeout(timer);if(alive()&&!document.hidden&&pending()&&reads<60)timer=setTimeout(query,3000);}
  const visibility=()=>{clearTimeout(timer);if(!document.hidden)void query();};document.addEventListener('visibilitychange',visibility);
  el.querySelector('[data-reload]').onclick=reload;paint();schedule();
  return()=>{clearTimeout(timer);document.removeEventListener('visibilitychange',visibility);};
 },{backHref:`#/c/${encodeURIComponent(ctx.charId)}/growth`,backLabel:'返回记忆成长'});
}
