import {MediaWorkspaceHeader} from './media.js';
import {Avatar,bindAvatars} from '../v3/proto/components/avatar.js';
import {stateLabel} from '../v3/domain-presentation.mjs';
import {InstantLabel} from '../v3/proto/components/instant-label.js';
import {get} from '../v3/proto/store.js';
import {mediaClient} from '../v3/proto/services.js';
import {asyncPage} from '../v3/proto/components/async-page.js';
import {mountCursorRecords} from '../v3/proto/components/cursor-records.js';
import {mountDisplayAvatar} from '../v3/proto/components/display-avatar.js';
import {JsonDetails} from '../v3/proto/components/records.js';
import {esc,toast} from '../v3/proto/ui.js';
export function renderRealAvatar(el,ctx){
 const fid=ctx.charId,owner=get('session')?.actorId;
 return asyncPage(el,()=>mediaClient.avatar(fid),(snapshot,reload,connected)=>{
  const alive=()=>connected()&&get('session')?.actorId===owner;
  el.innerHTML=`<div class="stack avatar-page">${MediaWorkspaceHeader(ctx,{title:'显示头像',active:'identity'})}<div class="row"><a class="btn" href="#/c/${encodeURIComponent(fid)}/profile">角色资料</a><a class="btn" href="#/c/${encodeURIComponent(fid)}/media/identity">规范视觉身份</a></div><div class="card" data-current></div><div data-editor></div><div class="card"><h3>显示头像选择历史</h3><div data-history></div></div></div>`;
  function paint(value){if(!alive())return;el.querySelector('[data-current]').innerHTML=`<div class="row">${Avatar({src:value.avatar?.url?mediaClient.url(value.avatar.asset_id):'',name:get('characters',fid)?.name||'角色',size:'lg'})}<div class="grow"><h3>当前实际显示</h3><p>${esc(value.mode==='canonical'?'跟随当前规范图':value.mode==='asset'?'使用选定素材':'显示方式待确认')} · ${esc(stateLabel(value.read_state,'读取状态待确认'))}</p></div></div>${!value.avatar?.url?'<p class="small muted">尚无可显示头像，暂时使用名字作为识别。</p>':''}<p class="small muted">这里的显示选择不改变规范视觉身份，也不作为模型参考授权。</p>${JsonDetails(value,'显示来源与版本')}`;bindAvatars(el.querySelector('[data-current]'));}
  paint(snapshot);const history=mountCursorRecords(el.querySelector('[data-history]'),{alive,label:'显示选择历史',read:cursor=>mediaClient.avatarHistory(fid,cursor),render:row=>`<div class="lrow"><span>v${esc(row.revision)} · ${InstantLabel(row.created_at)} · ${esc(row.reason)}</span>${JsonDetails(row,'选择来源与前后状态')}</div>`,empty:'尚未保存显示头像选择。可跟随规范图或明确选择已有素材。'});
  const editor=mountDisplayAvatar(el.querySelector('[data-editor]'),{fid,snapshot,onSaved:async value=>{if(alive()){paint(value);await history.reload({reset:true});try{if(alive())await mediaClient.refresh();}catch(error){if(alive())toast(`显示头像已保存；其他页面资料刷新失败：${error.message}`,'err');}}}});return()=>{editor.dispose();history.dispose();};
 });
}
