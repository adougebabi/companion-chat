import {errorMessage} from '../../domain-presentation.mjs';
import {SavedBindingSummary} from './configuration-summary.js';
import {bindingValues,providerBindingBody} from '../../provider-budget.mjs';
import {configurationClient} from '../services.js';
import {Field} from './forms.js';
import {esc,confirm} from '../ui.js';
import {mountConfigurationEditor} from './configuration-editor.js';

const roles=[{value:'generic_llm',label:'认知与通用模型'},{value:'conversation_reply',label:'交流表达'},{value:'embedding',label:'记忆向量'},{value:'vision_understanding',label:'视觉理解'}];
export function mountProviderEditors(host,{catalog,endpoints,bindings,alive=()=>true,onSaved=()=>{}}){
 let endpointEditor,bindingEditor,credentialEditor,active=true,lookupEpoch=0;
 let selectedEndpointId=endpoints[0]?.id||'',selectedRole='generic_llm';
 const live=()=>active&&alive()&&host.isConnected;
 host.innerHTML='<div class="provider-workspace"><aside class="stack"><div class="row"><h3 class="grow">模型连接</h3><button data-domain-ui="button" class="btn sm" data-new-endpoint>新建连接</button></div><div class="stack" data-endpoint-list></div><div class="small muted">先选择或保存连接，再为它配置凭据、选择模型用途。</div><a data-domain-ui="button" class="btn" href="#/setup">继续初始化向导</a></aside><section class="stack" data-connection-workspace></section></div>';
 function directory(){
  host.querySelector('[data-endpoint-list]').innerHTML=endpoints.length?endpoints.map(e=>`<button data-domain-ui="button" class="provider-entry ${e.id===selectedEndpointId?'on':''}" data-select-endpoint="${esc(e.id)}"><b>${esc(e.id)}</b><span>${esc(e.base_url)}</span><small>${e.secret_configured?'已保存访问凭据':'未配置凭据'} · ${(e.roles||[]).length} 个模型用途</small></button>`).join(''):'<p class="small muted">还没有模型连接，先保存一个服务地址。</p>';
  host.querySelectorAll('[data-select-endpoint]').forEach(b=>b.onclick=()=>selectEndpoint(b.dataset.selectEndpoint));
 }
 async function refreshCatalog(){const next=await configurationClient.read();if(!live())return false;catalog=next;endpoints=next.endpoints;bindings=next.bindings;directory();roleChoices();return true;}
 async function saved(result,latest){if(!await refreshCatalog())return;await onSaved(result,latest);}
 function roleChoices(){
  const target=host.querySelector('[data-role-choices]');if(!target)return;
  target.innerHTML=roles.map(r=>{const b=bindings.find(b=>b.role===r.value);return `<button data-domain-ui="button" class="provider-role ${r.value===selectedRole?'on':''}" data-select-role="${r.value}"><b>${r.label}</b><span>${b?'当前模型：'+esc(b.model_id)+' · 连接 '+esc(b.endpoint_id):'尚未绑定'}</span></button>`;}).join('');
  target.querySelectorAll('[data-select-role]').forEach(b=>b.onclick=()=>{selectedRole=b.dataset.selectRole;roleChoices();binding();});
 }
 function binding(){
  bindingEditor?.dispose();const box=host.querySelector('[data-binding-editor]');if(!box||!selectedEndpointId)return;
  const bound=bindings.find(b=>b.role===selectedRole),value=bindingValues(selectedRole,bound,catalog.provider_budget_defaults);
  host.querySelector('[data-binding-context]').innerHTML=bound&&bound.endpoint_id!==selectedEndpointId?`<p class="banner info">此用途当前绑定 ${esc(bound.endpoint_id)}。在这里保存，会明确改为使用当前连接 ${esc(selectedEndpointId)}；可以先从下方模型目录选择准确标识。</p>`:'';
  bindingEditor=mountConfigurationEditor(box,{slotKey:'binding:'+selectedRole,savedSummary:SavedBindingSummary,description:'选择此用途的服务、模型和超时。应用不设置输入、输出 Token 上限。',saveLabel:'保存模型用途',title:roles.find(r=>r.value===selectedRole).label,source:bound?'stored':'unconfigured',initial:value,versions:catalog.versions,operation:'binding',alive:live,contextFields:['modelId'],
   fields:[{key:'role',label:'用途',type:'select',fixed:selectedRole,options:roles.filter(r=>r.value===selectedRole)},{key:'endpointId',label:'使用当前连接',type:'text',fixed:selectedEndpointId,attrs:{readonly:true}},{key:'modelId',label:'模型精确标识',type:'text',hint:'可从此连接的模型目录点选，也可直接填写。保存后才会应用所选模型。'},{key:'timeoutSeconds',label:'请求超时（秒）',type:'number',min:1}],
   buildBody:providerBindingBody,readCurrent:async()=>{const next=await configurationClient.read(),b=next.bindings.find(v=>v.role===selectedRole);return {value:bindingValues(selectedRole,b,next.provider_budget_defaults),versions:next.versions,source:b?'stored':'unconfigured'};},onSaved:saved});
 }
 function selectEndpoint(id){
  selectedEndpointId=id;lookupEpoch++;endpointEditor?.dispose();bindingEditor?.dispose();credentialEditor?.dispose();
  const selected=endpoints.find(e=>e.id===id),workspace=host.querySelector('[data-connection-workspace]');
  workspace.innerHTML=`<div class="row"><h2 class="grow">${selected?'连接：'+esc(selected.id):'新建模型连接'}</h2>${!selected&&endpoints.length?'<button data-domain-ui="button" class="btn" type="button" data-cancel-endpoint>取消新建</button>':''}</div><p class="small muted">${selected?'此处的访问凭据、模型目录和用途绑定都属于当前连接。':'保存服务地址后，会接着显示凭据与模型用途。未配置的能力会提示准备状态，不妨碍浏览和准备草稿。'}</p><div data-endpoint-editor></div><details data-domain-ui="disclosure" class="provider-credentials" ${selected?'open':'hidden'}><summary>访问凭据 <span class="small muted">${selected?.secret_configured?'已配置':'服务需要时填写'}</span></summary><div data-credential-editor></div></details>${selected?'<section class="stack"><h3>选择此连接的模型用途</h3><div class="provider-role-list" data-role-choices></div><div data-binding-context></div><div class="stack"><button data-domain-ui="button" class="btn" type="button" data-models>读取此连接的模型目录</button><div data-model-list></div></div><div data-binding-editor></div></section>':''}`;
  const values=e=>({endpointId:e.id,kind:e.kind,baseUrl:e.base_url,secretPurpose:e.secret_purpose});
  endpointEditor=mountConfigurationEditor(workspace.querySelector('[data-endpoint-editor]'),{slotKey:'endpoint:'+id,title:'服务地址',saveLabel:'保存服务地址',source:selected?'stored':'unconfigured',initial:selected?values(selected):{endpointId:'',kind:'openai-compatible',baseUrl:'',secretPurpose:''},versions:catalog.versions,operation:'endpoint',alive:live,
   fields:[{key:'endpointId',label:'连接名称 / 标识',type:'text',fixed:selected?.id,attrs:{readonly:!!selected}},{key:'baseUrl',label:'API 服务地址',type:'url',hint:'填写模型 API 的根路径，例如 http://主机:端口/v1。'}],
   buildBody:(value,versions)=>({endpointId:value.endpointId,kind:value.kind||'openai-compatible',baseUrl:value.baseUrl,secretPurpose:value.secretPurpose||'provider:'+value.endpointId,expectedVersions:{...versions,['endpoint:'+value.endpointId]:versions['endpoint:'+value.endpointId]??0}}),
   readCurrent:async body=>{const next=await configurationClient.read(),actual=next.endpoints.find(e=>e.id===(body?.endpointId||id));return {value:actual?values(actual):{},versions:next.versions,source:actual?'stored':'unconfigured'};},
   beforeSave:()=>!selected||(selected.roles||[]).length===0||confirm({title:'更新连接地址',text:'更新后需要重新确认此连接原有的模型用途绑定；已受理任务继续使用冻结配置。'}),
   onSaved:async(result,latest)=>{if(!await refreshCatalog())return;const savedId=result.submittedBody?.endpointId||latest.value?.endpointId;if(!selectedEndpointId&&savedId&&workspace.querySelector('[name=endpointId]')?.value===savedId)selectEndpoint(savedId);await onSaved(result,latest);}});
  if(selected){
   credentialEditor=mountConfigurationEditor(workspace.querySelector('[data-credential-editor]'),{slotKey:'credential:'+selected.secret_purpose,title:'访问凭据（服务需要时填写）',saveLabel:'保存访问凭据',source:catalog.configured_secrets.includes(selected.secret_purpose)?'stored':'unconfigured',initial:{configured:catalog.configured_secrets.includes(selected.secret_purpose)},versions:catalog.versions,fields:[],secretPurpose:selected.secret_purpose,alive:live,
    readCurrent:async()=>{const next=await configurationClient.read();return {value:{configured:next.configured_secrets.includes(selected.secret_purpose)},versions:next.versions,source:next.configured_secrets.includes(selected.secret_purpose)?'stored':'unconfigured'};},buildBody:()=>({values:{},expectedVersions:{}}),onSaved:saved});
   roleChoices();binding();
   workspace.querySelector('[data-models]').onclick=async event=>{const epoch=++lookupEpoch,box=workspace.querySelector('[data-model-list]');event.target.disabled=true;box.textContent='正在读取当前连接的模型目录…';try{const result=await configurationClient.models(id);if(!live()||epoch!==lookupEpoch)return;box.innerHTML=result.models.length?result.models.map(model=>`<button data-domain-ui="button" class="btn sm" data-use-model="${esc(model)}">${esc(model)}</button>`).join(''):'<p class="banner info">连接返回了空目录，可核对服务中加载的模型；已填草稿保留。</p>';box.querySelectorAll('[data-use-model]').forEach(button=>button.onclick=()=>bindingEditor.adoptDraft({modelId:button.dataset.useModel},'已选择当前连接的模型。保存后绑定才生效。'));}catch(error){if(live()&&epoch===lookupEpoch)box.innerHTML=`<p class="banner info">${esc(errorMessage(error))}。可以继续保留草稿并核对当前连接地址与访问凭据。</p>`;}finally{if(live()&&epoch===lookupEpoch)event.target.disabled=false;}};
  }
  workspace.querySelector('[data-cancel-endpoint]')?.addEventListener('click',()=>selectEndpoint(endpoints[0].id));
  directory();
 }
 host.querySelector('[data-new-endpoint]').onclick=()=>selectEndpoint('');
 selectEndpoint(selectedEndpointId);
 return {dispose(){active=false;lookupEpoch++;endpointEditor?.dispose();bindingEditor?.dispose();credentialEditor?.dispose();}};
}

