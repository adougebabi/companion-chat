import {get} from '../store.js';
import {mediaClient} from '../services.js';
import {esc} from '../ui.js';
import {mediaUploadController} from '../../providers/media-upload.mjs';
import {studioDraftStorage} from './studio-draft-storage.js';
import {mediaDetailPath} from './media-assets.js';

export function mountMediaUpload(host,{onSaved=()=>{}}={}){
 const owner=get('session')?.actorId;let active=true,busy=false;
 const isOwner=()=>get('session')?.actorId===owner,alive=()=>active&&host.isConnected&&isOwner();
 const upload=mediaUploadController({slot:`fluctlight-media:${owner}:upload`,storage:studioDraftStorage,client:mediaClient,isOwner});
 host.innerHTML=`<form class="card stack" data-upload><h3>上传素材</h3><p class="small muted">保存到你的素材库。支持图片、视频和音频，单个文件不超过64MiB。上传不会修改角色头像、身体或生活事实。</p><label class="field">选择文件<input class="input" type="file" name="file" accept="image/png,image/jpeg,image/gif,image/webp,video/mp4,video/webm,audio/mpeg,audio/wav,audio/ogg"></label><p class="small" data-pending></p><p role="status" data-status></p><p role="alert" data-error></p><div class="row"><button class="btn primary" data-submit>上传素材</button><button class="btn" type="button" data-query>查询或重试原上传</button></div><div data-result></div></form>`;
 const form=host.querySelector('form'),status=host.querySelector('[data-status]'),errorBox=host.querySelector('[data-error]');
 function paint(){if(!alive())return;const pending=upload.pending();host.querySelector('[data-pending]').textContent=pending?`待确认：${pending.value.filename} · ${pending.value.size}字节。刷新后可查询结果；未完成时需重新选择原文件重试。`:'';host.querySelector('[data-submit]').disabled=busy||!!pending;host.querySelector('[data-query]').disabled=busy||!pending;form.elements.file.disabled=busy;const last=upload.lastAsset();host.querySelector('[data-result]').innerHTML=last?`<a class="btn" href="${esc(mediaDetailPath({id:last}))}">查询上次上传的素材详情</a>`:'';}
 async function run(retry){if(busy||!alive())return;busy=true;errorBox.textContent='';status.textContent=retry?'正在查询原提交；未完成时以原文件重试…':'正在计算文件摘要并上传…';paint();try{
  const result=await(retry?upload.retry():upload.submit());if(alive()){status.textContent=result.readable?'上传已持久化，媒体可读取。':`原上传已记录，资产当前状态：${result.status}；读取状态：${result.read_state}`;await onSaved(result);}
 }catch(error){if(alive()){status.textContent='';errorBox.textContent=`${error.message}${error.code?'（'+error.code+'）':''}`;}}finally{busy=false;paint();}}
 form.elements.file.onchange=e=>upload.select(e.target.files?.[0]||null);
 form.onsubmit=e=>{e.preventDefault();void run(false);};host.querySelector('[data-query]').onclick=()=>void run(true);paint();
 return {dispose(){active=false;}};
}
