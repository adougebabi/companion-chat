import {errorMessage,stateLabel} from '../../domain-presentation.mjs';
import { randomId } from '../../../random-id.mjs';
import * as store from '../store.js';
import { openConversation, conversation, client } from '../../providers/real.ts';
import { esc, badge, drawer, toast, objLink, requireOwner } from '../ui.js';
import { ChatPanel, MessageBubble } from '../components/chat.js';
import {bindAvatars} from '../components/avatar.js';
import {chatFailureText,chatDiagnosticTurn,chatWaitText,chatContextText} from '../../chat-feedback.mjs';
import { messageTimeLabel } from '../../../lib/message-time';
import {mountChatLifeSummary} from '../components/chat-life-summary.js';
const options={actionById:id=>store.get('actions',id),mediaById:id=>store.get('media',id),link:objLink};
const draftText={
  get(key){try{return sessionStorage.getItem('fluctlight.chat-draft:'+key)||'';}catch{return ''; }},
  set(key,value){try{sessionStorage.setItem('fluctlight.chat-draft:'+key,value);}catch{/* Current mounted input remains available. */}},
  delete(key){try{sessionStorage.removeItem('fluctlight.chat-draft:'+key);}catch{/* Current mounted input has already cleared. */}},
};
const readingPositions=new Map();
const localViews=new WeakMap();
const newSessionKeys=new Map();
const viewModes=new Map();
function projected(message,id){return {id:message.id,charId:id,from:message.kind==='user'?'user':message.kind==='system'?'sys':'char',text:message.text,at:messageTimeLabel(message),turnId:message.turnId,idempotencyKey:message.idempotencyKey,turnStatus:message.turnStatus,turnRetryable:message.turnRetryable,turnErrorCode:message.turnErrorCode,attachmentRefs:message.attachmentRefs||[],refs:message.attachmentRefs||[],sourceRef:message.sourceRef,memoryRef:message.memoryRef||message.memory_ref,goalRef:message.goalRef||message.goal_ref,recall:message.recall,mediaId:message.media_id,kind:message.kind==='image'?'image':undefined};}
export function refreshStream() {}
export function actionDrawer(id){const action=store.get('actions',id);drawer({title:'行动详情',body:action?`<p>${esc(action.title)}</p><pre>${esc(JSON.stringify(action,null,2))}</pre>`:'<p>当前范围未读取到该行动，请进入生活或诊断重新读取。</p>'});}
export function renderChat(el,ctx){
  let alive=true,unsubscribe=null;const c=store.get('characters',ctx.charId), token={};localViews.set(el,token);
  const current=()=>alive&&localViews.get(el)===token;
  if(c.status!=='正常'){el.innerHTML=`<div class="statebox"><h2>${esc(c.name)}${esc(c.status)}</h2><p>${esc(c.statusNote||'当前状态无法发送对话')}</p><a class="btn" href="#/c/${c.id}/profile">查看资料</a></div>`;return;}
  el.innerHTML='<div class="card" role="status">正在读取真实会话…</div>';
  let waitStarted=0,waitIdentity='',waitTimer;
  const updateWait=()=>{if(!current()||!status)return;const state=conversation();const pending=state.sending||state.hasPendingTurn||state.hasSettlingTurn;const original=state.messages.findLast(message=>message.turnId===state.activeTurnId)||state.messages.findLast(message=>message.kind==='user'&&['pending','running'].includes(message.turnStatus));const identity=state.activeTurnId||original?.turnId||state.retryTurn?.turnId||'';if(pending&&(!waitStarted||waitIdentity!==identity)){waitIdentity=identity;waitStarted=Date.parse(original?.createdAt||'')||Date.now();}if(!pending)waitStarted=0;status.textContent=[chatFailureText(state.error),chatWaitText(state,waitStarted),state.queuedTurn?'下一条消息已保留，将在本次回应结束后发送。':''].filter(Boolean).join(' ');};
  let stream,form,status,controls,feedback,locatedMessagePanel,composing=false,lastSignature='',summaryPane,scope='',viewMode='交流',unreadIds=new Set(),knownIds=new Set(),readyToRead=false,readFrame=0,lastReported=0;
  const scopeOf=state=>JSON.stringify([store.get('session')?.actorId,c.id,c.worldId,state.conversation?.id,state.senderActorId||state.authenticatedActorId]);
  const messageOptions=()=>({...options,viewMode,senderName:store.get('user')?.name||'你',characterAvatar:c.avatar,characterName:c.name,mediaUrl:id=>`/api/media/${encodeURIComponent(id)}`});
  const updateLatest=()=>{const button=el.querySelector('[data-latest]');if(!button)return;button.hidden=!unreadIds.size;button.querySelector('[data-new-count]').textContent=unreadIds.size;button.setAttribute('aria-label',`回到最新消息，${unreadIds.size} 条新消息`);};
  const reportVisible=()=>{
    cancelAnimationFrame(readFrame);readFrame=requestAnimationFrame(()=>{
      if(!current()||!readyToRead||!stream||document.hidden)return;
      const rect=stream.getBoundingClientRect();if(!rect.height||!rect.width)return;
      let sequence=0;
      for(const message of conversation().messages){const node=stream.querySelector(`[data-msg="${CSS.escape(message.id)}"]`);if(!node)continue;const bounds=node.getBoundingClientRect();if(bounds.top<rect.bottom&&bounds.bottom>rect.top){if(Number.isSafeInteger(message.sequence)&&message.sequence>0)sequence=Math.max(sequence,message.sequence);unreadIds.delete(message.id);}}
      updateLatest();if(sequence>lastReported){lastReported=sequence;void conversation().reportReadPosition(sequence);}
    });
  };
  const update=()=>{
    if(!current()||!stream)return;const state=conversation();if(state.fluctlightId!==c.id)return;
    const messages=state.messages.map(message=>projected(message,c.id));
    const signature=JSON.stringify(messages);
    if(signature!==lastSignature){const oldHeight=stream.scrollHeight,oldTop=stream.scrollTop,near=oldHeight-oldTop-stream.clientHeight<72;if(readyToRead&&!near)for(const message of messages)if(!knownIds.has(message.id)&&message.from==='char')unreadIds.add(message.id);knownIds=new Set(messages.map(message=>message.id));stream.innerHTML=messages.map(message=>MessageBubble(message,messageOptions())).join('');if(locatedMessagePanel)stream.prepend(locatedMessagePanel);bindAvatars(stream);if(near)stream.scrollTop=stream.scrollHeight;else stream.scrollTop=oldTop;lastSignature=signature;updateLatest();reportVisible();}
    updateWait();
    controls.innerHTML=`${state.nextBeforeSequence?'<button class="btn sm" data-older>加载更早消息</button>':''}${state.canCancel?'<button class="btn sm" data-cancel>取消生成</button>':''}${state.sending||state.hasPendingTurn||state.hasSettlingTurn?'<button class="btn sm" data-query-turn>刷新回复进度</button>':''}${state.canRetry?`<button class="btn sm" data-retry>${state.retryIsModelFailure?'再试一次':'重新发送'}</button>`:''}`;
    const failedTurn=chatDiagnosticTurn(state);
    if(failedTurn){const link=document.createElement('a');link.className='btn ghost sm';link.href='#/diag/turns?correlationId='+encodeURIComponent('turn:'+failedTurn)+'&returnTo='+encodeURIComponent('#'+ctx.raw)+'&returnLabel='+encodeURIComponent('返回对话');link.textContent='本次运行诊断';controls.append(link);}
    if(state.error||state.hasPendingTurn||state.sending){const detail=document.createElement('details');detail.className='small muted';const summary=document.createElement('summary');summary.textContent='连接详情';const pre=document.createElement('pre');pre.textContent=state.error||'正在等待回应';detail.append(summary,pre);const service=document.createElement('a');service.href='#/settings/models';service.textContent='检查模型连接';detail.append(service);if(failedTurn){const link=document.createElement('a');link.href='#/diag/model-runs?correlationId='+encodeURIComponent('turn:'+failedTurn)+'&returnTo='+encodeURIComponent('#'+ctx.raw)+'&returnLabel='+encodeURIComponent('返回对话');link.textContent='查看本次模型调用';detail.append(link);}controls.append(detail);}
    feedback.hidden=!status.textContent&&!controls.children.length;
    feedback.classList.toggle('is-pending',!!(state.sending||state.hasPendingTurn||state.hasSettlingTurn));
    const sendButton=form.querySelector('[data-test="chat-send"]');sendButton.disabled=!!state.queuedTurn;sendButton.textContent=state.sending?(state.queuedTurn?'下一条已保留':'追发一条'):'发送';
    controls.querySelector('[data-older]')?.addEventListener('click',async()=>{const before=stream.scrollHeight,top=stream.scrollTop;await state.loadOlder();update();stream.scrollTop=top+stream.scrollHeight-before;});
    controls.querySelector('[data-query-turn]')?.addEventListener('click',async event=>{const button=event.currentTarget;button.disabled=true;const conversationId=state.conversation?.id;try{const status=await state.refreshActiveTurn(true);if(!current()||conversation().conversation?.id!==conversationId)return;update();toast(status?'回复进度：'+stateLabel(status):'还在等待回应，消息已保留。','info');}catch(error){if(current())toast(errorMessage(error),'err');}finally{if(button.isConnected)button.disabled=false;}});
    controls.querySelector('[data-cancel]')?.addEventListener('click',()=>void state.cancel());
    controls.querySelector('[data-retry]')?.addEventListener('click',()=>void state.retry());
    // The composer remains mounted through every stream/poll update.
  };
  const readScope=async()=>{if(ctx.query.conversation)await client.selectDirectConversation(c.id,ctx.query.conversation);if(!current())throw new DOMException('页面已离开','AbortError');return openConversation(c.id,current);};
  void readScope().then(state=>{
    if(!current())return;
    scope=scopeOf(state);viewMode=viewModes.get(scope)||'交流';
    el.innerHTML=ChatPanel({character:c,messages:[],messageOptions:messageOptions(),viewMode,senderName:store.get('user')?.name||'你',worldName:store.get('worlds',c.worldId)?.name||'',sidebar:'<aside class="chatside" aria-label="她现在" data-life-summary></aside>'});
    summaryPane=mountChatLifeSummary(el.querySelector('[data-life-summary]'),{ctx,scope,character:c,canRefresh:()=>!conversation().sending,onContext:context=>{const line=el.querySelector('[data-chat-now]');if(!line)return;line.textContent=chatContextText(context);line.hidden=!line.textContent;},onFold:folded=>el.querySelector('.chatwrap').classList.toggle('summary-folded',folded),onClose:()=>el.querySelector('.chat-related-button')?.focus()});
    bindAvatars(el);
    const newSession=document.createElement('button');newSession.className='btn ghost sm';newSession.textContent='新会话';el.querySelector('.chat-secondary-actions').append(newSession);
    const diagnosis=document.createElement('a');diagnosis.className='btn ghost sm';diagnosis.href='#/diag';diagnosis.textContent='运行诊断';el.querySelector('.chat-secondary-actions').append(diagnosis);
    const related=document.createElement('button');related.className='btn ghost sm chat-related-button';related.textContent='她现在';el.querySelector('.chat-head-actions').append(related);
    const replySettings=document.createElement('a');replySettings.className='btn ghost sm';replySettings.textContent='回复设置';replySettings.href='#/settings/reply-vision?returnTo='+encodeURIComponent('#'+ctx.raw)+'&returnLabel='+encodeURIComponent('返回对话');el.querySelector('.chat-secondary-actions').append(replySettings);
    related.onclick=()=>{summaryPane.open();el.querySelector('[data-summary-close]')?.focus();};
    el.querySelectorAll('[data-chat-view]').forEach(button=>button.onclick=()=>{viewMode=button.dataset.chatView;viewModes.set(scope,viewMode);el.querySelector('[data-chat-view-help]').hidden=viewMode!=='管理';el.querySelectorAll('[data-chat-view]').forEach(item=>{item.classList.toggle('on',item===button);item.setAttribute('aria-pressed',String(item===button));});lastSignature='';update();});
    newSession.onclick=async()=>{newSession.disabled=true;const requestScope=scope;if(!newSessionKeys.has(requestScope))newSessionKeys.set(requestScope,randomId());try{draftText.set(scope,form.elements.t.value);await client.newDirectConversation(c.id,{idempotencyKey:newSessionKeys.get(requestScope)});newSessionKeys.delete(requestScope);await openConversation(c.id);scope=scopeOf(state);state.attachmentRef='';form.elements.t.value=draftText.get(scope)||'';knownIds=new Set();unreadIds.clear();lastReported=0;update();toast('已开始新会话，可以发送第一条消息','ok');}catch(error){status.textContent=chatFailureText(error.message);feedback.hidden=false;}finally{newSession.disabled=false;}};
    stream=el.querySelector('.stream');form=el.querySelector('form');
    feedback=document.createElement('div');feedback.className='chat-feedback';feedback.innerHTML='<p class="small muted chat-feedback-text" role="status" data-chat-status></p><div class="row chat-feedback-controls" data-chat-controls></div>';form.before(feedback);status=feedback.querySelector('[data-chat-status]');controls=feedback.querySelector('[data-chat-controls]');
    state.attachmentRef='';draftText.delete(scope+':attachment');
    form.elements.t.value=draftText.get(scope)||'';form.elements.t.placeholder=`和${c.name}说点什么…`;
    const resizeComposer=()=>{form.elements.t.style.height='auto';form.elements.t.style.height=Math.min(form.elements.t.scrollHeight,120)+'px';};
    form.elements.t.oninput=()=>{draftText.set(scope,form.elements.t.value);resizeComposer();};
    form.elements.t.oncompositionstart=()=>{composing=true;};form.elements.t.oncompositionend=()=>{composing=false;};
    form.elements.t.onkeydown=event=>{if(event.key==='Enter'&&!event.shiftKey&&!event.isComposing&&!composing&&event.keyCode!==229){event.preventDefault();form.requestSubmit();}};
    form.onsubmit=event=>{event.preventDefault();if(composing)return;const text=form.elements.t.value.trim();if(!text){toast('请先输入消息','err');form.elements.t.focus();return;}if(state.queuedTurn)return;form.elements.t.value='';resizeComposer();draftText.delete(scope);stream.scrollTop=stream.scrollHeight;void state.send(text).then(update);};
    resizeComposer();
    stream.addEventListener('scroll',reportVisible,{passive:true});
    stream.onclick=async event=>{const button=event.target.closest('[data-source]');if(!button)return;event.stopPropagation();const message=state.messages.find(row=>row.id===button.dataset.source);if(!message)return;drawer({title:'消息来源与依据',body:`<p>${esc(message.text)}</p><div class="stack">${message.memoryRef?`<a class="btn" href="#/c/${encodeURIComponent(c.id)}/growth/memory/${encodeURIComponent(message.memoryRef)}">查看关联记忆</a>`:''}${message.sourceRef?`<p>来源记录：${esc(message.sourceRef)}</p>`:'<p class="muted">这条消息尚无可直接查阅的来源引用。</p>'}<details class="mgmt"><summary>原消息与处理记录</summary><pre>${esc(JSON.stringify({conversationId:state.conversation.id,messageId:message.id,turnId:message.turnId,sourceRef:message.sourceRef,attachmentRefs:message.attachmentRefs},null,2))}</pre></details></div>`});};
    el.querySelector('[data-latest]').onclick=()=>{stream.scrollTop=stream.scrollHeight;reportVisible();};
    waitTimer=setInterval(()=>{const state=conversation();if(state.sending||state.hasPendingTurn||state.hasSettlingTurn)updateWait();},1000);
    unsubscribe=state.$subscribe(update,{flush:'sync',detached:true});update();
    const position=readingPositions.get(JSON.stringify([c.id,state.conversation.id]));
    stream.scrollTop=!ctx.query.message&&position&&!position.latest&&position.messageIds===JSON.stringify(state.messages.map(message=>message.id))?position.top:stream.scrollHeight;
    readyToRead=!ctx.query.message;if(readyToRead)reportVisible();
    if(ctx.query.message){
      const focus=document.createElement('section');focus.className='card located-message';focus.setAttribute('aria-label','定位到的原消息');locatedMessagePanel=focus;stream.prepend(focus);
      const locate=async()=>{
        focus.innerHTML='<p role="status">正在定位原消息…</p>';
        try {
          const message=await client.conversationMessage(state.conversation.id,ctx.query.message);
          if(!current())return;
          const existing=stream.querySelector(`[data-msg="${CSS.escape(message.id)}"]`);
          if(existing){locatedMessagePanel=null;focus.remove();existing.classList.add('located-message');existing.scrollIntoView({block:'center'});return;}
          focus.innerHTML=`<p class="small muted">定位到的原消息</p>${MessageBubble(projected(message,c.id),messageOptions())}`;
          bindAvatars(focus);
          focus.scrollIntoView({block:'center'});
        } catch(error) {if(current()){focus.innerHTML=`<p class="banner err" role="alert">原消息定位失败：${esc(errorMessage(error))}</p><button class="btn" data-locate-retry>重新定位</button>`;focus.querySelector('[data-locate-retry]').onclick=locate;}}
      };
      void locate();
    }
  }).catch(error=>{if(current()){el.innerHTML=`<div class="statebox" role="alert"><h2>对话读取失败</h2><p>${esc(errorMessage(error))}</p><button class="btn" data-chat-read-retry>重新读取会话</button></div>`;el.querySelector('[data-chat-read-retry]').onclick=()=>ctx.refresh();}});
  const timer=setInterval(()=>{if(current()&&!document.hidden)void conversation().refreshActiveTurn();},2000);
  return()=>{if(stream&&conversation().fluctlightId===c.id){readingPositions.set(JSON.stringify([c.id,conversation().conversation?.id]),{top:stream.scrollTop,messageIds:JSON.stringify(conversation().messages.map(message=>message.id)),latest:stream.scrollHeight-stream.scrollTop-stream.clientHeight<72});if(readingPositions.size>40)readingPositions.delete(readingPositions.keys().next().value);}alive=false;summaryPane?.dispose();cancelAnimationFrame(readFrame);unsubscribe?.();clearInterval(waitTimer);clearInterval(timer);if(conversation().fluctlightId===c.id)conversation().invalidateRequest();};
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
