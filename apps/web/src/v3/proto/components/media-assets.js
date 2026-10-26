import {esc,badge} from '../ui.js';
import {stateLabel} from '../../domain-presentation.mjs';
import {InstantLabel} from './instant-label.js';

export function mediaDetailPath(asset,fid=''){return fid?`#/c/${encodeURIComponent(fid)}/media/${encodeURIComponent(asset.id)}`:`#/studio/media/${encodeURIComponent(asset.id)}`;}
export function MediaPreview(asset,url){
 if(!asset.readable||!url)return `<div class="statebox" role="status">素材当前不可读取：${esc(stateLabel(asset.read_state,'请核对来源与读取权限'))}</div>`;
 if(asset.kind==='image')return `<img class="media-preview" src="${esc(url)}" alt="${esc(asset.title||'媒体素材')}" loading="lazy" data-media-content>`;
 if(asset.kind==='video')return `<video class="media-preview" src="${esc(url)}" controls preload="metadata" aria-label="${esc(asset.title||'视频素材')}" data-media-content></video>`;
 if(asset.kind==='audio')return `<audio class="media-preview" src="${esc(url)}" controls preload="metadata" aria-label="${esc(asset.title||'音频素材')}" data-media-content></audio>`;
 throw new Error('无法识别媒体类型');
}
export function MediaAssetCard(asset,{fid='',url,targetId='',returnTo=''}={}){
 const params=new URLSearchParams();if(targetId)params.set('target',targetId);if(returnTo){params.set('returnTo',returnTo);params.set('returnLabel','返回媒体库');}const href=mediaDetailPath(asset,fid)+(params.size?'?'+params:'');
 const cover=asset.kind==='image'&&asset.readable?`<img src="${esc(url)}" alt="" loading="lazy" data-media-cover>`:`<div class="media-kind-cover" aria-hidden="true">${asset.kind==='audio'?'♫':asset.kind==='video'?'▷':'▧'}</div>`;
 return `<article data-domain-ui="card" class="card media-asset-card"><div data-domain-ui="card" class="media-card-cover">${cover}<span class="small muted" data-cover-failure hidden>封面暂不可用</span></div><div data-domain-ui="card" class="media-card-body"><h3>${esc(asset.title||'未命名素材')}</h3><p class="small">${esc(asset.label)} ${asset.hidden?badge('已隐藏'):''}</p><p class="small muted">${esc(({image:'图片',video:'视频',audio:'音频'})[asset.kind]||'素材')} · ${esc(stateLabel(asset.status))}${asset.kind!=='image'?' · 时长未记录':''}<br>${InstantLabel(asset.created_at)}</p><a data-domain-ui="button" class="btn" href="${esc(href)}">${asset.kind==='audio'?'播放预览':'查看详情'}</a></div></article>`;
}
export function bindMediaFailures(host){host.querySelectorAll('[data-media-cover]').forEach(img=>{const fail=()=>{img.hidden=true;img.parentElement.querySelector('[data-cover-failure]').hidden=false;};img.addEventListener('error',fail,{once:true});if(img.complete&&!img.naturalWidth)fail();});host.querySelectorAll('[data-media-content]').forEach(media=>media.addEventListener('error',()=>{
 const error=document.createElement('p');error.className='banner err';error.setAttribute('role','alert');error.textContent='媒体字节读取失败，请重新查询素材状态或检查存储连接。';media.replaceWith(error);
 },{once:true}));}
