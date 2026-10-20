import { randomId } from '../../../random-id.mjs';
import * as store from '../store.js';
import {directory,activationFromAnalysis} from '../../providers/real.ts';
import {esc,requireOwner,toast} from '../ui.js';
import {Field} from '../components/forms.js';
import {JsonDetails} from '../components/records.js';
import {mountCursorRecords} from '../components/cursor-records.js';
import {CreationPreview,bindCreationPreview} from '../components/creation-preview.js';
import {studioDraftStorage} from '../components/studio-draft-storage.js';
import {parseCreationAnalysis,creationAnalysisRequest} from '../../creation-json.mjs';
import {creationDraft,checkpointCreation} from '../../creation-draft.mjs';
import {persistentCommand} from '../../providers/persistent-command.mjs';

export function renderRealCreate(el,ctx){
 const actor=store.get('session')?.actorId;let alive=true,busy=false,epoch=0,picker=null,errorText='',search='';
 let groupRows=store.list('groups').filter(g=>g.id!=='ungrouped');
 const groupOptions=value=>[{value:'',label:'未分组'},...(value&&!groupRows.some(g=>g.id===value)?[{value,label:'原选择分组已不可用，请重新选择'}]:[]),...groupRows.map(g=>({value:g.id,label:g.name}))];
 const current=()=>alive&&el.isConnected&&store.get('session')?.actorId===actor;
 if(!actor){el.innerHTML='<div class="statebox"><p>请先登录所有者会话。</p><a href="#/auth">登录</a></div>';return;}
 const recovery=creationDraft({owner:actor,storage:studioDraftStorage()});let draft;
 const command=persistentCommand({key:recovery.commandKey,storage:studioDraftStorage,preAdmissionConflicts:['activation_analysis_stale'],execute:async value=>{
  if(!current())throw new Error('所有者会话已切换，请返回原范围核对创建请求');
  const result=await directory.activate(value.body);
  checkpointCreation(recovery,value,result,()=>store.get('session')?.actorId===actor);
  return result;
 }});
 const chooseLinkedTemplate=()=>{if(ctx.query.template&&ctx.query.version){draft.mode='template';draft.templateId=ctx.query.template;draft.templateVersion=ctx.query.version;draft.templateName=ctx.query.template;}};
 const save=()=>{if(current())recovery.save(draft);};
 const changed=()=>{draft.requestId=randomId();save();};
 const fail=error=>{errorText=typeof error==='string'?error:error.message;if(current()){const box=el.querySelector('[data-result]');if(box)box.textContent=errorText;}};
 function capture(form){
  for(const name of ['name','bio','traits','world','groupId','mode','json','timezone'])if(form.elements[name])draft[name]=form.elements[name].value;
  save();
 }
 function sourceFields(){
  if(draft.mode==='blank')return '<p class="small muted">不预设身份经历、人格或世界；生活内容从真实互动逐步建立。</p>';
  if(draft.mode==='analysis')return Field({label:'同一所有者的分析预览 JSON',name:'json',value:draft.json,type:'textarea',rows:10,hint:'保留原分析来源，不再发起分析。激活仍核验所有者、最新来源与是否已使用。'});
  if(draft.mode==='json')return Field({label:'原始初始化 JSON',name:'json',value:draft.json,type:'textarea',rows:10,hint:'导入结构化定义，经 Core 校验并保存新分析来源；不是恢复运行存档。全部输入最多 60000 UTF-8 字节。'});
  if(draft.mode==='template')return `<p class="small muted">选择已发布的角色模板。新角色获得独立生活；读取该精确发布版本重新初始化。</p>${draft.templateId?`<div class="card"><b>${esc(draft.templateName||draft.templateId)}</b><p>发布版本 ${esc(draft.templateVersion)}</p><button class="btn" type="button" data-template-read>读取所选发布内容</button><div data-template-body></div></div>`:''}<div data-template-search>${Field({label:'搜索已发布角色模板',name:'templateSearch',value:search})}<button class="btn" type="button" data-search-templates>搜索</button></div><div data-template-picker></div><a href="#/studio/edit/new?kind=character_template">创作角色模板</a>`;
  return Field({label:'完整角色描述',name:'bio',value:draft.bio,type:'textarea',rows:6,hint:'描述身份、背景、生活、性格和世界。之后请求认知模型分析，确认页审阅结果。',attrs:{required:true}});
 }
 function paint(){
  if(!current())return;picker?.dispose();picker=null;
  const pending=command.pending(),locked=busy||!!pending||!!draft.activatedId;
  let body;
  if(draft.activatedId)body=`<h3>角色已激活</h3><p>标识：${esc(draft.activatedId)}</p><p>继续核对分组选择后进入角色。此步骤只操作已激活角色。</p>${Field({label:'角色分组',name:'activationGroupId',value:draft.activationGroupId||'',type:'select',options:groupOptions(draft.activationGroupId)})}<button class="btn" type="button" data-refresh-groups>重新读取分组</button><a class="btn" href="#/c/${encodeURIComponent(draft.activatedId)}/profile">查看已激活角色</a>`;
  else if(draft.step===0)body=`${Field({label:'创建方式',name:'mode',value:draft.mode,type:'select',options:[{value:'description',label:'描述并分析'},{value:'blank',label:'空白开始'},{value:'json',label:'导入原始初始化 JSON'},{value:'template',label:'从已发布角色模板创建'},{value:'analysis',label:'恢复当前所有者分析预览'}]})}${Field({label:'名字',name:'name',value:draft.name,attrs:{required:true,maxlength:200,'data-test':'wiz-name'}})}${sourceFields()}`;
  else if(draft.step===1)body=`<p>补充期望的性格特征（可留空）。</p>${Field({label:'性格描述',name:'traits',value:draft.traits,type:'textarea'})}`;
  else if(draft.step===2)body=`${draft.mode==='description'?Field({label:'世界与初始生活',name:'world',value:draft.world,type:'textarea',hint:'加入创建描述；不与其他角色共享运行世界。'}):'<p class="small muted">初始资料由所选创建来源决定。所有创建方式都可选择分组与生活时区。</p>'}${Field({label:'分组',name:'groupId',value:draft.groupId,type:'select',options:groupOptions(draft.groupId)})}${Field({label:'初始生活时区',name:'timezone',value:draft.timezone,hint:'使用 IANA 时区，例如 Asia/Shanghai。'})}`;
  else body=`<h3>${esc(draft.name)}</h3><p>分组：${esc(store.get('groups',draft.groupId)?.name||'未分组')} · 时区 ${esc(draft.timezone)}</p><div class="banner info">确认后激活独立角色，生活与自治任务按真实配置启动。</div>${CreationPreview(draft)}`;
  el.innerHTML=`<div class="card" style="max-width:640px;margin:auto"><div class="wizsteps">${['基础','性格','世界','确认'].map((label,i)=>`<span class="${i===draft.step?'on':''}">${i+1}. ${label}</span>`).join('')}</div><form class="stack" data-wf>${body}</form><p role="alert" data-result>${esc(errorText)}</p>${pending?`<div class="banner info"><p>原激活请求结果尚未确认。保留原内容与请求身份，先重试核对；不会使用新内容创建第二个角色。</p>${JsonDetails(pending.value,'原激活提交')}${pending.rejection?'<p>本次原提交已被明确拒绝，尚未激活。可保留输入重新取得分析来源。</p><button class="btn" data-release-analysis>保留输入并重新分析</button>':`<button class="btn primary" data-original ${busy?'disabled':''}>查询或重试原激活</button>`}</div>`:''}<div class="row" style="margin-top:16px"><button class="btn" data-cancel>保留草稿并返回</button><span class="spacer"></span>${draft.step&&!draft.activatedId?`<button class="btn" data-prev ${locked?'disabled':''}>上一步</button>`:''}<button class="btn primary" data-next data-test="${draft.step===3?'wiz-done':'wiz-next'}" ${busy||pending?'disabled':''}>${draft.activatedId?'保存所选分组并进入角色':draft.step===2?(draft.mode==='blank'||draft.mode==='analysis'?'审阅创建内容':'解析并审阅'):draft.step===3?'确认并激活':'下一步'}</button></div>${!pending&&!draft.activatedId?'<button class="btn ghost" data-reset>清除本地创建草稿</button>':''}</div>`;
  const form=el.querySelector('form');form.querySelectorAll('input,textarea,select,button').forEach(control=>control.disabled=locked);
  if(draft.activatedId&&!busy&&!pending){form.elements.activationGroupId.disabled=false;form.elements.activationGroupId.onchange=e=>{draft.activationGroupId=e.target.value;save();};el.querySelector('[data-refresh-groups]').disabled=false;el.querySelector('[data-refresh-groups]').onclick=async()=>{const token=++epoch;try{const latest=await directory.groups();if(!current()||token!==epoch)return;groupRows=latest;paint();}catch(error){if(current()&&token===epoch)fail(error);}};}
  form.oninput=e=>{if(locked)return;capture(form);if(['bio','traits','world','json'].includes(e.target.name)){draft.analysis=null;draft.analysisText='';}changed();};
  form.elements.mode?.addEventListener('change',()=>{capture(form);draft.analysis=null;draft.analysisText='';changed();paint();});
  form.onsubmit=e=>{e.preventDefault();el.querySelector('[data-next]').click();};
  if(draft.step===3&&!locked)bindCreationPreview(form,draft,{changed,error:fail});
  el.querySelector('[data-cancel]').onclick=()=>{save();ctx.navigate('#/library');};
  el.querySelector('[data-reset]')?.addEventListener('click',()=>{if(busy)return;draft=recovery.read();if(draft.activatedId){paint();return;}recovery.clear();draft=recovery.read();chooseLinkedTemplate();errorText='';paint();});
  el.querySelector('[data-prev]')?.addEventListener('click',()=>{capture(form);draft.step=draft.step===3?2:draft.step===2&&draft.mode!=='description'?0:draft.step-1;save();errorText='';paint();});
  el.querySelector('[data-original]')?.addEventListener('click',()=>void activate(true));
  el.querySelector('[data-release-analysis]')?.addEventListener('click',()=>{command.releaseRejected();draft.step=draft.mode==='analysis'?0:2;draft.analysis=null;draft.analysisText='';changed();errorText='原输入已保留，请重新分析或导入最新分析预览。';paint();});
  el.querySelector('[data-next]').onclick=()=>void next();
  if(draft.mode==='template'&&draft.step===0&&!locked)bindTemplates();
 }
 function bindTemplates(){
  const box=el.querySelector('[data-template-picker]');
  picker=mountCursorRecords(box,{alive:current,read:cursor=>directory.templates({q:search,cursor}),label:'已发布模板',empty:'没有匹配的已发布角色模板。可先在创作中心保存并发布模板。',render:r=>`<div class="card"><b>${esc(r.name)}</b><p class="small muted">已发布 ${esc(r.published_version)}</p><button type="button" class="btn" data-template="${esc(r.id)}" data-version="${esc(r.published_version)}" data-name="${esc(r.name)}">选择此版本</button></div>`,bind:panel=>panel.querySelectorAll('[data-template]').forEach(button=>button.onclick=()=>{
   draft.templateId=button.dataset.template;draft.templateVersion=button.dataset.version;draft.templateName=button.dataset.name;draft.analysis=null;epoch++;changed();paint();
  })});
  el.querySelector('[data-search-templates]').onclick=()=>{search=el.querySelector('[name=templateSearch]').value.trim();void picker.reload({reset:true});};
  el.querySelector('[data-template-read]')?.addEventListener('click',async()=>{
   const id=draft.templateId,version=draft.templateVersion,token=++epoch;
   try{const publication=await directory.publication(id,version);if(!current()||epoch!==token||draft.templateId!==id||draft.templateVersion!==version)return;el.querySelector('[data-template-body]').innerHTML=JsonDetails(publication,'精确发布内容');}catch(error){if(current()&&epoch===token)fail(error);}
  });
 }
 async function next(){
  if(busy)return;const saved=recovery.read();if(saved.activatedId){draft=saved;return activate(false);}const form=el.querySelector('form');capture(form);errorText='';
  if(draft.activatedId||draft.step===3)return activate(false);
  try{
   if(!draft.name.trim())throw new Error('请填写角色名字');
   if(draft.step===0){
    if(draft.mode==='description'&&!draft.bio.trim())throw new Error('请填写完整角色描述');
    if(draft.mode==='analysis'){draft.analysis=parseCreationAnalysis(draft.json);draft.analysisText=JSON.stringify(draft.analysis,null,2);}
    if(draft.mode==='json'||draft.mode==='template')creationAnalysisRequest(draft);
    draft.step=draft.mode==='description'?1:2;
   }else if(draft.step===1)draft.step=2;
   else {
    new Intl.DateTimeFormat('zh-CN',{timeZone:draft.timezone}).format();
    if(draft.mode==='analysis'){draft.analysis=parseCreationAnalysis(draft.json);draft.analysisText=JSON.stringify(draft.analysis,null,2);}
    if(draft.mode!=='blank'&&draft.mode!=='analysis'){
     busy=true;draft.analysis=null;draft.analysisText='';save();paint();const token=++epoch;
     const result=await directory.analyze(creationAnalysisRequest(draft));
     if(!current()||token!==epoch)return;
     draft.analysis=parseCreationAnalysis(JSON.stringify(result));draft.analysisText=JSON.stringify(draft.analysis,null,2);draft.includeGoals=true;
    }
    draft.step=3;draft.requestId=randomId();
   }
   save();
  }catch(error){fail(error);}finally{busy=false;if(current())paint();}
 }
 async function finishGrouping(){
  await directory.move(draft.activatedId,draft.activationGroupId||'');if(!current())return;
  const id=draft.activatedId;recovery.clear();toast('角色已激活，分组已读回确认','ok');ctx.navigate(`#/c/${encodeURIComponent(id)}/profile`);
 }
 async function activate(retry){
  if(busy||!(await requireOwner())||!current())return;
  try{
   draft=recovery.read();
   let submitted=retry?command.pending()?.value:null;
   if(!draft.activatedId&&!retry){
    if(draft.mode!=='blank')draft.analysis=parseCreationAnalysis(draft.analysisText);
    const analysis=draft.includeGoals?draft.analysis:{...draft.analysis,initial_goals:[],initial_intentions:[]};
    const body=draft.mode==='blank'?{requestId:draft.requestId,initializationMode:'blank_slate',name:draft.name,initializationTimezone:draft.timezone}:activationFromAnalysis(analysis,draft.name,draft.requestId,draft.timezone);
    submitted={body,groupId:draft.groupId};
   }
   busy=true;errorText='';save();paint();
   if(!draft.activatedId){const result=retry?await command.retry():await command.submit(submitted);if(!current())return;if(typeof result.id!=='string'||!result.id)throw new Error('激活响应缺少角色标识；保留原请求身份，请重新核对');draft.activatedId=result.id;draft.activationGroupId=submitted.groupId;save();}
   await finishGrouping();
  }catch(error){fail(error);}finally{busy=false;if(current())paint();}
 }
 try{const existing=studioDraftStorage().getItem(recovery.key);draft=recovery.read();if(!existing)chooseLinkedTemplate();else if(ctx.query.template&&ctx.query.template!==draft.templateId)errorText='已保留原创建草稿。若改用链接中的模板，请先核对原提交，再清除本地草稿。';paint();}catch(error){el.innerHTML=`<div class="statebox"><p role="alert">${esc(error.message)}</p><a href="#/library">返回角色库，保留原记录</a></div>`;}
 return()=>{alive=false;epoch++;picker?.dispose();};
}
