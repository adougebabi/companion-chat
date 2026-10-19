import { client, openConversation } from '../../providers/real.ts';
import { esc } from '../ui.js';
import { messageTimeLabel } from '../../../lib/message-time';
export function renderHistory(el,ctx){
  let alive=true,busy=false,rows=[],sessions=[],messageCursor='',directoryCursor='',currentId='',failure='';
  const keyword=ctx.query.q||'';
  el.innerHTML='<div class="card" role="status">正在读取会话历史…</div>';
  function paint(){
    if(!alive)return;
    el.innerHTML=`<form class="row"><input class="input grow" name="q" value="${esc(keyword)}" placeholder="搜索这个角色的全部会话…"><button class="btn primary">搜索</button></form><details class="card" style="margin-top:12px" ${keyword?'':'open'}><summary>会话列表</summary><div class="list">${sessions.map(row=>`<button class="lrow" data-session="${esc(row.conversation.id)}"><span class="grow">${esc(row.conversation.title||'私聊会话')} ${row.conversation.id===currentId?'（当前）':''}</span><span class="small muted">${esc(row.conversation.createdAt)} · 未读 ${row.unreadCount}</span></button>`).join('')||'<p>尚无会话</p>'}</div>${directoryCursor?'<button class="btn" data-more-sessions>加载更多会话</button>':''}</details><p class="small muted">${keyword?`全会话搜索“${esc(keyword)}”`:'当前会话消息'} · 已读取 ${rows.length} 条</p><div class="card"><div class="list">${rows.map(message=>`<button class="lrow" data-message="${esc(message.id)}" data-conversation="${esc(message.conversationId)}"><span class="grow">${esc(message.text)}</span><span class="small muted">${esc(messageTimeLabel(message))}</span></button>`).join('')||'<p>当前范围没有匹配消息。</p>'}</div></div>${messageCursor?'<button class="btn" data-older>加载更多消息</button>':''}<p role="alert" data-error>${esc(failure)}</p>`;
    el.querySelector('form').onsubmit=event=>{event.preventDefault();ctx.navigate(`#/c/${ctx.charId}/chat/history?q=${encodeURIComponent(event.target.elements.q.value)}`);};
    el.querySelector('[data-older]')?.addEventListener('click',()=>void loadMessages(true));
    el.querySelector('[data-more-sessions]')?.addEventListener('click',()=>void loadDirectory(true));
    el.querySelectorAll('[data-session],[data-message]').forEach(button=>button.onclick=async()=>{if(busy)return;busy=true;button.disabled=true;try{await client.selectDirectConversation(ctx.charId,button.dataset.session||button.dataset.conversation);if(alive)ctx.navigate(`#/c/${ctx.charId}/chat${button.dataset.message?`?message=${encodeURIComponent(button.dataset.message)}`:''}`);}catch(error){failure=error.message;paint();}finally{busy=false;}});
  }
  async function loadDirectory(append=false){
    if(append&&busy)return;busy=true;
    try{const page=await client.conversationDirectory(ctx.charId,{limit:30,...(append?{cursor:directoryCursor}:{})});if(!alive)return;sessions=append?[...sessions,...page.items]:page.items;directoryCursor=page.nextCursor;currentId=page.currentConversationId;failure='';paint();}
    catch(error){failure=error.message;paint();}finally{busy=false;}
  }
  async function loadMessages(append=false){
    if(append&&busy)return;busy=true;
    try{
      const page=keyword?await client.searchConversationMessages(ctx.charId,keyword,{limit:50,...(append?{cursor:messageCursor}:{})}):await client.messages(currentId,append?Number(messageCursor):undefined,50);
      if(!alive)return;
      const values=keyword?page.items:page.messages;
      rows=append?[...rows,...values]:values;
      messageCursor=String(keyword?page.nextCursor||'':page.nextBeforeSequence||'');failure='';paint();
    }catch(error){failure=error.message;paint();}finally{busy=false;}
  }
  void loadDirectory().then(()=>{if(alive&&currentId)return loadMessages();});
  return()=>{alive=false;};
}
