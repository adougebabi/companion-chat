import {mountMediaLifecycle} from '../components/media-lifecycle.js';
import {mountReferenceGrantManager} from '../components/media-reference-grants.js';
import {get} from '../store.js';
import {mediaClient} from '../services.js';
import {esc} from '../ui.js';
import {Field,KeyValueList} from '../components/forms.js';
import {JsonDetails} from '../components/records.js';
import {asyncPage} from '../components/async-page.js';
import {mountCursorRecords} from '../components/cursor-records.js';
import {MediaPreview,MediaAssetCard,bindMediaFailures} from '../components/media-assets.js';
import {mountMediaUpload} from '../components/media-upload.js';
import {mountMediaMetadata} from '../components/media-metadata.js';

export function renderRealMediaLibrary(el,ctx){
 const fid=ctx.charId||'',owner=get('session')?.actorId;let active=true,filters={scope:fid?'fluctlight':'all',q:ctx.query.q||'',kind:'',hidden:false};
 const alive=()=>active&&el.isConnected&&get('session')?.actorId===owner;
 el.innerHTML=`<div class="banner warn">媒体保留上传或生成来源；画面不会改变角色位置、穿着或生活事实。</div><form class="card stack" data-filters><div class="row"><h3 class="grow">${fid?'角色媒体库':'我的素材库'}</h3>${fid?`<a class="btn" href="#/c/${encodeURIComponent(fid)}/media/generate">生成一张</a><a class="btn" href="#/c/${encodeURIComponent(fid)}/media/identity">视觉身份</a>`:'<a class="btn" href="#/studio">返回创作中心</a>'}</div><div class="grid g3">${Field({label:'搜索素材名称',name:'q',value:filters.q,attrs:{maxlength:200}})}${Field({label:'媒体类型',name:'kind',type:'select',options:[{value:'',label:'全部类型'},{value:'image',label:'图片'},{value:'video',label:'视频'},{value:'audio',label:'音频'}]})}${Field({label:'素材来源',name:'scope',value:filters.scope,type:'select',options:fid?[{value:'all',label:'当前角色全部关联素材'},{value:'fluctlight',label:'角色生成素材'},{value:'studio',label:'明确关联的创作素材'},{value:'owner_upload',label:'明确关联的上传素材'}]:[{value:'all',label:'我的全部素材'},{value:'owner_upload',label:'我的上传'},{value:'studio',label:'创作素材'},{value:'fluctlight',label:'角色生成素材'}]})}</div><div class="row"><label class="toggle"><input type="checkbox" name="hidden"> 只看已在库中隐藏的素材</label><button class="btn">查询</button>${fid?'<a class="btn" href="#/studio/media">上传与管理我的素材</a>':''}</div></form><div data-media-list class="wgrid"></div>${fid?'':'<div data-upload-host></div>'}`;
 const panel=mountCursorRecords(el.querySelector('[data-media-list]'),{alive,label:'媒体素材',read:cursor=>mediaClient.list({...filters,kind:filters.kind||undefined,fluctlightId:fid||undefined,cursor}),render:asset=>MediaAssetCard(asset,{fid,url:mediaClient.url(asset.id),targetId:ctx.query.target}),empty:fid?'当前筛选没有关联媒体。可生成画面，或前往视觉身份建立角色的视觉资料。':'当前筛选没有素材。可在下方上传文件，或在创作中心生成并保留素材。',bind:bindMediaFailures});
 el.querySelector('[data-filters]').onsubmit=e=>{e.preventDefault();const form=e.target;filters={q:form.elements.q.value.trim(),kind:form.elements.kind.value,scope:form.elements.scope.value,hidden:form.elements.hidden.checked};void panel.reload({reset:true});};
 const upload=!fid?mountMediaUpload(el.querySelector('[data-upload-host]'),{onSaved:()=>panel.reload({reset:true})}):null;
 return()=>{active=false;panel.dispose();upload?.dispose();};
}
export function renderRealMediaDetail(el,ctx){
 const fid=ctx.charId||'',owner=get('session')?.actorId;
 return asyncPage(el,()=>mediaClient.asset(ctx.params.mdId,fid),(asset,reload,connected)=>{
  if(get('session')?.actorId!==owner)throw new Error('会话已切换，请重新打开素材');
  const sourceReference=asset.source_snapshot?.reference_asset?.asset_id;
  const library=fid?`#/c/${encodeURIComponent(fid)}/media`:'#/studio/media';
  el.innerHTML=`<div class="grid g2"><div class="card" data-preview>${MediaPreview(asset,mediaClient.url(asset.id))}</div><div class="card"><h2 data-title>${esc(asset.title||'未命名素材')}</h2><p>${esc(asset.label)}</p>${KeyValueList([['状态',`<span data-asset-status>${esc(asset.status)}</span>`],['读取状态',`<span data-read-state>${esc(asset.read_state)}</span>`],['类型',esc(asset.mime_type)],['大小',esc(asset.byte_size)+' 字节'],['来源',esc(asset.origin)],['创建时间',esc(asset.created_at)],['库中展示',`<span data-hidden>${asset.hidden?'隐藏':'可见'}</span>`]])}<p class="small muted">历史快照仅使用生成时已记录的来源；缺失来源显示未知，不用角色当前状态补写。</p>${typeof sourceReference==='string'?`<a class="btn" href="#/studio/media/${encodeURIComponent(sourceReference)}">查看原始参考素材</a>`:''}${JsonDetails(asset.source_snapshot,asset.source_state==='owner_original'?'用户原始素材（未作角色事实认证）':'生成时来源快照 · '+asset.source_state)}${JsonDetails({asset_id:asset.id,version:asset.version,asset_revision:asset.asset_revision,sha256:asset.sha256,upload_id:asset.upload_id,studio_job_id:asset.studio_job_id,generation_intent_id:asset.generation_intent_id},'资产与校验信息')}<a class="btn primary" href="${esc((fid?'#/c/'+encodeURIComponent(fid)+'/media/':'#/studio/media/')+encodeURIComponent(asset.id)+'?authorizeReference=1')}">以此为参考再生成</a><a class="btn" href="${esc(library)}">返回媒体库</a></div></div><div data-metadata></div><div data-reference-grants></div><div data-lifecycle></div>`;
  bindMediaFailures(el.querySelector('[data-preview]'));
  let previewMarkup=MediaPreview(asset,mediaClient.url(asset.id));
  const metadata=mountMediaMetadata(el.querySelector('[data-metadata]'),{asset,onSaved:result=>{if(!connected()||get('session')?.actorId!==owner)return;el.querySelector('[data-title]').textContent=result.title||'未命名素材';el.querySelector('[data-hidden]').textContent=result.hidden?'隐藏':'可见';}});
  const grants=asset.kind==='image'?mountReferenceGrantManager(el.querySelector('[data-reference-grants]'),{asset,targetId:ctx.query.target||fid,targetScope:ctx.query.target?'studio_resource':fid?'fluctlight':'studio_resource'}):null;
  if(ctx.query.authorizeReference==='1')el.querySelector('[data-reference-grants]').scrollIntoView({block:'start'});
  const lifecycle=mountMediaLifecycle(el.querySelector('[data-lifecycle]'),{asset,onAssetChanged:latest=>{if(!connected()||get('session')?.actorId!==owner)return;el.querySelector('[data-asset-status]').textContent=latest.status;el.querySelector('[data-read-state]').textContent=latest.read_state;const markup=MediaPreview(latest,mediaClient.url(latest.id));if(markup!==previewMarkup){previewMarkup=markup;const preview=el.querySelector('[data-preview]');preview.innerHTML=markup;bindMediaFailures(preview);}grants?.updateAsset(latest);metadata.updateAsset(latest);}});
  return()=>{metadata.dispose();grants?.dispose();lifecycle.dispose();};
 });
}
