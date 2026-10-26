import {errorMessage} from '../v3/domain-presentation.mjs';
import { randomId } from '../random-id.mjs';
import * as store from '../v3/proto/store.js';
import { readBackground, saveBackground } from '../v3/providers/real.ts';
import { backgroundDraft, backgroundFromDraft } from '../lib/actor-user-background.ts';
import { esc, requireOwner, drawer, confirm } from '../v3/proto/ui.js';
import { Field } from '../v3/proto/components/forms.js';
import { JsonDetails } from '../v3/proto/components/records.js';
import { SemanticFields } from '../v3/proto/components/semantic-fields.js';

const drafts=new Map();
const fields=[['full_name','姓名'],['name','称呼'],['age','年龄'],['gender','性别'],['occupation','职业/身份'],['relationship_description','关系'],['wants_you_to_know','你想让她知道的事']];
const supplementary=['background','interests','preferences','personality','habits','shared_information','rules'];
const supplementLabels={background:'背景',interests:'喜好',preferences:'交流偏好',personality:'性格',habits:'习惯',shared_information:'补充信息',rules:'边界'};
function project(background){
  const value=backgroundDraft(background);
  value.wants_you_to_know=[value.wants_you_to_know,...supplementary.filter(key=>value[key]).map(key=>supplementLabels[key]+'：'+value[key])].filter(Boolean).join('\n');
  return value;
}
export function renderRealAboutYou(el,ctx){
  let alive=true,saved,busy=false,epoch=0;
  const ownerId=store.get('session')?.actorId,actorId=ctx.query.actor||ownerId;
  const scope=JSON.stringify([ownerId,ctx.charId,actorId]);let draft=drafts.get(scope);
  const character=store.get('characters',ctx.charId);
  const fresh=()=>({fields:project(saved.background),reason:'',operation:'correct',dirty:false,editing:false,conflict:false,version:saved.context_version,factsRevision:saved.factsRevision,key:randomId(),consolidated:false});
  async function load(keep=true,rebase=false){
    const request=++epoch;busy=true;
    if(!saved)el.innerHTML='<div class="statebox" role="status">正在读取关于你的资料…</div>';
    try{const value=await readBackground(ctx.charId,actorId);if(!alive||request!==epoch)return;saved=value;
      if(!draft||!keep){draft=fresh();drafts.set(scope,draft);}
      else if(rebase){draft.version=value.context_version;draft.factsRevision=value.factsRevision;draft.conflict=false;draft.key=randomId();}
      paint();return true;
    }catch(error){if(alive&&request===epoch){if(saved){paint();el.querySelector('[data-result]').textContent=errorMessage(error);}else{el.innerHTML=`<div class="statebox" role="alert"><h2>资料读取失败</h2><p>${esc(errorMessage(error))}</p><button data-domain-ui="button" class="btn" data-retry>重新读取</button></div>`;el.querySelector('[data-retry]').onclick=()=>void load();}}return false;}
    finally{if(request===epoch)busy=false;}
  }
  function references(rows){
    return rows.map(row=>`<article class="reading-record">${SemanticFields(row,{empty:'这条记录尚无可读内容。'})}${JsonDetails(row,'来源与原记录')}</article>`).join('');
  }
  function inspect(kind){
    const relationship=kind==='relationships',rows=relationship?saved.relationships:saved.facts;
    drawer({title:relationship?'她视角下的关系与交往依据':'她对你的了解',body:`<p class="small muted">${relationship?'这是当前人格视角下的已有关系，与你自填的关系分开。':'查阅她已经记录的认识，不会把自填资料直接写成她的了解。'}</p><div class="stack">${rows?.length?references(rows):`<p>${relationship?'当前范围尚无关系记录。':'当前范围尚无了解记录。'}</p>`}</div>`});
  }
  function paint(){
    const editing=draft.editing;
    const conflict=draft.conflict?'<div class="banner warn" role="alert">资料已经变化，你的修改仍保留。<button data-domain-ui="button" type="button" class="btn" data-rebase>读取最新资料并保留我的修改</button></div>':'';
    const controls=fields.map(([key,label])=>{
      if(!editing)return `<div class="about-you-value ${key==='wants_you_to_know'?'wide':''}"><dt>${label}</dt><dd>${esc(draft.fields[key]||'未填写')}</dd></div>`;
      const genderOptions=[{value:'',label:'选择性别'},{value:'男',label:'男'},{value:'女',label:'女'},{value:'非二元性别',label:'非二元性别'},{value:'不愿透露',label:'不愿透露'}];
      if(draft.fields.gender&&!genderOptions.some(option=>option.value===draft.fields.gender))genderOptions.push({value:draft.fields.gender,label:draft.fields.gender});
      return `<div class="${key==='wants_you_to_know'?'wide':''}">${Field({label,name:key,value:draft.fields[key],type:key==='wants_you_to_know'?'textarea':key==='age'?'number':key==='gender'?'select':'text',rows:6,options:genderOptions,attrs:{'data-field':key,maxlength:1024,...(key==='age'?{min:0,step:1}:{}),placeholder:key==='age'?'填写年龄':key==='wants_you_to_know'?'你愿意分享的喜好、经历、交流习惯与边界':''}})}</div>`;
    }).join('');
    el.innerHTML=`<div class="about-you-workspace"><section class="reading-paper about-you-paper"><div class="row"><div class="grow"><h2 class="media-business-title">关于你</h2><p class="small muted">${esc(character?.name||'她')}会将这些作为你填写的资料。只填你愿意分享的。</p></div>${editing?'':'<button data-domain-ui="button" class="btn" data-edit>编辑资料</button>'}</div>${conflict}<form data-about-form><${editing?'div':'dl'} class="about-you-fields">${controls}</${editing?'div':'dl'}>${editing?`<details data-domain-ui="disclosure" class="mgmt" data-save-details><summary>本次修订</summary><div class="stack">${Field({label:'修改含义',name:'operation',value:draft.operation,type:'select',options:[{value:'correct',label:'之前写错了'},{value:'change',label:'现在情况变了'}]})}${Field({label:'修改说明',name:'reason',value:draft.reason,hint:'说明本次变化，保留原资料的修订依据。',attrs:{maxlength:saved.owner?500:1000}})}</div></details><div class="row about-you-actions"><button data-domain-ui="button" type="button" class="btn" data-cancel>取消编辑</button><button data-domain-ui="button" class="btn primary" data-test="owner-save">保存资料</button><span class="small muted" data-draft-state>${draft.dirty?'有未保存修改':''}</span></div>`:''}<p class="small muted" role="status" data-result></p></form></section><aside class="stack"><section class="card"><h3>她对你的了解</h3><p class="small muted">你填写的资料与她已有的认识分开保存。</p><button data-domain-ui="button" class="btn" data-understanding>查看了解记录</button></section><section class="card"><h3>她视角下的关系</h3><p class="small muted">你填写的关系描述不会直接改变她的关系记录。</p><button data-domain-ui="button" class="btn" data-relationship>查看关系与交往依据</button></section><section class="card"><h3>来源与修订历史</h3><p class="small muted">你填写的资料与关系记录分别保留来源。</p>${(saved.history||[]).length?references(saved.history):'<p class="small muted">尚无资料修订记录。</p>'}${(saved.relationship_history||[]).length?references(saved.relationship_history):'<p class="small muted">尚无关系修订记录。</p>'}${JsonDetails({history:saved.history,relationshipHistory:saved.relationship_history,background:saved.background},'完整来源与技术详情')}</section></aside></div>`;
    el.querySelector('[data-edit]')?.addEventListener('click',()=>{draft.editing=true;paint();el.querySelector('[data-field]')?.focus();});
    el.querySelector('[data-understanding]').onclick=()=>inspect('facts');el.querySelector('[data-relationship]').onclick=()=>inspect('relationships');
    el.querySelector('[data-cancel]')?.addEventListener('click',async()=>{if(busy)return;if(draft.dirty&&!(await confirm({title:'放弃这次资料修改？',text:'未保存的修改将被放弃，已经保存的资料保持不变。',okText:'放弃修改'})))return;if(!alive||busy)return;draft=fresh();drafts.set(scope,draft);paint();el.querySelector('[data-edit]')?.focus();});
    el.querySelector('[data-rebase]')?.addEventListener('click',()=>{if(!busy)void load(true,true);});
    el.querySelectorAll('[data-field]').forEach(input=>input.oninput=()=>{input.setCustomValidity('');draft.fields[input.dataset.field]=input.value;if(input.dataset.field==='wants_you_to_know')draft.consolidated=true;draft.key=randomId();draft.dirty=true;el.querySelector('[data-draft-state]').textContent='有未保存修改';});
    for(const name of ['operation','reason'])el.querySelector(`[name=${name}]`)?.addEventListener('input',event=>{event.target.setCustomValidity('');draft[name]=event.target.value;draft.key=randomId();draft.dirty=true;});
    el.querySelector('form').onsubmit=async event=>{
      event.preventDefault();if(!editing||busy||!(await requireOwner()))return;
      const result=el.querySelector('[data-result]');
      const rejectField=(name,message)=>{const input=el.querySelector(`[name=${name}]`);if(name==='reason')el.querySelector('[data-save-details]').open=true;input?.setCustomValidity(message);input?.reportValidity();input?.focus();result.textContent=message;};
      if(!event.target.reportValidity())return;
      if(!draft.reason.trim()){rejectField('reason','请填写本次修改说明');return;}
      if(saved.owner?[...draft.reason.trim()].length>500:new TextEncoder().encode(draft.reason.trim()).length>1000){rejectField('reason',saved.owner?'修改说明最多500字':'修改说明最多1000 UTF-8字节');return;}
      const tooLong=fields.find(([key])=>[...draft.fields[key].trim()].length>1024);if(tooLong){rejectField(tooLong[0],'每项资料最多1024字，输入已保留。');return;}
      if(draft.fields.age&&!/^\d+$/.test(draft.fields.age)){rejectField('age','年龄请填写非负整数。');return;}
      // Keep unexposed context fields; consolidate authored supplementary text
      // only when this single visible field was explicitly edited.
      const payload={...draft.fields};if(draft.consolidated)for(const key of supplementary)payload[key]='';
      else payload.wants_you_to_know=saved.background.wants_you_to_know||'';
      busy=true;el.querySelectorAll('input,textarea,select,form button').forEach(control=>control.disabled=true);
      try{await saveBackground(ctx.charId,actorId,{background:backgroundFromDraft(payload),operation:draft.operation,reason:draft.reason.trim(),expectedContextVersion:draft.version,idempotencyKey:draft.key},draft.factsRevision);const readBack=await load(false);if(alive)el.querySelector('[data-result]').textContent=readBack?'资料已保存，并已读取最新值。':'保存已提交，最新资料尚未读到，请重新读取后核对。';}
      catch(error){if(alive){if(error.status===409){draft.conflict=true;paint();}else result.textContent=errorMessage(error);}}
      finally{busy=false;if(alive)el.querySelectorAll('input,textarea,select,form button').forEach(control=>control.disabled=false);}
    };
  }
  void load();return()=>{alive=false;epoch++;};
}
