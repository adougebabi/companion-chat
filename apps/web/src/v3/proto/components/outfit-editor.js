import {inventoryClient} from '../services.js';
import {get,subscribe} from '../store.js';
import {Field} from './forms.js';
import {JsonDetails} from './records.js';
import {governanceCommand} from './governance-command.js';
import {drawer,esc} from '../ui.js';
const drafts=new Map();subscribe(()=>{if(!get('session'))drafts.clear();});
// Preparation is a local draft. The shared governance command owns submission
// lifetime/unknown-result retry, while the service owns the catalogue versions.
export function outfitEditor(fid,{outfit=null,profileId,revision,itemIds=[]},onSaved){
 if(get('characters',fid)?.status==='已退休')return;
 const existing=outfit?.profile_id?outfit:null,key=JSON.stringify([get('session')?.actorId,fid,existing?.id||'new']);
 if(!drafts.has(key))drafts.set(key,{name:outfit?.name||'',ids:(outfit?outfit.items.map(i=>i.id):itemIds).join('\n'),profileId,base:revision,outfitRevision:existing?.revision||0});
 const draft=drafts.get(key),snapshot=()=>({name:draft.name,ids:draft.ids,profileId:draft.profileId,base:draft.base,outfitRevision:draft.outfitRevision});
 const d=drawer({title:existing?'编辑已保存穿搭':outfit?'保存为当前人格新穿搭':'保存当前穿搭',body:`<p>保存的是现有服饰组合，不会自动穿上、取得物品或改写生活事实。</p><p class="small muted">套装归属：${esc(draft.profileId)}</p><form class="stack">${Field({label:'套装名称',name:'name',value:draft.name,attrs:{required:true,maxlength:128}})}${Field({label:'组成物品ID（一行一个，1—16件服饰）',name:'ids',value:draft.ids,type:'textarea',rows:8,attrs:{required:true,maxlength:2064}})}<p data-domain-ui="alert" role="alert" data-error></p></form>`,footer:'<button data-domain-ui="button" class="btn" data-close>关闭</button><button data-domain-ui="button" class="btn primary" data-next>核对组合并填写理由</button>'});
 d.el.querySelector('[name=name]').oninput=e=>draft.name=e.target.value;d.el.querySelector('[name=ids]').oninput=e=>draft.ids=e.target.value;d.el.querySelector('[data-close]').onclick=()=>d.close();
 d.el.querySelector('[data-next]').onclick=()=>{if(!d.el.querySelector('form').reportValidity())return;const ids=draft.ids.split('\n').map(id=>id.trim()).filter(Boolean);if(ids.length<1||ids.length>16||new Set(ids).size!==ids.length){d.el.querySelector('[data-error]').textContent='请选择1—16个不同的真实服饰ID。';return;}
  const captured=snapshot();const target={name:captured.name,itemIds:ids,profileId:captured.profileId,expectedOutfitRevision:captured.outfitRevision};d.close();
  governanceCommand({scope:[fid,'outfit-save',existing?.id||'new',target],title:'保存套装定义',revision:captured.base,note:'这次只保存当前人格的穿搭定义；实际换装另行请求。会校验衣柜、套装与人格范围，并保留修订。',snapshot:target,target,
   read:async()=>{const [current,page]=await Promise.all([inventoryClient.current(fid),inventoryClient.outfits(fid)]);return {...current,profileId:page.profile_id,...(existing?{outfit:await inventoryClient.outfit(fid,existing.id)}:{})};},
   rebaseTarget:(latest,target)=>({...target,profileId:latest.profileId,expectedOutfitRevision:latest.outfit?.revision||0}),
   execute:(body,target)=>inventoryClient.saveOutfit(fid,existing?.id||null,{...target,expectedRevision:body.expectedRevision,reason:body.reason}),
   onSaved:async result=>{if(drafts.get(key)===draft&&JSON.stringify(snapshot())===JSON.stringify(captured))drafts.delete(key);await onSaved(result);},
  });
 };return d;
}
