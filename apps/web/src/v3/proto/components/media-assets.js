import {esc,badge} from '../ui.js';

export function mediaDetailPath(asset,fid=''){return fid?`#/c/${encodeURIComponent(fid)}/media/${encodeURIComponent(asset.id)}`:`#/studio/media/${encodeURIComponent(asset.id)}`;}
export function MediaPreview(asset,url){
 if(!asset.readable||!url)return `<div class="statebox" role="status">素材当前不可读取：${esc(asset.read_state)}</div>`;
 if(asset.kind==='image')return `<img class="media-preview" src="${esc(url)}" alt="${esc(asset.title||'媒体素材')}" loading="lazy" data-media-content>`;
 if(asset.kind==='video')return `<video class="media-preview" src="${esc(url)}" controls preload="metadata" aria-label="${esc(asset.title||'视频素材')}" data-media-content></video>`;
 if(asset.kind==='audio')return `<audio class="media-preview" src="${esc(url)}" controls preload="metadata" aria-label="${esc(asset.title||'音频素材')}" data-media-content></audio>`;
 throw new Error('无法识别媒体类型');
}
export function MediaAssetCard(asset,{fid='',url,targetId=''}={}){return `<article class="card media-asset-card">${MediaPreview(asset,url)}<div class="small"><a href="${esc(mediaDetailPath(asset,fid)+(targetId?'?target='+encodeURIComponent(targetId):''))}"><b>${esc(asset.title||'未命名素材')}</b></a><p>${badge(asset.label)} ${asset.hidden?badge('已在库中隐藏'):''}</p><p class="muted">${esc(asset.kind)} · ${esc(asset.status)} · ${esc(asset.created_at)}</p></div></article>`;}
export function bindMediaFailures(host){host.querySelectorAll('[data-media-content]').forEach(media=>media.addEventListener('error',()=>{
 const error=document.createElement('p');error.className='banner err';error.setAttribute('role','alert');error.textContent='媒体字节读取失败，请重新查询素材状态或检查存储连接。';media.replaceWith(error);
 },{once:true}));}
