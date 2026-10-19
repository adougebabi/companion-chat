import {mockClient} from '../services.js';
import {esc,badge} from '../ui.js';
import {mountCursorRecords} from './cursor-records.js';
import {JsonDetails} from './records.js';
import {instant} from './async-page.js';
import {governanceCommand} from './governance-command.js';
import {Tabs} from './forms.js';

const titles={encounters:'接触记录',opportunities:'相遇机会',presences:'在场执行'};
const states={pending:'等待判断',deferred:'暂缓',contacted:'已交流',declined:'接触未被接受',passed:'本次未接触',expired:'在场来源已失效',failed:'处理失败',active:'有效在场',ended:'已结束',cancelled:'已终止'};
const sources={owner_confirmed_encounter:'Owner确认的实际接触',virtual_world_encounter:'虚拟世界实际相遇',npc_routine_execution:'生活规律执行'};
function NPCRecord(row,zone,readOnly){
 const status=states[row.status]||row.status||'已记录',result=row.result;
 return `<div class="card"><div class="row"><b class="grow">${esc(row.summary||titles[row.record_kind])}</b>${badge(status)}</div><p class="small muted">${esc(instant(row.occurred_at||row.start_at||row.created_at,zone))}${row.source_kind?' · '+esc(sources[row.source_kind]||row.source_kind):''}</p>${result?.reason?`<p>${esc(result.reason)}</p>`:''}${row.error_code?`<p class="banner warn">${esc(row.error_code)}</p>`:''}${row.record_kind==='opportunities'&&!readOnly&&['failed','deferred'].includes(row.status)?`<button class="btn sm" data-opportunity-retry="${esc(row.id)}">重新判断这次机会</button>`:''}${row.task_id?`<p><a href="#/diag/task/${encodeURIComponent(row.task_id)}">实际认知任务</a></p>`:''}${JsonDetails(row,'实际来源、参与者与结果')}</div>`;
}
export function mountNPCRuntimeRecords(box,{fid,npcId,zone,alive,readOnly=false,initial='encounters',onTab=()=>{}}){
 let tab=Object.hasOwn(titles,initial)?initial:'encounters',panel;
 box.innerHTML=`${Tabs({items:Object.entries(titles).map(([value,label])=>({value,label,attrs:{role:"tab"}})),value:tab,attribute:"record-tab",attrs:{role:"tablist","aria-label":"人物运行记录"}})}<div data-record-panel role="tabpanel"></div>`;
 function paint(){
  panel?.dispose();box.querySelectorAll('[data-record-tab]').forEach(button=>{button.classList.toggle('on',button.dataset.recordTab===tab);button.setAttribute('aria-selected',String(button.dataset.recordTab===tab));});
  const empty={encounters:'尚无实际接触记录。创建定义、应用或仅同处地点不会自动结识。',opportunities:'尚未产生同地相遇机会。需要角色有效在场和NPC实际运行在场（事件或进行中的活动）；浏览页面不会创建机会。',presences:'尚无实际在场执行记录。可配置生活规律并启用人物，等待运行世界执行；预测地点仍单独展示。'};
  panel=mountCursorRecords(box.querySelector('[data-record-panel]'),{alive,label:titles[tab],read:cursor=>mockClient.npcRecords(fid,npcId,tab,cursor),render:row=>NPCRecord(row,zone,readOnly),empty:empty[tab],bind:(list,page)=>list.querySelectorAll('[data-opportunity-retry]').forEach(button=>button.onclick=()=>{
   const record=page.items.find(row=>row.id===button.dataset.opportunityRetry);
   governanceCommand({scope:[fid,npcId,record.id,'retry-opportunity'],title:'重新判断相遇机会',revision:record.revision,maxReason:500,note:'只重新提交这条尚未结算的机会；角色与NPC仍可不接受接触。会重新核对原世界、地点、人格与双方在场，失效来源不能恢复成相遇。',snapshot:record,read:()=>mockClient.npcRecord(fid,npcId,record.id),execute:body=>mockClient.retryNPCOpportunity(fid,npcId,record.id,body),successMessage:'机会已重新排入判断，实际结果待查询',onSaved:()=>{if(alive())return panel.reload();}});
  })});
 }
 box.querySelectorAll('[data-record-tab]').forEach(button=>button.onclick=()=>{tab=button.dataset.recordTab;onTab(tab);paint();});paint();
 return {dispose(){panel?.dispose();}};
}
