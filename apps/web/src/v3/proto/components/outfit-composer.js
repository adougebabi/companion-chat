import {inventoryClient} from '../services.js';
import {get,subscribe} from '../store.js';
import {esc,toast} from '../ui.js';
import {Field} from './forms.js';
import {governanceCommand} from './governance-command.js';
import {studioDraftStorage} from './studio-draft-storage.js';
import {clothingTypeOptions,clothingTypeOf,clothingSeasons} from '../../../lib/wardrobe-form.ts';

const drafts=new Map();subscribe(()=>{if(!get('session'))drafts.clear();});
export function mountOutfitComposer(box,{fid,outfitId='',alive=()=>box.isConnected,onSaved=()=>{},navigate}){
 let active=true,epoch=0;const live=()=>active&&alive();
 let key;
 async function load(){
  if(!live())return;const version=++epoch;box.innerHTML='<div class="card" role="status">正在读取可搭配的服装与方案…</div>';
  try{
   const [catalogue,library,tagState,source]=await Promise.all([inventoryClient.clothingCatalogue(fid),inventoryClient.outfits(fid),inventoryClient.tags(fid),outfitId?inventoryClient.outfit(fid,outfitId):null]);
   if(!live()||epoch!==version)return;
   const updating=source?.profile_id===library.profile_id?source:null;
   key=`fluctlight-wardrobe:${get('session')?.actorId}:composer:${JSON.stringify([fid,library.profile_id,outfitId||'new'])}`;
   if(!drafts.has(key)&&studioDraftStorage().getItem(key))drafts.set(key,JSON.parse(studioDraftStorage().getItem(key)));
   if(!drafts.has(key))drafts.set(key,{name:source?.name||'',tags:[...(source?.tags||[])],ids:(source?.items||[]).map(i=>i.id),base:catalogue.revision,outfitRevision:updating?.revision||0,profileId:library.profile_id});
   const draft=drafts.get(key),items=new Map([...catalogue.items,...(source?.items||[])].map(i=>[i.id,i]));
   const selected=()=>draft.ids.map(id=>items.get(id)).filter(Boolean);
   const persist=()=>studioDraftStorage().setItem(key,JSON.stringify(draft));
   function paint(){persist();
    box.innerHTML=`<form class="card stack" data-compose><div><h2>${updating?'编辑搭配':'为她搭配'}</h2><p class="small muted">保存方案后，由她根据情境选择穿着。此处的预览不改变当前穿着。</p></div>${source&&!updating?'<p class="banner info">这是共享方案；本次会另存为当前人格的方案。</p>':''}${Field({label:'方案名称',name:'outfitName',value:draft.name,attrs:{required:true,maxlength:128}})}<fieldset class="outfit-tags"><legend>适用场景</legend><div class="row">${[...new Set([...tagState.tags,...draft.tags])].map(t=>`<label class="chip"><input type="checkbox" data-tag="${esc(t)}" ${draft.tags.includes(t)?'checked':''}>${esc(t)}</label>`).join('')||'<span class="small muted">还没有标签，可先到方案库维护。</span>'}</div><a class="small" href="#/c/${encodeURIComponent(fid)}/life/wardrobe?tab=plans">维护场景标签</a></fieldset><div class="outfit-slot-grid">${clothingTypeOptions.map(type=>{
     const options=catalogue.items.filter(i=>clothingTypeOf(i)===type.value),picked=selected().filter(i=>clothingTypeOf(i)===type.value);
     return `<section class="outfit-slot"><h3>${type.label}${type.value==='accessory'?'<span class="small muted"> · 可多选</span>':''}</h3>${type.value==='accessory'?`<div class="stack">${[...new Map([...options,...picked].map(i=>[i.id,i])).values()].map(i=>`<label class="outfit-choice"><input type="checkbox" data-accessory="${esc(i.id)}" ${draft.ids.includes(i.id)?'checked':''}><span>${esc(i.description)}${i.archived_at?'（登记已删除，可取消勾选）':''}<small>${(i.seasons||[]).map(s=>clothingSeasons.find(v=>v.value===s)?.label).filter(Boolean).join(' / ')||'季节未标注'}</small></span></label>`).join('')||'<p class="small muted">还没有配饰。</p>'}</div>`:Field({label:'选择'+type.label,name:'slot-'+type.value,type:'select',value:picked[0]?.id||'',options:[{value:'',label:'本方案不选择'},...[...new Map([...options,...picked].map(i=>[i.id,i])).values()].map(i=>({value:i.id,label:i.description+(i.archived_at?'（登记已删除）':'')}))],attrs:{'data-slot':type.value}})}${picked.map(i=>`<p class="small muted">${esc(i.description)}</p>`).join('')}</section>`;
    }).join('')}</div>${selected().some(i=>!clothingTypeOptions.some(t=>t.value===clothingTypeOf(i)))?`<section><h3>保留的原有服装</h3>${selected().filter(i=>!clothingTypeOptions.some(t=>t.value===clothingTypeOf(i))).map(i=>`<label class="outfit-choice"><input type="checkbox" data-legacy="${esc(i.id)}" checked>${esc(i.description)}</label>`).join('')}</section>`:''}<p class="small muted">${draft.ids.length} 件服装 · ${esc(get('characters',fid)?.name||'当前角色')}。缺少某类服装可在“全部衣服”中补录。</p><div class="row"><button class="btn primary">保存方案</button><button type="button" class="btn" data-new>新建搭配</button></div><p data-error role="alert"></p></form>`;
    box.querySelector('[name=outfitName]').oninput=e=>{draft.name=e.target.value;persist();};
    box.querySelectorAll('[data-tag]').forEach(c=>c.onchange=()=>{draft.tags=[...box.querySelectorAll('[data-tag]:checked')].map(c=>c.dataset.tag);persist();});
    box.querySelectorAll('[data-slot]').forEach(c=>c.onchange=()=>{const type=c.dataset.slot;draft.ids=draft.ids.filter(id=>clothingTypeOf(items.get(id)||{})!==type);if(c.value)draft.ids.push(c.value);paint();});
    box.querySelectorAll('[data-accessory],[data-legacy]').forEach(c=>c.onchange=()=>{const id=c.dataset.accessory||c.dataset.legacy;draft.ids=draft.ids.filter(v=>v!==id);if(c.checked)draft.ids.push(id);paint();});
    box.querySelector('[data-new]').onclick=()=>{if(!outfitId){drafts.delete(key);studioDraftStorage().removeItem(key);void load();}else navigate(`#/c/${encodeURIComponent(fid)}/life/wardrobe?tab=compose`);};
    box.querySelector('form').onsubmit=e=>{e.preventDefault();if(!e.target.reportValidity())return;if(!draft.ids.length){box.querySelector('[data-error]').textContent='请先选一件服装再保存。';return;}
     const captured=structuredClone(draft),target={name:captured.name.trim(),tags:captured.tags,itemIds:captured.ids,profileId:captured.profileId,expectedOutfitRevision:captured.outfitRevision};
     governanceCommand({persistent:true,scope:[fid,captured.profileId,'outfit-composer',updating?.id||'new'],title:'保存穿搭方案',defaultReason:'保存穿搭方案',note:'只保存方案；实际穿着仍由她按当前情境决定。',revision:captured.base,snapshot:source,target,
      read:async()=>{const [current,plans]=await Promise.all([inventoryClient.current(fid),inventoryClient.outfits(fid)]);return {...current,profileId:plans.profile_id,...(updating?{outfit:await inventoryClient.outfit(fid,updating.id)}:{})};},
      rebaseTarget:(latest,value)=>{if(latest.profileId!==value.profileId)throw new Error('当前人格已变化。原人格草稿已保留，请切回原人格继续，或在当前人格新建方案。');return {...value,expectedOutfitRevision:latest.outfit?.revision||0};},
      execute:(body,value)=>inventoryClient.saveOutfit(fid,updating?.id||null,{...value,expectedRevision:body.expectedRevision,reason:body.reason,idempotencyKey:body.idempotencyKey}),
      onSaved:async(result,submitted)=>{const latest=drafts.get(key);if(latest&&JSON.stringify({name:latest.name.trim(),tags:latest.tags,itemIds:latest.ids,profileId:latest.profileId,expectedOutfitRevision:submitted.expectedOutfitRevision})===JSON.stringify(submitted)){drafts.delete(key);studioDraftStorage().removeItem(key);}toast('方案已保存');if(live())await onSaved(result);},
     });
    };
   }paint();
  }catch(error){if(live()&&epoch===version){box.innerHTML=`<div class="statebox" role="alert"><p>${esc(error.message)}</p><button class="btn" data-retry>重新读取</button></div>`;box.querySelector('[data-retry]').onclick=()=>void load();}}
 }void load();return {dispose(){active=false;epoch++;}};
}
