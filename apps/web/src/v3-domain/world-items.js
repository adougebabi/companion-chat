import {get,list} from '../v3/proto/store.js';
import {mockWorld} from '../v3/proto/services.js';
import {asyncPage} from '../v3/proto/components/async-page.js';
import {mountCursorRecords} from '../v3/proto/components/cursor-records.js';
import {Button,Field,SearchForm} from '../v3/proto/components/forms.js';
import {WorldItemCard,WorldItemDownload,WorldItemPlaceName,WorldItemSource,WorldItemSuggestion,WorldItemSummary} from '../v3/proto/components/world-items.js';
import {governanceCommand} from '../v3/proto/components/governance-command.js';
import {MissingWorld} from '../v3/proto/components/world-presentation.js';

async function readPlaces(fid, worldId) {
 if(!worldId)return {places:[],failed:false};
 try{return {places:await mockWorld.places(fid,worldId),failed:false};}
 catch{return {places:list('places',place=>(place.worldId??place.world_id)===worldId),failed:true};}
}

export function renderWorldItems(el,ctx){
 const fid=ctx.charId,owner=get('session')?.actorId;let active=true,term=ctx.query.q||'';
 const alive=()=>active&&el.isConnected&&get('session')?.actorId===owner;
 el.innerHTML=`<div data-domain-ui="card" class="card"><h2>世界物品目录</h2><p>查看当前运行世界中已注册的物品定义。浏览、配置和提出请求不会取得、穿着或使用物品；实际结果可在衣柜与行动来源中查询。</p><div class="row"><button data-create data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" type="button" disabled>新建物品定义</button><a data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" href="#/c/${encodeURIComponent(fid)}/world">返回世界总览</a></div><div data-search>${SearchForm({value:term,placeholder:'搜索目录物品',inputAttrs:{disabled:true}})}</div><div data-items class="stack"></div></div>`;
 const box=el.querySelector('[data-items]'),create=el.querySelector('[data-create]'),form=el.querySelector('[data-search] form');
 let ready=false;
 const setReady=value=>{ready=value;for(const control of [create,...form.elements])control.disabled=!value;};
 setReady(false);
 create.onclick=()=>{if(ready)location.hash='#/studio/edit/new?kind=item';};
 const panel=mountCursorRecords(box,{
  alive,label:'世界物品',
  read:async cursor=>{
   setReady(false);
   const catalog=await mockWorld.catalog(fid);
   if(!alive())return {items:[]};
   const worldId=catalog.active_binding?.world_id;
   if(!worldId)return {items:[],missingWorld:true};
   const page=await mockWorld.items(fid,{cursor,q:term});
   if(!alive())return {items:[]};
   const pageWorldId=page.world_id||worldId;
   const world=catalog.items?.find(entry=>entry.id===pageWorldId)||get('worlds',pageWorldId);
   const {places}=page.items.some(item=>item.place_id)?await readPlaces(fid,pageWorldId):{places:[]};
   const placesById=new Map(places.map(place=>[place.id,place]));
   return {...page,items:page.items.map(item=>{
    const inWorld=!item.world_id||item.world_id===pageWorldId;
    return {item,world:inWorld?world:undefined,place:inWorld?placesById.get(item.place_id):undefined};
   })};
  },
  bind:(_,page)=>{if(page.missingWorld)el.innerHTML=MissingWorld(fid);else setReady(true);},
  onError:()=>{setReady(false);box.replaceChildren();},
  renderFailure:()=>`<div data-panel-failure data-domain-ui="alert" data-feedback-kind="error" class="stack" role="alert"><p>暂时无法读取世界物品目录，请稍后再试。</p><button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" type="button" data-panel-retry>重新读取</button></div>`,
  render:({item,world,place})=>WorldItemCard(item,fid,{world,place}),empty:'当前世界尚未注册物品。可在创作中发布物品定义，再明确应用到此角色的运行世界。'
 });
 form.onsubmit=e=>{e.preventDefault();if(!ready)return;term=form.elements.q.value.trim();void panel.reload({reset:true});};return()=>{active=false;panel.dispose();};
}
export function renderWorldItem(el,ctx){
 const fid=ctx.charId,id=ctx.params.itemId;
 return asyncPage(el,async()=>{
  const item=await mockWorld.item(fid,id);
  const {places,failed}=await readPlaces(fid,item.world_id);
  return {item,places,failed,world:item.world_id?get('worlds',item.world_id):undefined};
 },({item,places,failed,world},reload,alive)=>{
  const retired=get('characters',fid)?.status==='已退休';
  const place=places.find(entry=>entry.id===item.place_id);
  const placeOptions=places.map(entry=>({value:entry.id,label:WorldItemPlaceName(entry)})).filter(option=>option.label);
  if(item.place_id&&!placeOptions.some(option=>option.value===item.place_id)){
   placeOptions.unshift({value:item.place_id,label:'保留当前限定地点'});
  }
  placeOptions.unshift({value:'',label:'不限地点'});
  const base=`#/c/${encodeURIComponent(fid)}`;
  el.innerHTML=`<div data-domain-ui="card" class="card">
    ${WorldItemSummary(item,{world,place})}
    ${WorldItemSource(item)}
    ${WorldItemDownload(item)}
    <a data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" href="${base}/world/items">返回物品目录</a>
    <a data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" href="${base}/life/wardrobe">查看实际衣柜</a>
    ${item.available&&!retired?`<a data-domain-ui="button" data-domain-variant="default" data-domain-size="default" href="${base}/chat?say=${encodeURIComponent(WorldItemSuggestion(item))}">在对话中提出取得请求</a>
      <p class="small muted">这会准备一段待发送的请求，由你发送。角色可拒绝、延期；任务开始和完成取得分别显示，不把本目录当库存。</p>`:''}
  </div>
  <form data-domain-ui="card" class="card stack" data-config>
    <h3>取得范围</h3>
    ${Field({label:'是否可取得',name:'available',value:item.available?'true':'false',type:'select',options:[{value:'true',label:'可提出取得请求'},{value:'false',label:'当前不可取得'}]})}
    ${Field({label:'取得地点',name:'placeId',value:item.place_id||'',type:'select',options:placeOptions})}
    ${failed?`<p class="small muted">暂时无法读取完整地点列表。</p>${Button({label:'重新读取地点',attrs:{'data-place-retry':true}})}`:''}
    <p class="small muted">此配置对新开始的取得行动生效。已经开始的行动保留当时的目录快照，并由实际结果决定是否取得。</p>
    ${Button({label:'核对并保存取得范围',type:'submit',attrs:{disabled:retired}})}
  </form>`;
  const retry=el.querySelector('[data-place-retry]');if(retry)retry.onclick=reload;
  const form=el.querySelector('[data-config]');form.onsubmit=e=>{e.preventDefault();if(retired||!form.reportValidity())return;const target={available:form.elements.available.value==='true',placeId:form.elements.placeId.value.trim()||null};governanceCommand({scope:[fid,'world-item',id],title:'保存取得范围',maxReason:1000,note:'不会添加或删除角色库存，不影响已经开始的行动。',revision:item.revision,snapshot:item,target,read:()=>mockWorld.item(fid,id),rebaseTarget:()=>target,execute:(body,target)=>mockWorld.configureItem(fid,id,{...body,...target}),onSaved:()=>{if(alive())return reload();}});};
 });
}
