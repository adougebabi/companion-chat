import {Field,Toggle} from './forms.js';
import {esc,confirm} from '../ui.js';
import {get} from '../store.js';
import {mediaClient} from '../services.js';
import {persistentCommand} from '../../providers/persistent-command.mjs';
import {studioDraftStorage} from './studio-draft-storage.js';

export function mountMediaMetadata(host,{asset,onSaved=()=>{}}){
 const owner=get('session')?.actorId,slot=`fluctlight-media:${owner}:${asset.id}:metadata`;let active=true,busy=false,current=asset,readOnly=asset.status==='deleted';
 const isOwner=()=>get('session')?.actorId===owner,alive=()=>active&&host.isConnected&&isOwner();
 let draft;const raw=studioDraftStorage().getItem(slot+':draft');draft=raw?JSON.parse(raw):{title:asset.title,hidden:asset.hidden,reason:'',expectedAssetRevision:asset.asset_revision,expectedMetadataRevision:asset.metadata_revision};
 const command=persistentCommand({key:slot+':command',storage:studioDraftStorage,execute:async(body,key)=>{if(!isOwner())throw new Error('会话已切换，未提交修改');return mediaClient.save(asset.id,{...body,idempotencyKey:key});}});
 const persist=()=>{if(isOwner())studioDraftStorage().setItem(slot+':draft',JSON.stringify(draft));};
 host.innerHTML=`<form class="card stack"><h3>素材库信息</h3><p class="small muted">名称和隐藏状态仅影响库中的展示。字节、来源、引用及读取权限由实际资产持有。</p>${Field({label:'素材名称',name:'title',value:draft.title,attrs:{maxlength:200}})}${Toggle({label:'在素材库中隐藏',name:'hidden',checked:draft.hidden})}${Field({label:'修改原因',name:'reason',type:'textarea',value:draft.reason,attrs:{required:true,maxlength:1000}})}<p data-version class="small muted"></p><p role="alert" data-error></p><div class="row"><button data-domain-ui="button" class="btn primary" data-save>保存库信息</button><button data-domain-ui="button" class="btn" type="button" data-retry>重试原提交</button><button data-domain-ui="button" class="btn" type="button" data-rebase>读取最新版本并保留草稿</button></div></form>`;
 const form=host.querySelector('form'),errors=host.querySelector('[data-error]');
 function paint(){if(!alive())return;const pending=command.pending();host.querySelector('[data-version]').textContent=`资产 v${draft.expectedAssetRevision} · 库信息 v${draft.expectedMetadataRevision}${pending?' · 原提交待核对':''}`;host.querySelector('[data-save]').disabled=readOnly||busy||!!pending;host.querySelector('[data-retry]').disabled=busy||!pending||!!pending.rejection;host.querySelector('[data-rebase]').disabled=busy||!!pending&&!pending.rejection;}
 for(const name of ['title','reason','hidden'])form.elements[name].oninput=()=>{draft[name]=name==='hidden'?form.elements[name].checked:form.elements[name].value;persist();};
 async function save(retry){if(busy||!alive()||!retry&&(readOnly||!form.reportValidity()))return;const submitted=JSON.stringify(retry?command.pending()?.value:draft);busy=true;paint();let accepted=false;try{
  await(retry?command.retry():command.submit(draft));accepted=true;if(!alive())return;const result=await mediaClient.asset(asset.id);if(!alive())return;
  if(JSON.stringify(draft)===submitted){draft={title:result.title,hidden:result.hidden,reason:'',expectedAssetRevision:result.asset_revision,expectedMetadataRevision:result.metadata_revision};studioDraftStorage().removeItem(slot+':draft');if(alive()){form.elements.reason.value='';form.elements.title.value=result.title;form.elements.hidden.checked=result.hidden;}}
  else persist();current=result;if(alive()){errors.textContent=JSON.stringify(draft)===submitted||!studioDraftStorage().getItem(slot+':draft')?'已保存并读回库信息。':'原提交已保存。新草稿已保留，请读取最新版本并明确采用后再保存。';await onSaved(result);}
 }catch(error){if(alive())errors.textContent=`${accepted?'原保存操作已记录；最新库信息读取失败：':''}${error.message}${error.code?'（'+error.code+'）':''}`;}finally{busy=false;paint();}}
 form.onsubmit=e=>{e.preventDefault();void save(false);};host.querySelector('[data-retry]').onclick=()=>void save(true);
 host.querySelector('[data-rebase]').onclick=async()=>{if(busy||!alive())return;busy=true;paint();try{current=await mediaClient.asset(asset.id);if(alive()&&await confirm({title:'采用最新素材版本',text:`当前名称：${esc(current.title||'未命名')}；${current.hidden?'已隐藏':'展示中'}。以资产v${current.asset_revision}、库信息v${current.metadata_revision}保存你的草稿？`} )&&alive()){
  if(command.pending()?.rejection)command.releaseRejected();draft.expectedAssetRevision=current.asset_revision;draft.expectedMetadataRevision=current.metadata_revision;persist();errors.textContent='已采用最新版本，草稿保持，请明确保存。';
 }}catch(error){if(alive())errors.textContent=error.message;}finally{busy=false;paint();}};paint();return {updateAsset(value){current=value;readOnly=value.status==='deleted';paint();},dispose(){active=false;}};
}
