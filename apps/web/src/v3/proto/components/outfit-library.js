import {inventoryClient} from '../services.js';
import {esc,drawer,toast} from '../ui.js';
import {Field} from './forms.js';
import {OutfitPreview} from './outfit-preview.js';
import {governanceCommand} from './governance-command.js';
import {wardrobeDraft} from './wardrobe-drafts.js';
import {requestInventoryAction} from './inventory-actions.js';

export function mountOutfitLibrary(box,{fid,alive=()=>box.isConnected,navigate,onSaved=()=>{}}){
 let active=true,epoch=0,filter='';const live=()=>active&&alive();
 async function load(){
  if(!live())return;const version=++epoch;box.innerHTML='<div class="card" role="status">正在读取方案库…</div>';
  try{
   const [tagState,current]=await Promise.all([inventoryClient.tags(fid),inventoryClient.current(fid)]);
   let cursor='',profileId;const plans=[],seen=new Set();
   do{const page=await inventoryClient.outfits(fid,cursor);if(profileId&&profileId!==page.profile_id)throw new Error('当前人格已变化，请重新读取方案库。');profileId=page.profile_id;plans.push(...page.items);cursor=page.next_cursor;if(cursor&&seen.has(cursor))throw new Error('方案分页重复，请重新读取。');seen.add(cursor);}while(cursor);
   if(!live()||epoch!==version)return;
   function paint(){
    const visible=plans.filter(p=>!filter||(p.tags||[]).includes(filter));
    box.innerHTML=`<div class="stack"><section class="card stack"><div class="row"><div class="grow"><h2>方案库</h2><p class="small muted">同一场景可有多套搭配，每套方案也可适用于多个场景。</p></div><button class="btn" data-tags>维护场景标签</button><a class="btn primary" href="#/c/${encodeURIComponent(fid)}/life/wardrobe?tab=compose">新建搭配</a></div><div class="row"><button class="chip ${filter?'':'on'}" data-filter="">全部方案</button>${tagState.tags.map(t=>`<button class="chip ${filter===t?'on':''}" data-filter="${esc(t)}">${esc(t)}</button>`).join('')}</div></section><div class="grid g2">${visible.map(p=>`<section class="card stack">${OutfitPreview(p)}<div class="row"><button class="btn" data-preview="${esc(p.id)}">预览</button><button class="btn" data-edit="${esc(p.id)}">编辑搭配</button><button class="btn ghost" data-rename="${esc(p.id)}">重命名</button><button class="btn primary" data-wear="${esc(p.id)}">请她换上</button></div></section>`).join('')||'<div class="statebox"><h3>还没有这个场景的方案</h3><p>先为她搭配一套服装，保存后会出现在这里。</p></div>'}</div></div>`;
    box.querySelectorAll('[data-filter]').forEach(b=>b.onclick=()=>{filter=b.dataset.filter;paint();});
    box.querySelector('[data-tags]').onclick=()=>tagEditor(fid,profileId,tagState,()=>void load());
    box.querySelectorAll('[data-preview]').forEach(b=>b.onclick=()=>drawer({title:'穿搭预览',body:OutfitPreview(plans.find(p=>p.id===b.dataset.preview))}));
    box.querySelectorAll('[data-edit]').forEach(b=>b.onclick=()=>navigate(`#/c/${encodeURIComponent(fid)}/life/wardrobe?tab=compose&outfit=${encodeURIComponent(b.dataset.edit)}`));
    box.querySelectorAll('[data-rename]').forEach(b=>b.onclick=()=>renameOutfit(fid,plans.find(p=>p.id===b.dataset.rename),profileId,current.revision,()=>void load()));
    box.querySelectorAll('[data-wear]').forEach(b=>b.onclick=async()=>{b.disabled=true;try{const [latest,p]=await Promise.all([inventoryClient.current(fid),inventoryClient.outfit(fid,b.dataset.wear)]);if(!live()||epoch!==version)return;if(p.active_profile_id!==profileId)throw new Error('当前人格已变化，请重新读取方案库后选择。');requestInventoryAction(fid,'outfit',{outfitId:p.id,expectedOutfitRevision:p.revision,profileId},latest.revision,async()=>{await onSaved();if(live())await load();},live,p);}catch(error){if(live())toast(error.message,'err');}finally{if(b.isConnected)b.disabled=false;}});
   }paint();
  }catch(error){if(live()&&epoch===version){box.innerHTML=`<div class="statebox" role="alert"><p>${esc(error.message)}</p><button class="btn" data-retry>重新读取</button></div>`;box.querySelector('[data-retry]').onclick=()=>void load();}}
 }void load();return {dispose(){active=false;epoch++;}};
}

