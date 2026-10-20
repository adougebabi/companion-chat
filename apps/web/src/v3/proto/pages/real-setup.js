import * as store from '../store.js';
import {configurationClient} from '../services.js';
import {client} from '../../providers/real.ts';
import {persistentCommand} from '../../providers/persistent-command.mjs';
import {studioDraftStorage} from '../components/studio-draft-storage.js';
import {Field} from '../components/forms.js';
import {esc,toast} from '../ui.js';

export function renderRealSetup(el,ctx){
 const owner=store.get('session')?.actorId,storage=studioDraftStorage(),key=`fluctlight-setup:${owner}`;
 let alive=true,busy=false,catalog=null,worlds=null,warning='';
 let draft;try{draft=JSON.parse(storage.getItem(key)||'null');}catch{warning='原向导草稿暂不可读，可以继续核对已保存定义。';}draft||={name:'',background:'',rules:'',resourceId:null,resourceVersion:null};
 const save=()=>storage.setItem(key,JSON.stringify(draft)),live=()=>alive&&el.isConnected&&store.get('session')?.actorId===owner;
 const command=persistentCommand({key:key+':command',storage:()=>storage,execute:async(value,idempotencyKey)=>{
  const result=value.operation==='create'?await client.createStudioResource({...value.body,idempotencyKey}):value.operation==='update'?await client.updateStudioResource(value.id,{...value.body,idempotencyKey}):await client.publishStudioResource(value.id,{expectedRevision:value.revision,reason:'发布初始化世界底稿',idempotencyKey});
  if(value.operation!=='publish'){draft.resourceId=result.id;draft.resourceRevision=result.revision;draft.resourceVersion=null;}else draft.resourceVersion=result.published_version;
  save();return result;
 }});
 function choose(id,version){draft.resourceId=id;draft.resourceVersion=version;save();ctx.navigate(`#/library/create?worldResourceId=${encodeURIComponent(id)}&worldVersion=${encodeURIComponent(version)}`);}
 function paint(){if(!live())return;const bindings=catalog?.bindings||[],media=catalog?.effective_values?.['media.comfyui'];
  el.innerHTML=`<div class="settings-page stack"><div class="wizsteps"><span class="on">1. 服务连接</span><span class="on">2. 世界底稿</span><span>3. 创建人物</span></div><p>按同一条流程完成准备。可先保存底稿；未配置的能力会明确提示，之后仍可继续补充。</p>${warning?`<p class="banner info">${esc(warning)}</p>`:''}<section class="card stack"><h2>1. 确认模型与媒体服务</h2><div class="grid g2">${[['generic_llm','认知模型 · Qwen'],['conversation_reply','交流表达 · Cydonia'],['embedding','记忆向量'],['vision_understanding','视觉理解']].map(([role,label])=>{const b=bindings.find(b=>b.role===role);return `<div><b>${label}</b><p class="small muted">${catalog?(b?'已绑定 '+esc(b.model_id)+' · '+esc(b.endpoint_id):'尚未绑定；对应能力准备好后可继续使用'):'配置状态尚未读到'}</p></div>`;}).join('')}<div><b>ComfyUI</b><p class="small muted">${media?'已保存媒体服务配置，生成结果待实际使用确认':'尚未配置；不妨碍先准备世界与人物草稿'}</p></div></div><div class="row"><a class="btn primary" href="#/settings/models">配置模型连接与用途</a><a class="btn" href="#/settings/media-workflows">配置 ComfyUI</a><button class="btn" data-refresh>重新读取准备状态</button></div></section><section class="card stack"><h2>2. 选择已发布世界底稿</h2><p>底稿可以先于人物创建。人物激活后会生成自己的运行世界，并明确选用；不同人物保持独立。</p><div class="stack">${worlds===null?'<p class="small muted">世界目录尚未读取，可以先填写下面的新底稿。</p>':worlds.length?worlds.map(w=>`<div class="lrow"><span class="grow"><b>${esc(w.name)}</b><p class="small muted">发布版本 ${esc(w.published_version)}</p></span><button class="btn" data-world="${esc(w.id)}" data-version="${esc(w.published_version)}">选择并创建人物</button></div>`).join(''):'<p class="small muted">还没有发布的世界底稿，可在下方创建。</p>'}</div><form class="stack" data-world-form>${Field({label:'世界名称',name:'name',value:draft.name,attrs:{required:true}})}${Field({label:'背景与地点概况',name:'background',value:draft.background,type:'textarea',hint:'可以先写概况，尚未确定的细节留空，之后继续补充。'})}${Field({label:'世界规则（可留空）',name:'rules',value:draft.rules,type:'textarea'})}<button class="btn primary" ${busy?'disabled':''}>${draft.resourceId&&!draft.resourceVersion?'继续发布已保存的底稿':'保存并发布世界底稿'}</button></form>${command.pending()?'<button class="btn" data-original>继续核对原世界提交</button>':''}</section><section class="card stack"><h2>3. 创建或继续完善人物</h2><p>选择上面的底稿会带入人物向导。已经激活的人物仅继续世界与分组步骤，不会再创建一个人物。</p><div class="row"><a class="btn" href="#/library/create">继续已有创建草稿</a><a class="btn" href="#/library">查看人物</a></div></section></div>`;
  el.querySelectorAll('[data-world]').forEach(b=>b.onclick=()=>choose(b.dataset.world,b.dataset.version));
  el.querySelector('[data-refresh]').onclick=()=>void load();
  const form=el.querySelector('[data-world-form]');form.querySelectorAll('input,textarea').forEach(input=>input.disabled=busy);form.oninput=()=>{draft.name=form.elements.name.value;draft.background=form.elements.background.value;draft.rules=form.elements.rules.value;save();};
  form.onsubmit=e=>{e.preventDefault();if(form.reportValidity())void publish(false);};el.querySelector('[data-original]')?.addEventListener('click',()=>void publish(true));
 }
 async function publish(retry){if(busy)return;busy=true;warning='';paint();try{
  if(retry){await command.retry();warning='原世界提交已确认。当前草稿保留，请继续明确保存或选择已发布底稿。';return;}
  if(!draft.resourceId)await command.submit({operation:'create',body:{kind:'world',name:draft.name||'新的世界',body:{background:draft.background||'',rules:draft.rules||''},reason:'准备人物初始化世界底稿'}});
  let actual=await client.studioResource(draft.resourceId);
  if(actual.name!==draft.name||(actual.body?.background||'')!==(draft.background||'')||(actual.body?.rules||'')!==(draft.rules||'')){await command.submit({operation:'update',id:actual.id,body:{kind:'world',name:draft.name,body:{background:draft.background||'',rules:draft.rules||''},expectedVersion:actual.draft_version||actual.published_version,reason:'完善初始化世界底稿'}});actual=await client.studioResource(draft.resourceId);}
  if(actual.published_version&&actual.published_version===actual.draft_version){draft.resourceVersion=actual.published_version;save();}
  else await command.submit({operation:'publish',id:actual.id,revision:actual.revision});
  if(live()){toast('世界底稿已发布，继续创建人物','ok');choose(draft.resourceId,draft.resourceVersion);}
 }catch(error){warning=error.message+'。已保存底稿与原提交保持，可以继续核对。';}finally{busy=false;if(live())paint();}}
 async function load(){const results=await Promise.allSettled([configurationClient.read(),client.listStudioResources({kind:'world',published:true})]);if(!live())return;
  if(results[0].status==='fulfilled')catalog=results[0].value;else warning=results[0].reason.message;
  if(results[1].status==='fulfilled')worlds=results[1].value.items;else warning+=(warning?'；':'')+results[1].reason.message;
  paint();}
 paint();void load();return()=>{alive=false;};
}
