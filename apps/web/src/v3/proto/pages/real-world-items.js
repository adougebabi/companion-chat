import {get} from '../store.js';
import {mockWorld} from '../services.js';
import {asyncPage} from '../components/async-page.js';
import {mountCursorRecords} from '../components/cursor-records.js';
import {Field,SearchForm} from '../components/forms.js';
import {WorldItemCard,WorldItemSource,WorldItemSuggestion} from '../components/world-items.js';
import {governanceCommand} from '../components/governance-command.js';
import {esc} from '../ui.js';

export function renderWorldItems(el,ctx){
 const fid=ctx.charId,owner=get('session')?.actorId;let active=true,term=ctx.query.q||'';
 const alive=()=>active&&el.isConnected&&get('session')?.actorId===owner;
 el.innerHTML=`<div class="card"><h2>世界物品目录</h2><p>查看当前运行世界中已注册的物品定义。浏览、配置和提出请求不会取得、穿着或使用物品；实际结果可在衣柜与行动来源中查询。</p><a class="btn" href="#/studio/edit/new?kind=item">新建物品定义</a><a class="btn" href="#/c/${encodeURIComponent(fid)}/world">返回世界总览</a><div data-search>${SearchForm({value:term,placeholder:'搜索目录物品'})}</div></div><div data-items class="stack"></div>`;
 const panel=mountCursorRecords(el.querySelector('[data-items]'),{alive,label:'世界物品',read:cursor=>mockWorld.items(fid,{cursor,q:term}),render:item=>WorldItemCard(item,fid),empty:'当前世界尚未注册物品。可在创作中发布物品定义，再明确应用到此角色的运行世界。'});
 el.querySelector('[data-search] form').onsubmit=e=>{e.preventDefault();term=e.target.elements.q.value.trim();void panel.reload({reset:true});};return()=>{active=false;panel.dispose();};
}
export function renderWorldItem(el,ctx){
 const fid=ctx.charId,id=ctx.params.itemId;
 return asyncPage(el,()=>mockWorld.item(fid,id),(item,reload,alive)=>{
  const retired=get('characters',fid)?.status==='已退休';
  el.innerHTML=`<div class="card"><h2>${esc(item.name)}</h2><p>${esc(item.definition.description)}</p><p class="small muted">目录 ${esc(item.id)} · v${esc(item.revision)} · 世界 ${esc(item.world_id)}</p>${WorldItemSource(item)}<p>当前${item.available?'允许提出取得请求':'不可取得'}。${item.place_id?'范围限定地点 '+esc(item.place_id):'取得地点不限'}。</p><a class="btn" href="#/c/${encodeURIComponent(fid)}/world/items">返回物品目录</a><a class="btn" href="#/c/${encodeURIComponent(fid)}/life/wardrobe">查看实际衣柜</a>${item.available&&!retired?`<a class="btn primary" href="#/c/${encodeURIComponent(fid)}/chat?say=${encodeURIComponent(WorldItemSuggestion(item))}">在对话中提出取得请求</a>`:''}<p class="small muted">这会准备一段待发送的请求，由你发送。角色可拒绝、延期；任务开始和完成取得分别显示，不把本目录当库存。</p></div><form class="card stack" data-config><h3>取得范围</h3>${Field({label:'是否可取得',name:'available',value:item.available?'true':'false',type:'select',options:[{value:'true',label:'可提出取得请求'},{value:'false',label:'当前不可取得'}]})}${Field({label:'取得地点 ID（留空不限地点）',name:'placeId',value:item.place_id||'',attrs:{maxlength:128}})}<p class="small muted">此配置对新开始的取得行动生效。已经开始的行动保留当时的目录快照，并由实际结果决定是否取得。</p><button class="btn" ${retired?'disabled':''}>核对并保存取得范围</button></form>`;
  const form=el.querySelector('[data-config]');form.onsubmit=e=>{e.preventDefault();if(retired||!form.reportValidity())return;const target={available:form.elements.available.value==='true',placeId:form.elements.placeId.value.trim()||null};governanceCommand({scope:[fid,'world-item',id],title:'保存取得范围',maxReason:1000,note:'不会添加或删除角色库存，不影响已经开始的行动。',revision:item.revision,snapshot:item,target,read:()=>mockWorld.item(fid,id),rebaseTarget:()=>target,execute:(body,target)=>mockWorld.configureItem(fid,id,{...body,...target}),onSaved:()=>{if(alive())return reload();}});};
 });
}
