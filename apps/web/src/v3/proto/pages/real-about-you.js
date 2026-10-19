import * as store from '../store.js';
import { readBackground, saveBackground, client } from '../../providers/real.ts';
import { actorUserBackgroundFields, backgroundDraft, backgroundFromDraft } from '../../../lib/actor-user-background.ts';
import { esc, requireOwner } from '../ui.js';
import { Field, Tabs } from '../components/forms.js';
import { AboutYouCard } from '../components/about-you-card.js';
const drafts = new Map();
const tabs = {
 basic:['name','location','occupation','interests','shared_information'],
 relationship:['relationship_description','relationship_distance','personality','habits','wants_you_to_know'],
 rules:['rules'],
};
const extraFields={basic:['full_name','age','gender','background','preferences','location_scope','timezone'],relationship:['meeting_confirmed'],rules:[]};
const labels={location:'城市',interests:'喜好',relationship_distance:'双方距离与交流方式'};

export function renderRealAboutYou(el,ctx) {
  let alive=true,saved,busy=false;
  const ownerId=store.get('session')?.actorId, actorId=ctx.query.actor||ownerId, key=`${ownerId}:${ctx.charId}:${actorId}`, tab=Object.hasOwn(tabs,ctx.query.tab)?ctx.query.tab:'basic';
  let draft=drafts.get(key), participants=[{id:ownerId,name:'你'}];
  el.innerHTML='<div class="card" role="status">正在读取交流参与者资料…</div>';
  async function load(keep=false, rebase=false){
    busy=true;
    try{saved=await readBackground(ctx.charId,actorId);if(!alive)return;
      if(!keep||!draft){draft={fields:backgroundDraft(saved.background),reason:'',operation:'correct',dirty:false,conflict:false,version:saved.context_version,factsRevision:saved.factsRevision,key:crypto.randomUUID()};drafts.set(key,draft);}
      else if(rebase) {draft.conflict=false;draft.version=saved.context_version;draft.factsRevision=saved.factsRevision;draft.key=crypto.randomUUID();}
      paint();
    }catch(error){if(alive){el.innerHTML=`<div class="banner err" role="alert">${esc(error.message)}</div><button class="btn" data-retry>重新读取</button>`;el.querySelector('[data-retry]').onclick=()=>load(keep);}}
    finally{busy=false;}
  }
  function paint(){
    const character=store.get('characters',ctx.charId);
    const fieldMarkup=(key,extra=false)=>{
      const field=actorUserBackgroundFields.find(item=>item.key===key);
      const multiline=key==='rules'||(tab!=='basic'||extra)&&['background','preferences','personality','habits','wants_you_to_know'].includes(key);
      return Field({label:labels[key]||field.label,name:key,value:draft.fields[key],type:key==='meeting_confirmed'?'select':multiline?'textarea':'text',rows:key==='rules'?6:3,options:key==='meeting_confirmed'?[{value:'unknown',label:'未确认'},{value:'true',label:'已约定'},{value:'false',label:'未约定'}]:undefined,attrs:{'data-field':key,maxlength:1024}});
    };
    const conflict=draft.conflict?'<div class="banner warn"><p>资料版本已变化。草稿已保留，请读取新版本并核对后再保存。</p><button type="button" class="btn" data-refresh>读取新版本并保留草稿</button></div>':'';
    const extra=extraFields[tab].length?`<details class="mgmt small muted"><summary>${tab==='basic'?'更多基本资料':'更多交流资料'}</summary><div class="stack" style="margin-top:8px">${extraFields[tab].map(key=>fieldMarkup(key,true)).join('')}</div></details>`:'';
    const saveDetails=`<details class="mgmt small muted" data-save-details><summary>保存说明</summary><div class="stack" style="margin-top:8px">${Field({label:'修改含义',name:'operation',value:draft.operation,type:'select',options:[{value:'correct',label:'纠正原信息'},{value:'change',label:'情况变化'}]})}${Field({label:'修改说明',name:'reason',value:draft.reason,attrs:{maxlength:saved.owner?500:1000},hint:'保存前请说明本次资料变化。'})}${draft.conflict?'':'<button type="button" class="btn" data-refresh>读取新版本并保留草稿</button>'}</div></details>`;
    el.innerHTML=`${Tabs({value:tab,items:[['basic','基本资料'],['relationship','关系与自述'],['rules','给她的规则']].map(([value,label])=>({value,label,href:`#/c/${ctx.charId}/people/owner?actor=${encodeURIComponent(actorId)}&tab=${value}`}))})}<div class="row" style="margin-bottom:12px">${Field({label:'交流参与者',name:'actor',value:actorId,type:'select',options:participants.map(row=>({value:row.id,label:row.name})),attrs:{'data-actor':true}})}<span class="small muted">${esc(character?.name)}的交流范围</span></div>${AboutYouCard({subtitle:`${character?.name}会把这些作为你填写的资料。只填你愿意让她知道的。`,fields:tabs[tab].map(key=>fieldMarkup(key)).join(''),advanced:extra+saveDetails,conflict,actions:`<button type="button" class="btn" data-discard>撤销更改</button><button class="btn accent" data-test="owner-save">保存</button><span class="small muted" data-draft-state>${draft.dirty?'有未保存修改':''}</span>`,result:'<p class="small muted" data-result role="alert"></p>'})}<details class="mgmt small muted" style="max-width:640px"><summary>来源与修订历史</summary><p>本人填写的资料与她对你的印象分开保存。留空表示未知。</p><pre>${esc(JSON.stringify({contextVersion:saved.context_version,currentFactsRevision:saved.factsRevision,facts:saved.facts,history:saved.history,relationships:saved.relationships,relationshipHistory:saved.relationship_history},null,2))}</pre></details>`;
    el.querySelectorAll('[data-field]').forEach(input=>input.oninput=()=>{draft.fields[input.dataset.field]=input.value;draft.key=crypto.randomUUID();draft.dirty=true;el.querySelector('[data-draft-state]').textContent='有未保存修改';});
    for(const name of ['operation','reason'])el.querySelector(`[name=${name}]`).oninput=event=>{draft[name]=event.target.value;draft.key=crypto.randomUUID();draft.dirty=true;el.querySelector('[data-draft-state]').textContent='有未保存修改';};
    el.querySelector('[data-actor]').onchange=event=>ctx.navigate(`#/c/${ctx.charId}/people/owner?actor=${encodeURIComponent(event.target.value)}&tab=${tab}`);
    el.querySelector('[data-discard]').onclick=()=>{if(!busy)void load(false);};
    el.querySelector('[data-refresh]').onclick=()=>{if(!busy)void load(true,true);};
    el.querySelector('form').onsubmit=async event=>{
      event.preventDefault();if(busy||!(await requireOwner()))return;
      if(!draft.reason.trim()){el.querySelector('[data-save-details]').open=true;el.querySelector('[name=reason]').focus();el.querySelector('[data-result]').textContent='请填写修改说明';return;}
      if(saved.owner ? [...draft.reason.trim()].length>500 : new TextEncoder().encode(draft.reason.trim()).length>1000){el.querySelector('[data-result]').textContent=saved.owner?'修改说明最多 500 字':'该参与者的修改说明最多 1000 UTF-8 字节';return;}
      if(Object.entries(draft.fields).some(([key,value])=>key!=='meeting_confirmed'&&[...value.trim()].length>1024)){el.querySelector('[data-result]').textContent='每个资料字段最多 1024 字';return;}
      busy=true;el.querySelectorAll('form button').forEach(button=>button.disabled=true);
      try{await saveBackground(ctx.charId,actorId,{background:backgroundFromDraft(draft.fields),operation:draft.operation,reason:draft.reason.trim(),expectedContextVersion:draft.version,idempotencyKey:draft.key},draft.factsRevision);await load(false);if(alive)el.querySelector('[data-result]').textContent='资料已保存并读取最新状态。';}
      catch(error){if(alive){if(error.status===409){draft.conflict=true;paint();}else el.querySelector('[data-result]').textContent=error.message;}}
      finally{busy=false;if(alive)el.querySelectorAll('form button').forEach(button=>button.disabled=false);}
    };
  }
  void client.directConversation(ctx.charId).then(page=>{participants=page.participants.filter(row=>row.actorId===ownerId||row.role==='human').map(row=>({id:row.actorId,name:row.actorId===ownerId?'你':row.displayName||row.actorId}));if(!participants.some(row=>row.id===ownerId))participants.unshift({id:ownerId,name:'你'});return load(Boolean(draft));}).catch(error=>{if(alive)el.innerHTML=`<div class="banner err">${esc(error.message)}</div>`;});
  return()=>{alive=false;};
}
