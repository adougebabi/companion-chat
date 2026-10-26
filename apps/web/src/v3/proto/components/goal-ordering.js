import {esc} from '../ui.js';

function cyclic(graph){
 const active=new Set(),done=new Set();
 function visit(id){if(active.has(id))return true;if(done.has(id))return false;active.add(id);for(const prior of graph[id]||[])if(visit(prior))return true;active.delete(id);done.add(id);return false;}
 return Object.keys(graph).some(visit);
}
export function mountGoalOrdering(host,{goals,dependencies,mode='automatic',initialOrder=[],initialSelected='',onChange=()=>{}}){
 const names=new Map(goals.map(goal=>[goal.id,goal.desired_outcome||goal.description||'未命名目标']));
 let order=[...initialOrder.filter(id=>names.has(id)),...goals.map(goal=>goal.id).filter(id=>!initialOrder.includes(id))],graph=structuredClone(dependencies),selected=names.has(initialSelected)?initialSelected:order[0],message='';
 function paint(focus=''){
  const at=order.indexOf(selected);
  host.innerHTML=`<section class="goal-ordering"><h3>目标顺序与前置条件</h3><p class="small muted">选择一个目标，再调整顺序或前置条件。完成并确认有效后，才能推进依赖它的目标。</p><div class="goal-order-list">${order.map((id,index)=>`<div class="goal-order-row ${id===selected?'on':''}"><button data-domain-ui="button" type="button" class="goal-order-select" data-select-goal="${esc(id)}" aria-pressed="${id===selected}"><span class="small muted">${index+1}</span><span>${esc(names.get(id))}</span></button>${id===selected?`<div class="row"><button data-domain-ui="button" type="button" class="btn sm" data-move="-1" ${mode!=='manual'||at<=0?'disabled':''} aria-label="上移所选目标">↑</button><button data-domain-ui="button" type="button" class="btn sm" data-move="1" ${mode!=='manual'||at===order.length-1?'disabled':''} aria-label="下移所选目标">↓</button></div>`:''}</div>`).join('')||'<p class="small muted">当前没有可排序的目标。</p>'}</div>${mode!=='manual'?'<p class="small muted">当前由系统排序。选择“手动排序”后可上移、下移目标。</p>':''}${selected?`<fieldset class="goal-prerequisites"><legend>${esc(names.get(selected))}的前置条件</legend>${order.filter(id=>id!==selected).map(id=>`<label class="outfit-choice"><input data-domain-ui="field" type="checkbox" data-prerequisite="${esc(id)}" ${(graph[selected]||[]).includes(id)?'checked':''}><span>${esc(names.get(id))}</span></label>`).join('')||'<p class="small muted">当前没有其它可选目标。</p>'}${(graph[selected]||[]).filter(id=>!names.has(id)).map(id=>`<p class="small muted">保留的前置目标已不在当前列表中 <button data-domain-ui="button" type="button" class="btn ghost sm" data-remove-prior="${esc(id)}">移除这项前置条件</button></p>`).join('')}</fieldset>`:''}<p data-domain-ui="alert" class="small muted" role="alert" data-order-error>${esc(message)}</p><textarea data-domain-ui="field" name="order" hidden>${esc(order.join('\n'))}</textarea><textarea data-domain-ui="field" name="dependencies" hidden>${esc(JSON.stringify(graph))}</textarea></section>`;
  host.querySelectorAll('[data-select-goal]').forEach(button=>button.onclick=()=>{selected=button.dataset.selectGoal;message='';paint(selected);});
  host.querySelectorAll('[data-move]').forEach(button=>button.onclick=()=>{const index=order.indexOf(selected),next=index+Number(button.dataset.move);if(mode!=='manual'||next<0||next>=order.length)return;[order[index],order[next]]=[order[next],order[index]];message='顺序已调整，保存策略后生效。';paint(selected);});
  host.querySelectorAll('[data-prerequisite]').forEach(input=>input.onchange=()=>{
   const id=input.dataset.prerequisite,next=structuredClone(graph);next[selected]=(next[selected]||[]).filter(value=>value!==id);if(input.checked)next[selected].push(id);
   if(cyclic(next))message='这项前置条件会形成循环，已保留原来的选择。';else{graph=next;message='前置条件已调整，保存策略后生效。';}
   paint();host.querySelector(`[data-prerequisite="${CSS.escape(id)}"]`)?.focus();
  });
  host.querySelectorAll('[data-remove-prior]').forEach(button=>button.onclick=()=>{graph[selected]=graph[selected].filter(id=>id!==button.dataset.removePrior);paint(selected);});
  onChange({order:[...order],dependencies:structuredClone(graph),selected});
  if(focus)host.querySelector(`[data-select-goal="${CSS.escape(focus)}"]`)?.focus();
 }
 paint();return {setMode(value){mode=value;paint();}};
}