// Q06 uses the same per-role save slot as the connection workspace.
export function mountModelPurposeEditor(host,{catalog,role='conversation_reply',alive=()=>true,onSaved=()=>{}}){
 const bound=catalog.bindings.find(b=>b.role===role),value=bindingValues(role,bound,catalog.provider_budget_defaults);
 return mountConfigurationEditor(host,{slotKey:'binding:'+role,title:'模型用途 · '+(roles.find(r=>r.value===role)?.label||role),saveLabel:'保存模型用途',description:'选择此用途的服务、模型和超时。每个用途保留一份草稿；应用不设置 Token 上限。',source:bound?'stored':'unconfigured',initial:value,versions:catalog.versions,operation:'binding',alive,savedSummary:SavedBindingSummary,
  fields:[{key:'role',label:'用途',type:'select',fixed:role,options:roles.filter(r=>r.value===role)},{key:'endpointId',label:'模型连接',type:'select',options:[{value:'',label:'请选择连接'},...catalog.endpoints.map(e=>({value:e.id,label:e.id}))],resetOnChange:['modelId']},{key:'modelId',label:'模型精确标识',type:'text',hint:'可在模型与连接页面查阅服务返回的模型目录。'},{key:'timeoutSeconds',label:'用途请求超时（秒）',type:'number',min:1}],
  buildBody:providerBindingBody,readCurrent:async()=>{const next=await configurationClient.read(),binding=next.bindings.find(b=>b.role===role);return {value:bindingValues(role,binding,next.provider_budget_defaults),versions:next.versions,source:binding?'stored':'unconfigured'};},onSaved});
}
