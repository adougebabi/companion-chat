import {get} from '../store.js';
import {mediaClient} from '../services.js';
import {Field,SearchForm} from './forms.js';
import {esc,confirm} from '../ui.js';
import {mountCursorRecords} from './cursor-records.js';
import {MediaAssetCard,bindMediaFailures} from './media-assets.js';
import {persistentCommand} from '../../providers/persistent-command.mjs';
import {studioDraftStorage} from './studio-draft-storage.js';

export function mountDisplayAvatar(host,{fid,snapshot,onSaved=()=>{}}){
 const owner=get('session')?.actorId,slot=`fluctlight-media:${owner}:${fid}:avatar`;let active=true,busy=false,search='';
 const isOwner=()=>get('session')?.actorId===owner,alive=()=>active&&host.isConnected&&isOwner(),readOnly=get('characters',fid)?.status==='已退休';
 const raw=studioDraftStorage().getItem(slot+':draft');let draft=raw?JSON.parse(raw):{mode:snapshot.mode==='asset'?'asset':'canonical',selected:snapshot.selection?.asset_id?{id:snapshot.selection.asset_id,asset_revision:snapshot.selection.asset_revision,title:snapshot.selection.asset_id}:null,expectedRevision:snapshot.revision,reason:''};
 const command=persistentCommand({key:slot+':command',storage:studioDraftStorage,execute:async(body,key)=>{if(!isOwner())throw new Error('会话已切换，未提交头像选择');return mediaClient.selectAvatar(fid,{...body,idempotencyKey:key});}});
 const body=()=>({mode:draft.mode,expectedRevision:draft.expectedRevision,reason:draft.reason,...(draft.mode==='asset'?{assetId:draft.selected?.id,expectedAssetRevision:draft.selected?.asset_revision}:{})});
 const persist=()=>{if(isOwner())studioDraftStorage().setItem(slot+':draft',JSON.stringify(draft));};
 host.innerHTML=`<div class="stack"><form class="card stack" data-avatar-command><h3>显示头像</h3><p>这里选择角色库和聊天中显示的头像。选择素材不会修改规范视觉身份、身体、穿着，也不会授权模型使用这张图。</p><p data-selection></p>${Field({label:'选择方式',name:'mode',value:draft.mode,type:'select',options:[{value:'canonical',label:'跟随当前规范图'},{value:'asset',label:'使用明确选择的素材'}]})}${Field({label:'选择原因',name:'reason',type:'textarea',value:draft.reason,attrs:{required:true,maxlength:1000}})}<p class="small muted" data-version></p><p role="alert" data-error></p><div class="row"><button class="btn primary" data-save>保存显示头像</button><button class="btn" type="button" data-retry>重试原提交</button><button class="btn" type="button" data-rebase>读取最新版本并保留选择</button></div></form><div class="card"><h3>从我的图片素材中选择</h3><div data-search>${SearchForm({placeholder:'搜索图片名称'})}</div><p class="small muted">只列出你有权读取的素材；跨角色选择只用于明确的显示头像关联。不可读的素材需先恢复来源或对象存储。</p><a class="btn" href="#/studio/media">上传与管理我的素材</a><div data-picker class="wgrid"></div></div></div>`;
 const form=host.querySelector('[data-avatar-command]'),errors=host.querySelector('[data-error]');
 function paint(){if(!alive())return;const pending=command.pending();host.querySelector('[data-selection]').textContent=draft.mode==='asset'?`当前草稿：${draft.selected?.title||'尚未选择图片'}`:'当前草稿：跟随当前规范图';host.querySelector('[data-version]').textContent=`显示选择 v${draft.expectedRevision}${pending?' · 原提交待确认':''}`;host.querySelector('[data-save]').disabled=readOnly||busy||!!pending;host.querySelector('[data-retry]').disabled=readOnly||busy||!pending||!!pending.rejection;host.querySelector('[data-rebase]').disabled=busy||!!pending&&!pending.rejection;}
 form.elements.mode.onchange=()=>{draft.mode=form.elements.mode.value;persist();paint();};form.elements.reason.oninput=()=>{draft.reason=form.elements.reason.value;persist();};
 const picker=mountCursorRecords(host.querySelector('[data-picker]'),{alive,label:'图片素材',read:cursor=>mediaClient.list({kind:'image',q:search,cursor}),render:asset=>`<div>${MediaAssetCard(asset,{url:mediaClient.url(asset.id)})}<button class="btn" type="button" data-pick="${esc(asset.id)}" ${readOnly||!asset.readable?'disabled':''}>选作显示头像</button></div>`,empty:'当前没有图片素材。可前往我的素材库上传，或在创作中心生成并保留图片。',bind:(box,page)=>{bindMediaFailures(box);box.querySelectorAll('[data-pick]').forEach(button=>button.onclick=()=>{if(!alive()||readOnly)return;const asset=page.items.find(row=>row.id===button.dataset.pick);draft.mode='asset';draft.selected={id:asset.id,asset_revision:asset.asset_revision,title:asset.title};form.elements.mode.value='asset';persist();paint();});}});
 host.querySelector('[data-search] form').onsubmit=e=>{e.preventDefault();search=e.target.elements.q.value.trim();void picker.reload({reset:true});};
 async function submit(retry){if(readOnly||busy||!alive()||!retry&&!form.reportValidity())return;if(!retry&&draft.mode==='asset'&&!draft.selected){errors.textContent='请先从素材库明确选择一张图片';return;}const submitted=JSON.stringify(retry?command.pending()?.value:body());busy=true;paint();try{
  const result=await(retry?command.retry():command.submit(body()));if(!alive())return;
  if(JSON.stringify(body())===submitted){draft.expectedRevision=result.revision;draft.reason='';form.elements.reason.value='';persist();errors.textContent='显示头像已保存并读回。';}else errors.textContent='原选择已保存，新选择或理由已保留；请读取最新版本再明确保存。';
  await onSaved(result);
 }catch(error){if(alive())errors.textContent=`${error.message}${error.code?'（'+error.code+'）':''}`;}finally{busy=false;paint();}}
 form.onsubmit=e=>{e.preventDefault();void submit(false);};host.querySelector('[data-retry]').onclick=()=>void submit(true);
 host.querySelector('[data-rebase]').onclick=async()=>{if(busy||!alive())return;busy=true;paint();try{const current=await mediaClient.avatar(fid);if(alive()&&await confirm({title:'采用最新显示选择版本',text:`当前显示方式：${current.mode==='canonical'?'跟随当前规范图':current.mode==='asset'?'选定素材':'待确认'}，版本v${current.revision}。保留你的选择和理由，以此版本继续？`})&&alive()){if(command.pending()?.rejection)command.releaseRejected();draft.expectedRevision=current.revision;persist();errors.textContent='已采用最新版本，选择和理由保留。';}}catch(error){if(alive())errors.textContent=error.message;}finally{busy=false;paint();}};
 paint();return {dispose(){active=false;picker.dispose();}};
}
