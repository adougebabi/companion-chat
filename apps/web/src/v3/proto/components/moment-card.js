import {esc,badge} from '../ui.js';
import {get,list} from '../store.js';
import {Avatar} from './avatar.js';
import {mediaClient} from '../services.js';
import {MediaPreview} from './media-assets.js';
import {InstantLabel} from './instant-label.js';
import {momentReading} from '../../moment-reading.mjs';

export function MomentCard(post,{full=false,returnTo=''}={}){
 const reading=momentReading(post);
 const author=post.author_type==='user'?'我':post.author_name||'作者名称未记录';
 const person=get('characters',post.author_actor_id)||list('npcs',n=>n.actor_id===post.author_actor_id)[0];
 const count=value=>Number.isSafeInteger(value)&&value>=0?String(value):'—';
 const reaction=(kind,label)=>`<button class="btn ghost sm" type="button" data-moment-action="${esc(post.id)}" data-action="reaction" data-reaction="${post.viewer_reaction===kind?'none':kind}" ${post.status==='hidden'?'disabled':''}>${post.viewer_reaction===kind?'取消'+label:label}</button>`;
 return `<article class="moment-post" data-test="post-${esc(post.id)}"><div class="moment-avatar">${Avatar({src:person?.avatar,name:author})}</div><div class="moment-body"><header class="row"><b class="grow">${esc(author)}</b>${post.status==='hidden'?badge('已隐藏'):''}</header><p class="prewrap">${esc(reading.text)}</p>${reading.corrected?`<details class="moment-text-repair"><summary>阅读版已略去历史正文末尾的异常字符 · 查看原文</summary><p class="prewrap">${esc(reading.original)}</p></details>`:''}<div class="moment-media">${(post.media||[]).map(asset=>`<div>${MediaPreview(asset,mediaClient.url(asset.id))}<a class="small muted" href="#/studio/media/${encodeURIComponent(asset.id)}">查看媒体来源</a></div>`).join('')}</div><p class="small muted">${InstantLabel(post.created_at)}</p><div class="row moment-actions">${reaction('like',post.viewer_reaction==='like'?'点赞':'赞')}<span class="small muted">${count(post.like_count)}</span>${full?`<button class="btn ghost sm" type="button" data-scroll-comments>评论 ${count(post.comment_count)}</button>`:`<a class="btn ghost sm" href="#/feed/${encodeURIComponent(post.id)}${returnTo?'?returnTo='+encodeURIComponent(returnTo):''}">评论 ${count(post.comment_count)}</a>`}<details class="moment-more"><summary>更多互动</summary><div class="moment-more-menu">${reaction('care','关心')}${reaction('celebrate','庆祝')}<button class="btn ghost sm" type="button" data-moment-action="${esc(post.id)}" data-action="${post.status==='hidden'?'restore':'hide'}">${post.status==='hidden'?'恢复展示':'隐藏'}</button></div></details></div>${post.viewer_reaction==='care'||post.viewer_reaction==='celebrate'?`<p class="small muted">你已${post.viewer_reaction==='care'?'关心':'庆祝'}</p>`:''}${post.request_id?`<p class="small"><a href="#/diag/request/${encodeURIComponent(post.request_id)}">原发布建议与处理结果</a></p>`:''}</div></article>`;
}
