import {LifeNavigation} from '../components/life-navigation.js';
import {openOwnerMediaDrawer} from '../components/media-generation-drawer.js';
import {lifeClient,inventoryClient} from '../services.js';
import {get} from '../store.js';
import {asyncPage,requestReceipt} from '../components/async-page.js';
import {InventoryCard,CurrentInventory,InventoryIdentity,InventoryHistory} from '../components/inventory-records.js';
import {requestInventoryAction} from '../components/inventory-actions.js';
import {mountWearingHistory} from '../components/wearing-history.js';
import {mountOutfitCatalogue} from '../components/outfit-catalogue.js';
import {inventoryEditor} from '../components/inventory-editor.js';
import {mountCursorRecords} from '../components/cursor-records.js';
import {CursorPager,bindCursorPager} from '../components/cursor-pager.js';
import {governanceCommand} from '../components/governance-command.js';
import {mountWardrobePreparation} from '../components/wardrobe-preparation.js';
import {mountOutfitComposer} from '../components/outfit-composer.js';
import {mountOutfitLibrary} from '../components/outfit-library.js';
import {Field,SearchForm,Tabs} from '../components/forms.js';
import {drawer,esc,badge} from '../ui.js';
const retired=fid=>get('characters',fid)?.status==='已退休';
const usable=item=>!item.archived_at&&item.source_verified&&item.availability==='available'&&['owned','borrowed'].includes(item.ownership);
export function renderRealWardrobe(el,ctx){
 const fid=ctx.charId;if(ctx.query.item){ctx.navigate(`#/c/${fid}/life/wardrobe/item/${encodeURIComponent(ctx.query.item)}`);return;}
 const tab=['compose','plans','clothes'].includes(ctx.query.tab)?ctx.query.tab:'compose';
 return asyncPage(el,()=>lifeClient.wardrobe(fid),(wardrobe,reload,alive)=>{
  el.innerHTML=`<div class="stack wardrobe-workspace"><div class="row"><h2 class="page-title grow">衣柜</h2><button class="btn" data-current-wear>查看当前穿着</button></div>${LifeNavigation(ctx)}${Tabs({value:tab,items:[['compose','为她搭配'],['plans','方案库'],['clothes','全部衣服']].map(([value,label])=>({value,label,href:`#/c/${encodeURIComponent(fid)}/life/wardrobe?tab=${value}`}))})}<details class="mgmt wardrobe-preparation"><summary>初始穿搭准备与任务</summary><div data-preparation></div></details><div data-wardrobe-panel></div></div>`;
  el.querySelector('[data-current-wear]').onclick=()=>drawer({title:'当前穿着',body:CurrentInventory(wardrobe.worn_items,[],{showUsed:false})+'<p class="small muted">当前登记与方案保存分别成立；未登记不等于没有穿衣。</p>'});
  const box=el.querySelector('[data-wardrobe-panel]');
  const preparation=mountWardrobePreparation(el.querySelector('[data-preparation]'),{fid,alive,onCompleted:reload});
  if(tab==='clothes'){const dispose=renderRealWardrobeInventory(box,ctx);return()=>{preparation.dispose();dispose?.();};}
  const child=tab==='compose'?mountOutfitComposer(box,{fid,outfitId:ctx.query.outfit||'',alive,navigate:ctx.navigate,onSaved:()=>ctx.navigate(`#/c/${encodeURIComponent(fid)}/life/wardrobe?tab=plans`)}):mountOutfitLibrary(box,{fid,alive,navigate:ctx.navigate,onSaved:reload});
  return()=>{preparation.dispose();child.dispose();};
 });
}
function renderRealWardrobeInventory(el,ctx){
 const fid=ctx.charId;
 if(ctx.query.item){ctx.navigate(`#/c/${fid}/life/wardrobe/item/${encodeURIComponent(ctx.query.item)}`);return;}
 const options={q:ctx.query.q||'',kind:'wearable',ownership:ctx.query.ownership||'',availability:ctx.query.availability||'',state:['current','worn','archived'].includes(ctx.query.state)?ctx.query.state:'current'},cursors=[ctx.query.cursor||''];let at=0;
 const listHash=()=>`#/c/${fid}/life/wardrobe?${new URLSearchParams({tab:'clothes',...options,...(cursors[at]?{cursor:cursors[at]}:{})})}`;
 const updatePage=()=>{history.replaceState(null,'',listHash());};
 return asyncPage(el,()=>inventoryClient.list(fid,{...options,cursor:cursors[at]}),(page,reload,alive)=>{
  el.innerHTML=`<div class="stack wardrobe-page"><div class="grid wtop">${CurrentInventory(page.worn_items,[],{showUsed:false})}<div class="card"><h3>拍一张</h3><p class="small muted">${badge('生成图·非事实')} 生成画面不改变穿着、位置或拥有关系。</p><button class="btn primary" data-photoall ${retired(fid)?'disabled':''}>拍照</button><button class="btn" data-add ${retired(fid)?'disabled':''}>新增服装</button><p class="small muted">衣柜版本 ${esc(page.revision)}</p></div></div><div class="card">${SearchForm({value:options.q,placeholder:'搜索全部登记物品',attrs:{'data-search':true}})}<div class="row" style="margin-top:8px">${[['state','范围',[['current','当前服装'],['worn','正在穿着'],['archived','已归档']]],['ownership','拥有关系',[['','全部'],['owned','已拥有'],['borrowed','借用'],['unknown','未确认']]],['availability','可用状态',[['','全部'],['available','可用'],['unavailable','不可用'],['lost','遗失']]]].map(([name,label,items])=>Field({name,label,type:'select',value:options[name],options:items.map(([value,label])=>({value,label}))})).join('')}</div><button class="btn sm" data-reload>重新读取</button></div>${page.items.length?`<div class="wgrid">${page.items.map(item=>InventoryCard(item,{returnTo:listHash()})).join('')}</div>`:'<div class="statebox"><h2>当前筛选没有物品</h2><p>可调整筛选、补录已知物品，或通过真实交流提出取得意向；尚未登记不证明不存在。</p><a class="btn" href="#/c/${fid}/chat">提出取得意向</a></div>'}${CursorPager({more:!!page.next_cursor,previous:at>0,label:'物品'})}${cursors[0]?'<button class="btn" data-first>返回第一页</button>':''}<p class="small muted">${page.inventory_complete?'物品登记已标为完整':'物品登记可能不完整'}；穿着、使用、拥有与可用状态分别显示。</p><div data-wearing-history></div></div>`;
  const navigate=()=>ctx.navigate(`#/c/${fid}/life/wardrobe?${new URLSearchParams({tab:'clothes',...Object.fromEntries(Object.keys(options).map(name=>[name,el.querySelector(`[name=${name}]`)?.value??options[name]]))})}`);
  el.querySelector('[data-search]').onsubmit=e=>{e.preventDefault();navigate();};el.querySelectorAll('select').forEach(select=>select.onchange=navigate);el.querySelector('[data-reload]').onclick=reload;
  el.querySelector('[data-add]').onclick=()=>{if(!retired(fid))inventoryEditor(fid,null,page.revision,reload);};

  const wearing=mountWearingHistory(el.querySelector('[data-wearing-history]'),{fid,alive});
  el.querySelector('[data-photoall]').onclick=()=>{if(!retired(fid))photoDrawer(fid);};
  el.querySelector('[data-first]')?.addEventListener('click',()=>{cursors.length=1;cursors[0]='';at=0;updatePage();void reload();});
  bindCursorPager(el,{next:()=>{cursors[++at]=page.next_cursor;updatePage();void reload();},previous:()=>{at--;updatePage();void reload();}});return()=>{wearing.dispose();};
 });
}
export function renderRealInventoryItem(el,ctx){
 const fid=ctx.charId,id=ctx.params.itemId;
 const listBase=`#/c/${fid}/life/wardrobe`,returnTo=ctx.query.returnTo?.startsWith(listBase+'?')?ctx.query.returnTo:listBase;
 return asyncPage(el,async()=>({item:await inventoryClient.read(fid,id),current:await inventoryClient.current(fid)}),({item,current},reload,alive)=>{
  if(item.item_kind!=='wearable'){el.innerHTML=`<div class="statebox"><h2>当前衣柜只管理服装</h2><p>普通物品与借还办理已退出当前范围，历史记录仍保留。</p><a class="btn" href="${esc(returnTo)}">返回衣柜</a></div>`;return;}
  const active=usable(item),readOnly=retired(fid)||!!item.archived_at;
  el.innerHTML=`<a class="btn" href="${esc(returnTo)}">返回衣柜</a>${InventoryIdentity(item)}<div class="row" style="margin:12px 0"><button class="btn" data-reload>重新读取</button><button class="btn" data-edit ${readOnly?'disabled':''}>编辑登记</button><button class="btn danger" data-archive ${readOnly?'disabled':''}>归档登记</button>${item.item_kind==='wearable'?`<button class="btn primary" data-action="wear" ${!active||retired(fid)?'disabled':''}>换装请求</button>`:`<button class="btn primary" data-action="use" ${(!active&&!item.use)||retired(fid)?'disabled':''}>${item.use?'请求停止使用':'请求开始使用'}</button>`}<button class="btn" data-photo ${readOnly?'disabled':''}>拍照</button>${item.worn?`<button class="btn" data-remove ${retired(fid)?'disabled':''}>请求脱下此部位</button>`:''}</div>${!active&&!item.use?'<p class="small muted">当前未满足来源、拥有或可用条件，不能据此发起穿用。</p>':''}<div class="card"><h3>使用、穿着与登记历史</h3><p class="small muted">使用事件保留既有记录；状态修订从本次迁移起追踪。Owner修正不冒充角色行动。</p><div data-history></div></div>`;
  el.querySelector('[data-reload]').onclick=reload;el.querySelector('[data-edit]').onclick=()=>{if(!readOnly)inventoryEditor(fid,item,current.revision,reload);};el.querySelector('[data-photo]').onclick=()=>{if(!readOnly)photoDrawer(fid,id);};
  el.querySelector('[data-return]')?.addEventListener('click',()=>requestInventoryAction(fid,'return',{itemIds:[id]},current.revision,reload,alive));
  el.querySelector('[data-remove]')?.addEventListener('click',()=>requestInventoryAction(fid,'remove',{slots:[item.worn.slot]},current.revision,reload,alive));
  el.querySelector('[data-archive]').onclick=()=>{
   if(readOnly)return;
   governanceCommand({scope:[fid,id,'inventory-archive'],title:'归档物品登记',note:'从当前物品目录移出，清除当前穿用关联。套装引用与使用、来源和修订历史保留，可在归档筛选中继续查询。这是Owner登记修正，不是角色丢弃或归还行动。',revision:current.revision,snapshot:item,target:{expectedItemRevision:item.revision},read:async()=>({...(await inventoryClient.current(fid)),item:await inventoryClient.read(fid,id)}),rebaseTarget:latest=>({expectedItemRevision:latest.item.revision}),execute:(body,target)=>inventoryClient.archive(fid,id,{...body,...target}),onSaved:reload});
  };
  el.querySelectorAll('[data-action]').forEach(button=>button.onclick=()=>{
   if(readOnly)return;const requestedActive=!item.use;
   governanceCommand({scope:[fid,id,button.dataset.action,!!item.use],title:button.textContent,note:'提交真实请求，由她判断是否接受。受理、工具执行和实际结果分别读回；不会在前端直接改穿着或使用状态。',revision:current.revision,snapshot:item,target:{slot:item.slot,active:requestedActive},maxReason:500,successMessage:result=>`请求 ${result.request.id} 已受理，执行结果待查询`,read:async()=>({...(await inventoryClient.current(fid)),item:await inventoryClient.read(fid,id)}),rebaseTarget:(latest,target)=>{if(button.dataset.action==='use'&&!!latest.item.use===target.active)throw new Error('最新使用状态已达到原请求目标，请关闭后重新查看；本次请求不会反转。');return {...target,slot:latest.item.slot};},execute:(body,target)=>button.dataset.action==='wear'?lifeClient.wear(fid,id,target.slot,body.expectedRevision,body.reason):lifeClient.use(fid,id,target.active,body.expectedRevision,body.reason),onSaved:envelope=>{if(alive())return requestReceipt(fid,envelope,reload);}});
  });
  const history=mountCursorRecords(el.querySelector('[data-history]'),{alive,label:'物品历史',read:cursor=>inventoryClient.history(fid,id,cursor),render:InventoryHistory,empty:'还没有可读取的使用或修订记录。登记来源仍见上方；不会补造旧历史。'});return()=>history.dispose();
 });
}
function photoDrawer(fid,itemId){return openOwnerMediaDrawer({fid,itemId,title:itemId?'物品图片请求':'当前穿搭图片请求',prompt:itemId?'':'请把你当前实际穿着的整套衣服画成一张全身插画。只采用当前有效外貌与合法穿着，不新增换装或移动，不把生成画面当作发生的事件。'});}
