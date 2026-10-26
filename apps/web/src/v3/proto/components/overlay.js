import {flushSync} from 'react-dom';
import {openOverlay,closeOverlay,closeAllOverlays} from '../../../state/overlays.ts';
import {get} from '../store.js';
// The remaining domain controllers retain their content node and exact handlers;
// the single React application owns every Sheet/Dialog and its focus lifecycle.
const readable=value=>{const template=document.createElement('template');template.innerHTML=String(value||'');return template.content.textContent||'';};
function overlay(kind,{title='',body='',footer='',onClose,wide}={}){
 const trigger=document.activeElement;
 const box=document.createElement('div');box.className='approved-domain domain-overlay-content';
 box.innerHTML=`<div class="body">${body}</div>${footer?`<footer>${footer}</footer>`:''}`;
 let id,closed=false;
 const api={el:box,body:box.querySelector('.body'),close(){if(!closed&&id)closeOverlay(id);},setBody(html){api.body.innerHTML=html;},setFooter(html){let element=box.querySelector('footer');if(!element){element=document.createElement('footer');box.append(element);}element.innerHTML=html;}};
 flushSync(()=>{id=openOverlay({kind,title:readable(title),node:box,wide,owner:get('session')?.actorId,trigger,onClose:()=>{if(closed)return;closed=true;onClose?.();}});});
 return api;
}
export const drawer=options=>overlay('sheet',options);
export const modal=options=>overlay('dialog',options);
export function confirm({title='确认',text='',okText='确认',identity='所有者修改',type='default'}){
 const trigger=document.activeElement;
 return new Promise(resolve=>{flushSync(()=>openOverlay({kind:'confirm',title:readable(title),text:readable(text)+(identity==='所有者修改'?'\n此操作将记入操作审计。':''),okText,destructive:type==='danger',owner:get('session')?.actorId,trigger,onClose:resolve}));});
}
export function closeOverlays(){closeAllOverlays();}