function renameOutfit(fid,outfit,profileId,revision,onSaved){
 const draft=wardrobeDraft([fid,profileId,'rename',outfit.id],{name:outfit.name});
 const d=drawer({title:'重命名方案',body:`${Field({label:'方案名称',name:'name',value:draft.get().name,attrs:{required:true,maxlength:128}})}<p role="alert" data-error></p>`,footer:'<button class="btn primary" data-save>保存名称</button>'});
 d.el.querySelector('[name=name]').oninput=e=>draft.set({name:e.target.value});
 d.el.querySelector('[data-save]').onclick=()=>{const field=d.el.querySelector('[name=name]');if(!field.reportValidity())return;
  const target={name:field.value.trim(),profileId,expectedOutfitRevision:outfit.profile_id?outfit.revision:0,...(!outfit.profile_id?{tags:outfit.tags||[],itemIds:outfit.items.map(i=>i.id)}:{})};
  const id=outfit.profile_id?outfit.id:null;d.close();governanceCommand({persistent:true,scope:[fid,profileId,'outfit-rename',outfit.id],title:id?'保存方案名称':'另存共享方案',defaultReason:'修改方案名称',revision,snapshot:outfit,target,note:'方案名称改变后，服装组合和当前穿着保持原样。',
   read:async()=>({...await inventoryClient.current(fid),outfit:await inventoryClient.outfit(fid,outfit.id)}),
   rebaseTarget:(latest,value)=>{if(latest.outfit.active_profile_id!==value.profileId)throw new Error('当前人格已变化，原草稿已保留。');return {...value,expectedOutfitRevision:id?latest.outfit.revision:0};},
   execute:(body,value)=>(id?inventoryClient.renameOutfit(fid,id,{...value,expectedRevision:body.expectedRevision,reason:body.reason,idempotencyKey:body.idempotencyKey}):inventoryClient.saveOutfit(fid,null,{...value,expectedRevision:body.expectedRevision,reason:body.reason,idempotencyKey:body.idempotencyKey})),onSaved:async(result,submitted)=>{draft.acknowledge({name:submitted.name});await onSaved(result);},
  });
 };
}

function tagEditor(fid,profileId,state,onSaved){
 const draft=wardrobeDraft([fid,profileId,'tags'],{rows:state.tags.map(name=>({original:name,name}))});
 const rows=draft.get().rows;const persist=()=>draft.set({rows});
 const d=drawer({title:'维护场景标签',body:'',footer:'<button class="btn" data-close>关闭</button><button class="btn primary" data-save>保存标签</button>'});
 function paint(){persist();d.body.innerHTML=`<p class="small muted">例如“在家”“外出”“上班”。改名会同步所有引用方案；移除标签会解除方案上的这个标签，保留方案与服装。</p><div class="stack">${rows.map((r,i)=>`<div class="row">${Field({label:r.original?'场景标签':'新增标签',name:'tag-'+i,value:r.name,attrs:{maxlength:40,'data-index':i}})}<button type="button" class="btn ghost danger" data-remove="${i}">移除</button></div>`).join('')}</div><button class="btn" data-add>添加标签</button><p role="alert" data-error></p>`;
  d.el.querySelectorAll('[data-index]').forEach(input=>input.oninput=()=>{rows[Number(input.dataset.index)].name=input.value;persist();});d.el.querySelectorAll('[data-remove]').forEach(b=>b.onclick=()=>{rows.splice(Number(b.dataset.remove),1);paint();});d.el.querySelector('[data-add]').onclick=()=>{rows.push({original:'',name:''});paint();};
 }paint();d.el.querySelector('[data-close]').onclick=()=>d.close();
 d.el.querySelector('[data-save]').onclick=()=>{const values=rows.map(r=>r.name.trim()).filter(Boolean);if(new Set(values).size!==values.length){d.el.querySelector('[data-error]').textContent='标签名称有重复，请保留一个。';return;}
  const submittedRows=structuredClone(rows);const target={tags:values,renames:Object.fromEntries(rows.filter(r=>r.original&&r.name.trim()&&r.name.trim()!==r.original).map(r=>[r.original,r.name.trim()]))};
  d.close();governanceCommand({persistent:true,scope:[fid,profileId,'wardrobe-tags'],title:'保存场景标签',defaultReason:'维护穿搭场景标签',revision:state.revision,snapshot:state,target,note:'这次只维护方案分类，不改变当前穿着。',read:()=>inventoryClient.tags(fid),execute:(body,value)=>inventoryClient.saveTags(fid,{...value,expectedRevision:body.expectedRevision,reason:body.reason,idempotencyKey:body.idempotencyKey}),onSaved:async(result,submitted)=>{const acknowledged={tags:submittedRows.map(r=>r.name.trim()).filter(Boolean),renames:Object.fromEntries(submittedRows.filter(r=>r.original&&r.name.trim()&&r.name.trim()!==r.original).map(r=>[r.original,r.name.trim()]))};if(JSON.stringify(acknowledged)===JSON.stringify(submitted))draft.acknowledge({rows:submittedRows});toast('标签已保存并同步方案');await onSaved();}});
 };
}
