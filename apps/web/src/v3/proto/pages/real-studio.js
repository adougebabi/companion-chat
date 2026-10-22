import {OutfitDefinitionFields,mountOutfitDefinition,newOutfitDefinition,OutfitApplicationMembers} from '../components/outfit-definition.js';
import {ItemDefinitionFields,bindItemDefinition,newItemDefinition} from '../components/item-definition.js';
import {FoundationSourceLinks} from '../components/studio-foundation.js';
import {VisualStyleFields,bindVisualStyle,newVisualStyle} from '../components/visual-style.js';
import {mountResourceLifecycle} from '../components/resource-lifecycle.js';
import {CharacterTemplateFields,bindCharacterTemplate,newCharacterTemplate} from '../components/character-template.js';
import {readStudioDraft,writeStudioDraft} from '../components/studio-working-draft.js';
import {mountStudioJobs} from '../components/studio-jobs.js';
import * as store from '../store.js';
import {studioClient} from '../services.js';
import {esc,toast,requireOwner,confirm} from '../ui.js';
import {Field,Tabs,DiffRows,SearchForm} from '../components/forms.js';
import {JsonDetails} from '../components/records.js';
import {mountCursorRecords} from '../components/cursor-records.js';
import {EventDefinitionFields,bindEventDefinition,newEventDefinition} from '../components/event-definition.js';
import {studioDraftStorage} from '../components/studio-draft-storage.js';
import {persistentCommand} from '../../providers/persistent-command.mjs';
const kinds={event_template:'事件模板',npc:'NPC',place:'地点',item:'物品',outfit:'搭配',world:'世界',character_template:'角色模板',persona_fragment:'人格片段',visual_style:'视觉风格'};
const owner=()=>store.get('session')?.actorId;
const json=value=>JSON.stringify(value,null,2);
function scope(el){const actor=owner();let active=true;return {actor,alive:()=>active&&el.isConnected&&owner()===actor,dispose:()=>{active=false;}};}
export function renderRealStudioHome(el,ctx){
 const life=scope(el),type=ctx.query.type||'全部',kind=Object.keys(kinds).find(k=>kinds[k]===type);let search=ctx.query.q||'',archived=ctx.query.archived==='1';
 const createHref=kind==='npc'?`#/c/${ctx.charId}/people/npc/new`:`#/studio/edit/new?kind=${encodeURIComponent(kind||'event_template')}`,createLabel=kind?'新建'+kinds[kind]:'新建资源';
 el.innerHTML=`<div class="stack studio-home"><div class="row">${Tabs({value:type,className:'tabs grow',items:['全部',...Object.values(kinds)].map(value=>({value,label:value,href:`#/studio?type=${encodeURIComponent(value)}`}))})}<a class="btn" href="#/studio/media">我的素材库</a><a class="btn primary" href="${esc(createHref)}">${esc(createLabel)}</a></div><p class="small muted">全局定义保存、发布与应用独立进行。事件必须应用到角色当前世界并启用后才可运行。</p><label class="toggle"><input type="checkbox" data-archived ${archived?'checked':''}> 查看已归档定义</label><div data-search>${SearchForm({value:search,placeholder:'搜索资源名称'})}</div><div class="stack" data-directory></div></div>`;
 const panel=mountCursorRecords(el.querySelector('[data-directory]'),{alive:life.alive,read:cursor=>studioClient.list({kind,q:search,archived,cursor}),label:'资源',empty:'当前目录尚无资源',render:r=>`<a class="card lrow" href="#/studio/${r.kind==='npc'?'npc':'edit'}/${encodeURIComponent(r.id)}"><span class="grow"><b>${esc(r.name)}</b><p class="small muted">${esc(kinds[r.kind]||r.kind)} · ${r.published_version?'已发布 v'+esc(r.published_version):'尚未发布'} · ${r.draft_version?'保存 v'+esc(r.draft_version):'尚未保存'}</p></span>编辑</a>`});
 el.querySelector('[data-archived]').onchange=e=>{archived=e.target.checked;void panel.reload({reset:true});};
 el.querySelector('[data-search] form').onsubmit=e=>{e.preventDefault();search=e.target.elements.q.value.trim();void panel.reload({reset:true});};
 return()=>{life.dispose();panel.dispose();};
}
export function renderRealStudioEdit(el,ctx){
 const life=scope(el),creating=ctx.params.resId==='new';let resource=null,busy=false,conflict=false,preview=null,previewEpoch=0,readEpoch=0,errorText='';
 const slot=`fluctlight-studio:${life.actor}:${ctx.params.resId}`,draftKey=slot+':draft',commandKey=slot+':command';
 let draft=null,jobPanel=null,lifecyclePanel=null,definitionPanel=null;
 const command=persistentCommand({key:commandKey,storage:()=>studioDraftStorage(),execute:async(value,key)=>{
  if(!life.alive())throw new Error('会话或资源范围已切换，请返回原范围重试原提交');
  if(value.operation==='save')return studioClient.save(value.id,{...value.body,idempotencyKey:key});
  if(value.operation==='publish')return studioClient.publish(value.id,value.revision,key);
  if(value.operation==='prepare')return studioClient.createJob({...value.body,idempotencyKey:key});
  if(value.operation==='apply')return studioClient.apply(value.id,{...value.body,idempotencyKey:key});
  throw new Error('未知创作命令');
 }});
 const persist=()=>{writeStudioDraft(life.actor,ctx.params.resId,draft);jobPanel?.updateDraft();const pub=el.querySelector('[data-publish]');if(pub)pub.disabled=busy||!resource?.draft_version||draft.name!==resource.name||json(draft.body)!==json(resource.body||{});const diff=el.querySelector('[data-draft-diff]');if(diff)diff.innerHTML=DiffRows(['name','body'].filter(k=>json(k==='name'?(resource?.name||''):resource?.body)!==json(draft[k])).map(field=>({field:field==='name'?'名称':'定义内容',before:json(field==='name'?(resource?.name||''):resource?.body),after:json(draft[field])})));};
 const fail=error=>{if(!life.alive())return;conflict=!!command.pending()?.rejection;paint(error.message);};
 async function submit(value,retry=false){
  if(busy||!(await requireOwner())||!life.alive())return;busy=true;errorText='';paint();
  try{const result=retry?await command.retry():await command.submit(value);if(!life.alive())return;
   const op=retry?value.operation:value.operation;
   if(op==='save'){
    // Keep any newer local draft separate from the immutable submitted body.
    const unchanged=json({name:draft.name,body:draft.body,kind:draft.kind})===json({name:value.body.name,body:value.body.body,kind:value.body.kind});
    resource=result;draft.expectedVersion=result.draft_version||result.published_version||undefined;
    if(unchanged){draft.name=result.name;draft.body=structuredClone(result.body||value.body.body);}persist();
    if(creating){writeStudioDraft(life.actor,result.id,draft);studioDraftStorage().removeItem(draftKey);ctx.navigate(`#/studio/edit/${encodeURIComponent(result.id)}`);return;}
    resource=await studioClient.resource(result.id);if(!life.alive())return;toast('定义草稿已保存并读回','ok');
   }else if(op==='publish'){resource=await studioClient.resource(value.id);if(!life.alive())return;toast('保存的定义版本已发布','ok');}
   else if(op==='prepare'){ctx.query.job=result.id;toast('隔离解析任务已受理，请在右栏核对真实结果并审阅采用。','ok');}
   else if(result.target_kind==='foundation'&&result.status==='proposed'&&result.proposal_id){resource=await studioClient.resource(value.id);if(!life.alive())return;preview=null;toast('身份核心提案已保存，尚未生效；请到提案详情接受并应用。','ok');}
   else {if(!result.applied)throw new Error('服务端尚未应用此资源，请核对实际结果');if(draft.target===value.body.fluctlightId)preview=null;toast(result.target_kind==='wardrobe_outfit'?'有来源的实际服饰组合已保存，可到衣柜核对；穿着仍需独立请求。':result.target_kind==='world_item'?'物品已注册到运行世界目录，尚未取得；可到世界物品目录配置和提出请求。':result.target_kind==='visual_brief'?'视觉参考已保存并读回；新的视觉生成会话将使用此版本。':`定义已应用到 ${store.get('characters',value.body.fluctlightId)?.name||value.body.fluctlightId}，请到对应世界核对`,'ok');}
   conflict=false;
  }catch(error){fail(error);}finally{busy=false;if(life.alive())paint();}
 }
 function paint(error=null){
  if(error!==null)errorText=error;error=errorText;
  if(!life.alive()||!draft)return;
  let pending=null;try{pending=command.pending();}catch(e){error=e.message;}
  conflict=!!pending?.rejection;
  const event=draft.kind==='event_template',character=draft.kind==='character_template',visual=draft.kind==='visual_style',item=draft.kind==='item',outfit=draft.kind==='outfit',saved=resource?.body||{},dirty=!resource||draft.name!==resource.name||json(draft.body)!==json(saved);
  jobPanel?.dispose();lifecyclePanel?.dispose();definitionPanel?.dispose();
  el.innerHTML=`<div class="studio"><div class="stack"><div class="card"><h2>${creating?'新建资源':esc(resource.name)}</h2><p class="small muted">全局定义 · ${esc(resource?.published_version||'尚未发布')} · 保存版本 ${esc(resource?.draft_version||'无')}</p><p class="banner info">保存与发布只更新定义。应用到实例后，首次事件模板保持关闭；启用和触发在世界设置中独立进行。</p></div><form class="card stack" data-editor novalidate>${Field({label:'名称',name:'name',value:draft.name,attrs:{required:true}})}${creating?Field({label:'资源类型',name:'kind',value:draft.kind,type:'select',options:Object.entries(kinds).filter(([k])=>k!=='npc').map(([value,label])=>({value,label}))}):''}<div data-definition>${event?EventDefinitionFields(draft.body):character?CharacterTemplateFields(draft.body):visual?VisualStyleFields(draft.body):item?ItemDefinitionFields(draft.body):outfit?OutfitDefinitionFields(draft.body):Field({label:'定义内容（JSON）',name:'body',value:json(draft.body),type:'textarea',attrs:{rows:16}})}</div>${Field({label:'保存原因',name:'reason',value:draft.reason,attrs:{required:true,maxlength:500}})}<div class="row"><button class="btn primary" ${busy?'disabled':''}>保存草稿</button><button type="button" class="btn" data-rebase ${busy?'disabled':''}>重读并保留草稿</button></div></form><details class="card"><summary>已保存版本与当前草稿的差异</summary><div data-draft-diff>${DiffRows(['name','body'].filter(k=>json(k==='name'?(resource?.name||''):saved)!==json(draft[k])).map(field=>({field:field==='name'?'名称':'定义内容',before:json(field==='name'?(resource?.name||''):saved),after:json(draft[field])})))}</div></details><p role="alert" class="banner ${error?'err':'info'}" ${error?'':'hidden'}>${esc(error)}</p>${pending?`<div class="card"><p>原提交结果尚未确认，重试将保留原内容和幂等身份。当前新草稿单独保留。</p>${JsonDetails(pending.value,'查看原提交')}<button class="btn" data-original ${busy?'disabled':''}>重试原提交</button></div>`:''}${resource?`<button class="btn" data-confirm-rebase ${pending&&!conflict?'disabled':''}>明确改用最新版本作为草稿基线</button>`:''}<div class="row"><button class="btn primary" data-publish ${busy||dirty||!resource?.draft_version?'disabled':''}>发布已保存版本</button><a class="btn" href="#/studio">返回目录</a>${resource?`<a class="btn" href="#/studio/preview?res=${encodeURIComponent(resource.id)}">文本预览沙盒</a>`:''}</div></div><div class="stack"><div data-studio-jobs></div><div class="card">${character?`<a class="btn primary" href="#/library/create?template=${encodeURIComponent(resource?.id||'')}&version=${encodeURIComponent(resource?.published_version||'')}">以已发布版本创建角色</a>`:''}${draft.kind==='world'&&resource?.published_version?`<a class="btn primary" href="#/library/create?worldResourceId=${encodeURIComponent(resource.id)}&worldVersion=${encodeURIComponent(resource.published_version)}">以此世界底稿继续人物初始化</a>`:''}<h3>应用已发布版本</h3><p class="small muted">${visual?'选择目标角色，逐项确认视觉参考差异；不会触发生成或改变现有规范图。':outfit?'选择目标角色当前人格，为搭配成员明确选择实际库存服饰；保存组合与穿着分别进行。':character||draft.kind==='persona_fragment'?'对照目标角色Foundation字段，逐项选择后形成提案，再到治理页接受并应用。':'选择角色的当前活动世界，先读取实际差异再确认应用。'}已发布 ${esc(resource?.published_version||'无')}</p>${Field({label:'目标角色',name:'target',value:draft.target||'',type:'select',options:[{value:'',label:'请选择角色'},...store.list('characters').filter(c=>c.status!=='已退休').map(c=>({value:c.id,label:c.name}))]})}<button class="btn" data-preview ${busy||!resource?.published_version?'disabled':''}>读取应用差异</button><div data-preview-body></div></div>${FoundationSourceLinks(resource)}<div data-resource-lifecycle></div>${JsonDetails(resource?.versions||[],'保存版本记录')}</div></div>`;
  jobPanel=mountStudioJobs(el.querySelector('[data-studio-jobs]'),{actor:life.actor,resource,readDraft:()=>draft,adopt:value=>{draft.name=value.name;draft.body=value.body;persist();paint();},alive:life.alive,jobId:ctx.query.job||'',onSelect:id=>{ctx.query.job=id;},linkBase:resource?`#/studio/edit/${encodeURIComponent(resource.id)}`:''});
  if(resource)lifecyclePanel=mountResourceLifecycle(el.querySelector('[data-resource-lifecycle]'),{actor:life.actor,resource,alive:life.alive,navigate:ctx.navigate,onChanged:result=>{resource=result;paint();}});
  const form=el.querySelector('[data-editor]');form.elements.name.oninput=e=>{draft.name=e.target.value;persist();};form.elements.reason.oninput=e=>{draft.reason=e.target.value;persist();};
  if(event)bindEventDefinition(el.querySelector('[data-definition]'),draft.body,{onChange:persist,repaint:paint});
  else if(character)bindCharacterTemplate(el.querySelector('[data-definition]'),draft.body,{changed:persist,repaint:paint});
  else if(outfit)definitionPanel=mountOutfitDefinition(el.querySelector('[data-definition]'),draft.body,{changed:persist,alive:life.alive});
  else if(item)bindItemDefinition(el.querySelector('[data-definition]'),draft.body,{changed:persist,repaint:paint});
  else if(visual)bindVisualStyle(el.querySelector('[data-definition]'),draft.body,{changed:persist});
  else form.elements.body.oninput=e=>{try{const body=JSON.parse(e.target.value);if(!body||Array.isArray(body)||typeof body!=='object')throw new Error('定义须为JSON对象');draft.body=body;persist();e.target.setCustomValidity('');}catch(error){e.target.setCustomValidity(error.message);}};
  if(creating)form.querySelector('select[name=kind]').onchange=e=>{draft.kind=e.target.value;draft.body=draft.kind==='event_template'?newEventDefinition():draft.kind==='character_template'?newCharacterTemplate():draft.kind==='outfit'?newOutfitDefinition():draft.kind==='item'?newItemDefinition():draft.kind==='visual_style'?newVisualStyle():draft.kind==='persona_fragment'?{schema:'persona-fragment.v1',changes:{}}:{};persist();paint();};
  form.onsubmit=e=>{e.preventDefault();if(!form.reportValidity())return;if(!draft.name.trim()||!draft.reason.trim()){fail(new Error('请填写名称和保存原因'));return;}void submit({operation:'save',id:resource?.id||null,body:{kind:draft.kind,name:draft.name,body:structuredClone(draft.body),expectedVersion:draft.expectedVersion,reason:draft.reason}});};
  el.querySelector('[data-rebase]').onclick=()=>void load(true);
  el.querySelector('[data-original]')?.addEventListener('click',()=>void submit(pending.value,true));
  el.querySelector('[data-confirm-rebase]')?.addEventListener('click',async()=>{if(!(await confirm({title:'重设草稿基线',text:'保留当前草稿，读取最新保存版本后按新版本重新提交。未确认结果的原请求必须先核对。'}))||!life.alive())return;await load(true,true);});
  el.querySelector('[data-publish]').onclick=async()=>{if(draft.name!==resource?.name||json(draft.body)!==json(resource?.body||{}))return fail(new Error('请先保存当前修改，再发布保存版本'));const id=resource.id,revision=resource.revision;if(!(await confirm({title:'发布保存版本',text:`发布 ${resource.name} 的保存版本 ${resource.draft_version}。应用和运行仍须单独操作。`}))||!life.alive())return;void submit({operation:'publish',id,revision});};
  el.querySelector('[name=target]').onchange=e=>{draft.target=e.target.value;persist();preview=null;previewEpoch++;el.querySelector('[data-preview-body]').replaceChildren();};
  el.querySelector('[data-preview]').onclick=()=>void readPreview();if(preview)paintPreview();if(resource?.status==='archived'){form.querySelectorAll('input,textarea,select,button').forEach(c=>c.disabled=true);el.querySelector('[data-publish]').disabled=true;el.querySelector('[data-preview]').disabled=true;}
 }
 async function readPreview(){const fid=draft.target,version=resource?.published_version,epoch=++previewEpoch;if(!fid||!version)return fail(new Error('请选择目标角色并先发布定义'));try{const p=await studioClient.preview(fid,resource.id,version);if(!life.alive()||epoch!==previewEpoch||fid!==draft.target)return;preview={...p,fid,version,choices:{},memberItems:(p.members||[]).map(()=>'')};paintPreview();}catch(error){if(epoch===previewEpoch)fail(error);}}
 function paintPreview(){
  const p=preview,box=el.querySelector('[data-preview-body]');if(!box||!p)return;
  if(p.preparation_required){box.innerHTML=`<p class="banner info">${esc(p.semantics)}</p><button class="btn primary" data-prepare-template ${busy?'disabled':''}>解析已发布文本为结构化定义</button><p class="small muted">这会运行隔离创作任务。完成后先比较并采用到草稿，再保存、发布及读取差异；不会创建角色或替换初始化分析凭证。</p>`;box.querySelector('[data-prepare-template]').onclick=async()=>{try{const id=resource.id,revision=resource.revision,pub=await studioClient.publication(id,p.version);if(!life.alive()||preview!==p)return;void submit({operation:'prepare',id,body:{resourceId:id,resourceRevision:revision,kind:'assistant',purpose:'foundation_prepare',resourceVersion:p.version,name:pub.name,body:pub.body,instruction:'按精确已发布文本解析完整结构化角色定义，保持来源意图，供所有者审阅后形成Foundation应用提案',reason:'所有者明确解析已发布文本模板'}});}catch(error){fail(error);}};return;}
  const rows=p.diff||[];box.innerHTML=`<h3>实际应用差异</h3>${DiffRows(rows.map(row=>({field:row.field,before:json(row.from),after:json(row.to)})))}${rows.filter(()=>p.runtime_id).map(row=>Field({label:`${row.field} 的处理`,name:row.field,type:'select',value:p.choices[row.field]||'',options:[{value:'',label:'请选择'},{value:'keep',label:'保留实例值'},{value:'override',label:'采用发布值'}],attrs:{'data-apply-choice':row.field}})).join('')}${JsonDetails(p.conflicts,'冲突来源')}${p.target_kind==='wardrobe_outfit'?OutfitApplicationMembers(p):''}<p class="small muted">${p.target_kind==='wardrobe_outfit'?esc(p.semantics):p.target_kind==='world_item'?esc(p.semantics):p.target_kind==='foundation'?esc(p.semantics):p.target_kind==='visual_brief'?'应用只保存目标角色的视觉参考。当前身体、穿着、规范图和已排队会话保持；新会话才使用新参考。':'应用只更新选定世界中的定义。不会触发事件或改写已有生活事实。'}</p><button class="btn teal" data-confirm-apply ${busy?'disabled':''}>${p.target_kind==='foundation'?'创建身份核心应用提案':'确认应用'} ${esc(p.version)}</button><p data-apply-error role="alert"></p>`;
  box.querySelectorAll('[data-apply-choice]').forEach(input=>input.onchange=()=>{p.choices[input.dataset.applyChoice]=input.value;});
  box.querySelectorAll('[data-outfit-member]').forEach(input=>input.oninput=()=>{p.memberItems[Number(input.dataset.outfitMember)]=input.value.trim();});
  box.querySelector('[data-confirm-apply]').onclick=async()=>{if(p.runtime_id&&rows.some(row=>!p.choices[row.field])){box.querySelector('[data-apply-error]').textContent='请逐项选择如何处理实际差异';return;}if(p.target_kind==='wardrobe_outfit'&&p.choices.members!=='keep'&&p.memberItems.some(id=>!id)){box.querySelector('[data-apply-error]').textContent='请为每个定义成员明确选择实际库存服饰';return;}void submit({operation:'apply',id:resource.id,body:{fluctlightId:p.fid,resourceVersion:p.version,expectedInstanceRevision:p.instance_revision,expectedRuntimeRevision:p.runtime_revision??0,...(p.target_kind==='wardrobe_outfit'?{profileId:p.profile_id,expectedWardrobeRevision:p.wardrobe_revision,memberItems:p.members.map((m,i)=>({resourceId:m.resource_id,resourceVersion:m.resource_version,itemId:p.memberItems[i]}))}:{}),...(p.world_id?{worldId:p.world_id,expectedWorldRevision:p.world_binding_revision}:{}),choices:Object.entries(p.choices).map(([field,decision])=>({field,decision})),reason:'所有者确认差异后应用已发布资源'}});};
 }
 async function load(preserve=false,rebase=false){
  const reading=++readEpoch;try{const latest=creating?null:await studioClient.resource(ctx.params.resId);if(!life.alive()||reading!==readEpoch)return;if(latest?.kind==='npc'){ctx.navigate(`#/studio/npc/${encodeURIComponent(latest.id)}${ctx.query.job?'?job='+encodeURIComponent(ctx.query.job):''}`);return;}
   resource=latest;if(!draft)draft=readStudioDraft(life.actor,ctx.params.resId,latest,{kind:ctx.query.kind||'event_template',body:ctx.query.kind==='outfit'?newOutfitDefinition():ctx.query.kind==='item'?newItemDefinition():ctx.query.kind==='persona_fragment'?{schema:'persona-fragment.v1',changes:{}}:ctx.query.kind==='visual_style'?newVisualStyle():ctx.query.kind==='character_template'?newCharacterTemplate():ctx.query.kind&&ctx.query.kind!=='event_template'?{}:newEventDefinition()});
   if(rebase){if(command.pending())command.releaseRejected();draft.expectedVersion=latest?.draft_version||latest?.published_version||undefined;conflict=false;persist();}preview=null;previewEpoch++;paint(preserve&&!rebase?'已重读，草稿和原版本基线保留；如需以最新版本保存，请明确重设基线。':'');
  }catch(error){if(!life.alive()||reading!==readEpoch)return;if(!draft){el.innerHTML=`<div class="statebox"><p role="alert">${esc(error.message)}</p><button class="btn" data-reload>重新读取</button></div>`;el.querySelector('[data-reload]').onclick=()=>void load();}else fail(error);}
 }
 el.innerHTML='<div class="statebox" role="status">正在读取资源…</div>';void load();return()=>{life.dispose();previewEpoch++;jobPanel?.dispose();lifecyclePanel?.dispose();definitionPanel?.dispose();};
}

