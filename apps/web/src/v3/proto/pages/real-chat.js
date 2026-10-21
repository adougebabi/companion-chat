import { randomId } from '../../../random-id.mjs';
import * as store from '../store.js';
import { openConversation, conversation, client } from '../../providers/real.ts';
import { esc, badge, drawer, toast, objLink, requireOwner } from '../ui.js';
import { ChatPanel, MessageBubble } from '../components/chat.js';
import {bindAvatars} from '../components/avatar.js';
import {chatFailureText,chatDiagnosticTurn} from '../../chat-feedback.mjs';
import { messageTimeLabel } from '../../../lib/message-time';
const options={actionById:id=>store.get('actions',id),mediaById:id=>store.get('media',id),link:objLink};
const draftText=new Map();
const readingPositions=new Map();
const localViews=new WeakMap();
const newSessionKeys=new Map();
function projected(message,id){return {id:message.id,charId:id,from:message.kind==='user'?'user':message.kind==='system'?'sys':'char',text:message.text,at:messageTimeLabel(message),turnId:message.turnId,idempotencyKey:message.idempotencyKey,turnStatus:message.turnStatus,turnRetryable:message.turnRetryable,turnErrorCode:message.turnErrorCode,attachmentRefs:message.attachmentRefs||[],refs:message.attachmentRefs||[],sourceRef:message.sourceRef,memoryRef:message.memoryRef||message.memory_ref,goalRef:message.goalRef||message.goal_ref,recall:message.recall,mediaId:message.media_id,kind:message.kind==='image'?'image':undefined};}
export function refreshStream() {}
export function actionDrawer(id){const action=store.get('actions',id);drawer({title:'行动详情',body:action?`<p>${esc(action.title)}</p><pre>${esc(JSON.stringify(action,null,2))}</pre>`:'<p>当前范围未读取到该行动，请进入生活或诊断重新读取。</p>'});}
export function renderChat(el,ctx){
  let alive=true,unsubscribe=null;const c=store.get('characters',ctx.charId), token={};localViews.set(el,token);
  const current=()=>alive&&localViews.get(el)===token;
  if(c.status!=='正常'){el.innerHTML=`<div class="statebox"><h2>${esc(c.name)}${esc(c.status)}</h2><p>${esc(c.statusNote||'当前状态无法发送对话')}</p><a class="btn" href="#/c/${c.id}/profile">查看资料</a></div>`;return;}
  el.innerHTML='<div class="card" role="status">正在读取真实会话…</div>';
  let stream,form,status,controls,feedback,locatedMessagePanel,composing=false,lastSignature='';
  const update=()=>{
    if(!current()||!stream)return;const state=conversation();if(state.fluctlightId!==c.id)return;
    const messages=state.messages.map(message=>projected(message,c.id));
    const signature=JSON.stringify(messages);
    if(signature!==lastSignature){const oldHeight=stream.scrollHeight,oldTop=stream.scrollTop,near=oldHeight-oldTop-stream.clientHeight<72;stream.innerHTML=messages.map(message=>MessageBubble(message,{...options,characterAvatar:c.avatar,characterName:c.name,mediaUrl:id=>`/api/media/${encodeURIComponent(id)}`})).join('');if(locatedMessagePanel)stream.prepend(locatedMessagePanel);bindAvatars(stream);if(near)stream.scrollTop=stream.scrollHeight;else stream.scrollTop=oldTop;lastSignature=signature;}
    status.textContent=chatFailureText(state.error)||((state.sending||state.hasPendingTurn)?'正在处理回复…':state.hasSettlingTurn?'正在结束本次生成…':'');
    controls.innerHTML=`${state.nextBeforeSequence?'<button class="btn sm" data-older>加载更早消息</button>':''}${state.canCancel?'<button class="btn sm" data-cancel>取消生成</button>':''}${state.canRetry?`<button class="btn sm" data-retry>${state.retryIsModelFailure?'重试生成':'确认后重发'}</button>`:''}`;
    const failedTurn=chatDiagnosticTurn(state);
    if(failedTurn){const link=document.createElement('a');link.className='btn ghost sm';link.href='#/diag/events?correlationId='+encodeURIComponent('turn:'+failedTurn);link.textContent='查看关联诊断';controls.append(link);}
    feedback.hidden=!status.textContent&&!controls.children.length;
    feedback.classList.toggle('is-pending',!!(state.sending||state.hasPendingTurn||state.hasSettlingTurn));
    form.querySelector('[data-test="chat-send"]').disabled=state.sending;
    controls.querySelector('[data-older]')?.addEventListener('click',async()=>{const before=stream.scrollHeight,top=stream.scrollTop;await state.loadOlder();update();stream.scrollTop=top+stream.scrollHeight-before;});
    controls.querySelector('[data-cancel]')?.addEventListener('click',()=>void state.cancel());
    controls.querySelector('[data-retry]')?.addEventListener('click',()=>void state.retry());
    // The composer remains mounted through every stream/poll update.
  };
  const readScope=async()=>{if(ctx.query.conversation)await client.selectDirectConversation(c.id,ctx.query.conversation);if(!current())throw new DOMException('页面已离开','AbortError');return openConversation(c.id,current);};
  void readScope().then(state=>{
    if(!current())return;
    el.innerHTML=ChatPanel({character:c,summary:`<details class="summary card tint-primary" style="margin:8px 12px;padding:8px 12px"><summary>此刻：${esc(c.doing||'尚无当前活动')}</summary><p>${esc(c.locationText||store.get('places',c.locationId)?.name||'位置尚未记录')}</p></details>`,messages:[],messageOptions:options,sidebar:`<aside class="chatside stack"><div class="card"><h3>相关资料</h3><a href="#/c/${c.id}/profile">角色资料</a><p><a href="#/c/${c.id}/people/owner">关于你</a></p><a href="#/c/${c.id}/chat/corrections">纠错记录</a></div></aside>`});
    bindAvatars(el);
    const newSession=document.createElement('button');newSession.className='btn ghost sm';newSession.textContent='新会话';el.querySelector('.chat-head-actions').append(newSession);
    const related=document.createElement('button');related.className='btn ghost sm chat-related-button';related.textContent='资料';el.querySelector('.chat-head-actions').append(related);
    related.onclick=()=>drawer({title:`${esc(c.name)} · 相关资料`,body:`<nav class="stack" aria-label="对话相关资料"><a class="btn" href="#/c/${c.id}/profile">角色资料</a><a class="btn" href="#/c/${c.id}/people/owner">关于你</a><a class="btn" href="#/c/${c.id}/chat/corrections">纠错记录</a></nav>`});
    newSession.onclick=async()=>{newSession.disabled=true;if(!newSessionKeys.has(c.id))newSessionKeys.set(c.id,randomId());try{await client.newDirectConversation(c.id,{idempotencyKey:newSessionKeys.get(c.id)});newSessionKeys.delete(c.id);await openConversation(c.id);update();}catch(error){status.textContent=chatFailureText(error.message);feedback.hidden=false;}finally{newSession.disabled=false;}};
    stream=el.querySelector('.stream');form=el.querySelector('form');
    feedback=document.createElement('div');feedback.className='chat-feedback';feedback.innerHTML='<p class="small muted chat-feedback-text" role="status" data-chat-status></p><div class="row chat-feedback-controls" data-chat-controls></div>';form.before(feedback);status=feedback.querySelector('[data-chat-status]');controls=feedback.querySelector('[data-chat-controls]');
    form.elements.t.value=draftText.get(c.id)||ctx.query.say||'';form.elements.t.placeholder=`和${c.name}说点什么…`;
    const resizeComposer=()=>{form.elements.t.style.height='auto';form.elements.t.style.height=Math.min(form.elements.t.scrollHeight,120)+'px';};
    form.elements.t.oninput=()=>{draftText.set(c.id,form.elements.t.value);resizeComposer();};
    form.elements.t.oncompositionstart=()=>{composing=true;};form.elements.t.oncompositionend=()=>{composing=false;};
    form.elements.t.onkeydown=event=>{if(event.key==='Enter'&&!event.shiftKey&&!event.isComposing&&!composing&&event.keyCode!==229){event.preventDefault();form.requestSubmit();}};
    form.onsubmit=event=>{event.preventDefault();if(composing)return;const text=form.elements.t.value.trim();if(!text||state.sending)return;form.elements.t.value='';resizeComposer();draftText.delete(c.id);void state.send(text).then(update);};
    resizeComposer();
    stream.onclick=event=>{const button=event.target.closest('[data-wrong]');if(button)openCorrection(button.dataset.wrong);};
    unsubscribe=state.$subscribe(update,{flush:'sync',detached:true});update();
    const position=readingPositions.get(JSON.stringify([c.id,state.conversation.id]));
    stream.scrollTop=!ctx.query.message&&position&&!position.latest&&position.messageIds===JSON.stringify(state.messages.map(message=>message.id))?position.top:stream.scrollHeight;
    if(ctx.query.message){
      const focus=document.createElement('section');focus.className='card';focus.setAttribute('aria-label','定位到的原消息');locatedMessagePanel=focus;stream.prepend(focus);
      const locate=async()=>{
        focus.innerHTML='<p role="status">正在定位原消息…</p>';
        try {
          const message=await client.conversationMessage(state.conversation.id,ctx.query.message);
          if(!current())return;
          const existing=stream.querySelector(`[data-msg="${CSS.escape(message.id)}"]`);
          if(existing){locatedMessagePanel=null;focus.remove();existing.scrollIntoView({block:'center'});return;}
          focus.innerHTML=`<p class="small muted">定位到的原消息 · 按实际消息ID读取，未改变下方历史分页</p>${MessageBubble(projected(message,c.id),{...options,characterAvatar:c.avatar,characterName:c.name,mediaUrl:id=>`/api/media/${encodeURIComponent(id)}`})}`;
          bindAvatars(focus);
          focus.scrollIntoView({block:'center'});
        } catch(error) {if(current()){focus.innerHTML=`<p class="banner err" role="alert">原消息定位失败：${esc(error.message)}</p><button class="btn" data-locate-retry>重新定位</button>`;focus.querySelector('[data-locate-retry]').onclick=locate;}}
      };
      void locate();
    }
  }).catch(error=>{if(current())el.innerHTML=`<div class="banner err" role="alert">${esc(error.message)}</div>`;});
  const timer=setInterval(()=>{if(current()&&!document.hidden)void conversation().refreshActiveTurn();},2000);
  return()=>{if(stream&&conversation().fluctlightId===c.id){readingPositions.set(JSON.stringify([c.id,conversation().conversation?.id]),{top:stream.scrollTop,messageIds:JSON.stringify(conversation().messages.map(message=>message.id)),latest:stream.scrollHeight-stream.scrollTop-stream.clientHeight<72});if(readingPositions.size>40)readingPositions.delete(readingPositions.keys().next().value);}alive=false;unsubscribe?.();clearInterval(timer);if(conversation().fluctlightId===c.id)conversation().invalidateRequest();};
}
export function openCorrection(messageId){
  const state=conversation(), message=state.messages.find(row=>row.id===messageId)||store.get('messages',messageId);
  if(!message)return toast('当前范围未找到原消息','err');
  const id=message.charId||state.fluctlightId, sourceText=message.text;
  const modal=drawer({title:'这条不对',body:`<div class="card tint-sun"><p>${esc(sourceText)}</p></div><p class="small muted">告诉她会提交真实对话，由她决定如何更新。所有者纠正仅作用于已明确关联且有版本的事实。</p><textarea class="input" rows="4" data-correction></textarea><p role="alert" data-error></p>`,footer:'<button class="btn" data-close>取消</button><button class="btn primary" data-tell>告诉她</button>'});
  modal.el.querySelector('[data-close]').onclick=()=>modal.close();
  modal.el.querySelector('[data-tell]').onclick=async()=>{const text=modal.el.querySelector('[data-correction]').value.trim();if(!text)return;const button=modal.el.querySelector('[data-tell]');button.disabled=true;try{if(state.fluctlightId!==id)await openConversation(id);await state.send(`关于你刚才说的“${sourceText}”：${text}`);if(state.error)throw new Error(state.error);modal.close();}catch(error){modal.el.querySelector('[data-error]').textContent=error.message;button.disabled=false;}};
  const memoryId=message.memory_ref||message.memoryRef;
  const memory=memoryId&&store.get('memories',memoryId);
  if(memory && memory.charId===id && Number.isSafeInteger(memory.version)) {
    const button=document.createElement('button');button.className='btn accent';button.textContent='所有者纠正关联记忆';
    modal.el.querySelector('[data-tell]').after(button);
    button.onclick=async()=>{if(!(await requireOwner()))return;const content=modal.el.querySelector('[data-correction]').value.trim();if(!content)return;button.disabled=true;try{await client.reviseMemory(memory.id,{expectedRevision:memory.version,content,evidenceRefs:[messageId]});const detail=await client.detail(id);const updated=detail.memories?.find(row=>row.id===memory.id);if(!updated||updated.content!==content)throw new Error('纠正已提交，但最新记忆尚未确认一致，请重新读取');modal.close();toast('关联记忆已纠正并读回；原消息保留','ok');}catch(error){modal.el.querySelector('[data-error]').textContent=error.status===409?'事实版本已变化，草稿已保留。请重新读取后核对。':error.message;button.disabled=false;}};
  }
  return modal;
}
