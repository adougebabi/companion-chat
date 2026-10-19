import {get} from '../store.js';
import {mediaClient} from '../services.js';
import {asyncPage} from '../components/async-page.js';
import {mountCursorRecords} from '../components/cursor-records.js';
import {mountDisplayAvatar} from '../components/display-avatar.js';
import {JsonDetails} from '../components/records.js';
import {esc,toast} from '../ui.js';
export function renderRealAvatar(el,ctx){
 const fid=ctx.charId,owner=get('session')?.actorId;
 return asyncPage(el,()=>mediaClient.avatar(fid),(snapshot,reload,connected)=>{
  const alive=()=>connected()&&get('session')?.actorId===owner;
  el.innerHTML=`<div class="row"><a class="btn" href="#/c/${encodeURIComponent(fid)}/profile">角色资料</a><a class="btn" href="#/c/${encodeURIComponent(fid)}/media/identity">规范视觉身份</a></div><div class="card" data-current></div><div data-editor></div><div class="card"><h3>显示头像选择历史</h3><div data-history></div></div>`;
  function paint(value){if(!alive())return;el.querySelector('[data-current]').innerHTML=`<h3>当前实际显示</h3>${value.avatar?.url?`<img class="av lg" src="${esc(mediaClient.url(value.avatar.asset_id))}" alt="当前显示头像">`:''}<p>${esc(value.mode)} · ${esc(value.read_state)} · v${value.revision}</p><p class="small muted">规范视觉身份仍由原生成与评审流程管理；当前选择不作为模型参考授权。</p>`;}
  paint(snapshot);const history=mountCursorRecords(el.querySelector('[data-history]'),{alive,label:'显示选择历史',read:cursor=>mediaClient.avatarHistory(fid,cursor),render:row=>`<div class="lrow"><span>v${esc(row.revision)} · ${esc(row.created_at)} · ${esc(row.reason)}</span>${JsonDetails(row,'选择来源与前后状态')}</div>`,empty:'尚未保存显示头像选择。可跟随规范图或明确选择已有素材。'});
  const editor=mountDisplayAvatar(el.querySelector('[data-editor]'),{fid,snapshot,onSaved:async value=>{if(alive()){paint(value);await history.reload({reset:true});try{if(alive())await mediaClient.refresh();}catch(error){if(alive())toast(`显示头像已保存；其他页面资料刷新失败：${error.message}`,'err');}}}});return()=>{editor.dispose();history.dispose();};
 });
}