export function renderRealStudioPreview(el,ctx){
 const life=scope(el);let resource=null,panel=null,picker=null,reading=0;
 async function load(){const token=++reading;panel?.dispose();picker?.dispose();el.innerHTML='<div class="statebox">正在读取创作资源…</div>';
  try{
   const id=ctx.query.res;resource=id?await studioClient.resource(id):null;if(!life.alive()||token!==reading)return;
   el.innerHTML=`<div class="studio"><div class="card stack"><h2>文本预览沙盒</h2><p class="small muted">选择实际资源，使用其已保存定义或编辑页保留的工作草稿。预览不进入生活。</p>${resource?`<b>${esc(resource.name)}</b><a class="btn" href="#/studio/${resource.kind==='npc'?'npc':'edit'}/${encodeURIComponent(resource.id)}">编辑同一份工作草稿</a><div data-selected-draft></div>`:'<p>选择一项实际资源进入文本预览。</p><div data-resource-picker></div>'}<a class="btn" href="#/studio">资源目录</a></div><div data-studio-jobs></div></div>`;
   if(!resource){picker=mountCursorRecords(el.querySelector('[data-resource-picker]'),{alive:life.alive,read:cursor=>studioClient.list({cursor}),label:'资源',empty:'尚无创作资源',render:r=>`<a class="lrow" href="#/studio/preview?res=${encodeURIComponent(r.id)}">${esc(r.name)} · ${esc(kinds[r.kind]||r.kind)}</a>`});return;}
   let draft=readStudioDraft(life.actor,resource.id,resource);
   el.querySelector('[data-selected-draft]').innerHTML=JsonDetails(draft,'当前工作草稿');
   panel=mountStudioJobs(el.querySelector('[data-studio-jobs]'),{initialKind:'text_preview',actor:life.actor,resource,readDraft:()=>draft,adopt:value=>{draft={...draft,...value};writeStudioDraft(life.actor,resource.id,draft);el.querySelector('[data-selected-draft]').innerHTML=JsonDetails(draft,'当前工作草稿');toast('建议已采用到此资源的工作草稿，尚未保存定义','ok');},alive:life.alive,jobId:ctx.query.job||'',onSelect:id=>{ctx.query.job=id;},linkBase:`#/studio/preview?res=${encodeURIComponent(resource.id)}`});
  }catch(e){if(life.alive()&&token===reading)el.innerHTML=`<div class="statebox"><p role="alert">${esc(e.message)}</p><a href="#/studio">返回资源目录</a></div>`;}
 }
 void load();return()=>{life.dispose();reading++;panel?.dispose();picker?.dispose();};
}
