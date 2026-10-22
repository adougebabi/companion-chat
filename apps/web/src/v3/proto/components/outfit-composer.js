import {inventoryClient} from '../services.js';
import {get,subscribe} from '../store.js';
import {esc,toast,confirm} from '../ui.js';
import {Field} from './forms.js';
import {governanceCommand} from './governance-command.js';
import {studioDraftStorage} from './studio-draft-storage.js';
import {clothingTypeOptions,clothingTypeOf,clothingSeasons} from '../../../lib/wardrobe-form.ts';

const drafts=new Map();subscribe(()=>{if(!get('session'))drafts.clear();});
export function mountOutfitComposer(box,{fid,outfitId='',alive=()=>box.isConnected,onSaved=()=>{},navigate}){
 let active=true,epoch=0;const live=()=>active&&alive();
 let key,expandedSlot='',seasonFilter=[];
 const whole=item=>item?.slot==='full_body';
 const typeOf=item=>whole(item)?'lower_body':clothingTypeOf(item);
 const selectable=item=>!item.archived_at&&item.availability==='available'&&item.source_verified&&['owned','borrowed'].includes(item.ownership);
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
   draft.perTagDrafts??={};draft.selectedTag??='';
   const focusSlot=type=>box.querySelector(`[data-slot-toggle="${type}"]`)?.focus();
   const persist=()=>studioDraftStorage().setItem(key,JSON.stringify(draft));
   function paint(){persist();
    box.innerHTML=`<form class="stack outfit-composition" data-compose><nav class="outfit-scenes row" aria-label="场景搭配草稿">${['',...tagState.tags].map(t=>`<button type="button" class="chip ${draft.selectedTag===t?'on':''}" data-scene-draft="${esc(t)}" aria-pressed="${draft.selectedTag===t}">${esc(t||'未标记场景')}</button>`).join('')}</nav><div><h2>${updating?'编辑搭配':'为她搭配'}</h2><p class="small muted">保存方案后，由她根据情境选择穿着。此处的预览不改变当前穿着。</p></div>${source&&!updating?'<p class="banner info">这是共享方案；本次会另存为当前人格的方案。</p>':''}${Field({label:'方案名称',name:'outfitName',value:draft.name,attrs:{required:true,maxlength:128}})}<details class="mgmt"><summary>方案适用场景</summary><fieldset class="outfit-tags"><legend>适用场景</legend><div class="row">${[...new Set([...tagState.tags,...draft.tags])].map(t=>`<label class="chip"><input type="checkbox" data-tag="${esc(t)}" ${draft.tags.includes(t)?'checked':''}>${esc(t)}</label>`).join('')||'<span class="small muted">还没有标签，可先到方案库维护。</span>'}</div><a class="small" href="#/c/${encodeURIComponent(fid)}/life/wardrobe?tab=plans">维护场景标签</a></fieldset></details><div class="outfit-slot-grid">${clothingTypeOptions.map(type=>{
     const options=[...new Map([...catalogue.items.filter(i=>typeOf(i)===type.value&&selectable(i)&&(!seasonFilter.length||(i.seasons||[]).some(s=>seasonFilter.includes(s)))),...selected().filter(i=>typeOf(i)===type.value&&!selectable(i))].map(i=>[i.id,i])).values()];
     const picked=selected().filter(i=>typeOf(i)===type.value),covered=type.value==='upper_body'&&selected().some(whole),expanded=expandedSlot===type.value;
     return `<section class="outfit-slot ${expanded?'is-expanded':''}"><button type="button" class="outfit-slot-trigger" data-slot-toggle="${type.value}" aria-expanded="${expanded}"><span><b>${type.label}</b><small>${covered?'由整身服装覆盖':esc(picked.map(i=>i.description).join('、')||'尚未选择')}</small></span><span aria-hidden="true">${expanded?'⌃':'⌄'}</span></button>${expanded?`<div class="outfit-candidates" data-candidates="${type.value}"><div class="row"><b class="grow small">${type.value==='accessory'?'可连续选择多件':'选择后立即加入搭配'}</b><button type="button" class="btn ghost sm" data-slot-close="${type.value}">收起</button></div><div class="row" aria-label="季节筛选">${clothingSeasons.map(s=>`<button type="button" class="chip ${seasonFilter.includes(s.value)?'on':''}" data-candidate-season="${s.value}" aria-pressed="${seasonFilter.includes(s.value)}">${s.label}</button>`).join('')}</div><div class="row outfit-candidate-list"><button type="button" class="chip" data-pick="" data-pick-type="${type.value}">${type.value==='accessory'?'不佩戴':'不搭配'}</button>${options.map(i=>`<button type="button" class="chip ${draft.ids.includes(i.id)?'on':''}" data-pick="${esc(i.id)}" data-pick-type="${type.value}" aria-pressed="${draft.ids.includes(i.id)}">${draft.ids.includes(i.id)?'✓ ':''}${esc(i.description)}${whole(i)?' · 整身':''}</button>`).join('')}</div>${!options.length?'<p class="small muted">当前筛选没有可用衣物；已选搭配仍保留。</p>':''}</div>`:''}</section>`;
    }).join('')}</div>${selected().some(i=>!clothingTypeOptions.some(t=>t.value===typeOf(i)))?`<section><h3>保留的原有服装</h3>${selected().filter(i=>!clothingTypeOptions.some(t=>t.value===typeOf(i))).map(i=>`<label class="outfit-choice"><input type="checkbox" data-legacy="${esc(i.id)}" checked>${esc(i.description)}</label>`).join('')}</section>`:''}<p class="small muted">${draft.ids.length} 件服装 · ${esc(get('characters',fid)?.name||'当前角色')}。缺少某类服装可在“全部衣服”中补录。</p><div class="row"><button class="btn primary">保存方案</button><button type="button" class="btn" data-new>新建搭配</button></div><p data-error role="alert"></p></form>`;
    box.querySelector('[name=outfitName]').oninput=e=>{draft.name=e.target.value;persist();};
    box.querySelectorAll('[data-tag]').forEach(c=>c.onchange=()=>{draft.tags=[...box.querySelectorAll('[data-tag]:checked')].map(c=>c.dataset.tag);persist();});
    box.querySelectorAll('[data-slot-toggle]').forEach(button=>button.onclick=()=>{expandedSlot=expandedSlot===button.dataset.slotToggle?'':button.dataset.slotToggle;paint();focusSlot(button.dataset.slotToggle);});
    box.querySelectorAll('[data-slot-close]').forEach(button=>button.onclick=()=>{expandedSlot='';paint();focusSlot(button.dataset.slotClose);});
    box.querySelectorAll('[data-candidate-season]').forEach(button=>button.onclick=()=>{const season=button.dataset.candidateSeason;seasonFilter=seasonFilter.includes(season)?seasonFilter.filter(s=>s!==season):[...seasonFilter,season];paint();box.querySelector(`[data-candidate-season="${season}"]`)?.focus();});
    box.querySelectorAll('[data-scene-draft]').forEach(button=>button.onclick=()=>{draft.perTagDrafts[draft.selectedTag]={name:draft.name,tags:[...draft.tags],ids:[...draft.ids]};draft.selectedTag=button.dataset.sceneDraft;const prior=draft.perTagDrafts[draft.selectedTag]||(source?{name:source.name,tags:[...(source.tags||[])],ids:(source.items||[]).map(i=>i.id)}:{name:'',tags:draft.selectedTag?[draft.selectedTag]:[],ids:[]});Object.assign(draft,structuredClone(prior));expandedSlot='';paint();box.querySelector(`[data-scene-draft="${CSS.escape(draft.selectedTag)}"]`)?.focus();});
    box.querySelectorAll('[data-pick]').forEach(button=>button.onclick=async()=>{
      const type=button.dataset.pickType,id=button.dataset.pick,item=items.get(id),tag=draft.selectedTag;
      if(id&&!selectable(item)&&!draft.ids.includes(id))return;
      if(whole(item)&&!(await confirm({title:'使用这件整身服装',text:'这会替换当前上衣与下衣的选择。其它部位保留，只改变搭配草稿。',okText:'替换上下衣'})))return;
      if(!live()||draft.selectedTag!==tag)return;
      if(type==='accessory'){if(!id)draft.ids=draft.ids.filter(i=>typeOf(items.get(i)||{})!==type);else draft.ids=draft.ids.includes(id)?draft.ids.filter(i=>i!==id):[...draft.ids,id];}
      else{draft.ids=draft.ids.filter(i=>{const old=items.get(i)||{};return typeOf(old)!==type&&!(whole(item)&&typeOf(old)==='upper_body')&&!(type==='upper_body'&&whole(old));});if(id)draft.ids.push(id);expandedSlot='';}
      paint();if(type==='accessory')box.querySelector(`[data-pick="${CSS.escape(id)}"]`)?.focus();else focusSlot(type);
    });
    box.querySelector('form').onkeydown=event=>{if(event.key==='Escape'&&expandedSlot){event.preventDefault();const type=expandedSlot;expandedSlot='';paint();focusSlot(type);}};
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
