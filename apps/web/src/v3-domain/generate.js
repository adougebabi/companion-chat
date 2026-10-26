import {MediaWorkspaceHeader} from './media.js';
import {userInstant} from '../v3/presentation-time.mjs';
import {get} from '../v3/proto/store.js';
import {mediaClient} from '../v3/proto/services.js';
import {esc} from '../v3/proto/ui.js';
import {mountOwnerMediaRequest} from '../v3/proto/components/media-generation.js';
import {mountCursorRecords} from '../v3/proto/components/cursor-records.js';
export function renderRealGenerate(el,ctx){
 const owner=get('session')?.actorId,fid=ctx.charId;let active=true;const alive=()=>active&&el.isConnected&&get('session')?.actorId===owner;
 el.innerHTML=`<div class="stack">${MediaWorkspaceHeader(ctx,{title:'生成画面',active:'generate'})}<div><a class="btn" href="#/c/${encodeURIComponent(fid)}/media">返回媒体库</a></div><div data-compose></div><div class="card"><h3>图片请求历史</h3><div data-generations></div></div></div>`;
 let history;const composer=mountOwnerMediaRequest(el.querySelector('[data-compose]'),{fid,requestId:ctx.query.request||'',initialGrantId:ctx.query.grant||'',onSubmitted:()=>history.reload({reset:true})});
 history=mountCursorRecords(el.querySelector('[data-generations]'),{alive,label:'图片请求',read:cursor=>mediaClient.generations(fid,cursor),render:r=>`<div class="lrow"><span class="grow">${esc(r.payload?.prompt)}<br><span class="small muted">角色处理状态：${esc(r.status)} · ${esc(userInstant(r.created_at))}</span></span><button class="btn" type="button" data-generation="${esc(r.id)}">查询实际图片结果</button></div>`,empty:'尚未提出图片请求。可以填写画面描述并提交，由角色判断后创建真实生成任务。',bind:box=>box.querySelectorAll('[data-generation]').forEach(button=>button.onclick=()=>{if(alive())composer.select(button.dataset.generation);})});
 return()=>{active=false;composer.dispose();history.dispose();};
}
