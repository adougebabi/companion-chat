import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {React,act,installDOM,mountReact} from './helpers/react-dom.mjs';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const {default:DomainPrimitives}=await vite.ssrLoadModule('/src/react-app/shared/DomainPrimitives.tsx');
const {Field}=await vite.ssrLoadModule('/src/v3/proto/components/forms.js');
test.after(async()=>{await vite.close();env.restore();});
function Harness({prepare,html}){const [element,setElement]=React.useState(null);return React.createElement(React.Fragment,null,React.createElement('section',{ref:React.useCallback(node=>{if(node){node.innerHTML=html;prepare(node);setElement(node);}},[])}),React.createElement(DomainPrimitives,{root:element}));}
const visible=(host,selector)=>[...host.querySelectorAll(selector)].filter(node=>!node.closest('[data-domain-carrier]'));
test('one application tree ports exact native form values, original listeners and submission while preserving a long focused draft',async()=>{
 let form,submitted,inputs=0;const html='<form>'+Field({label:'正文',name:'text',value:'x'.repeat(79),type:'textarea'})+Field({label:'类型',name:'kind',value:'',type:'select',options:[{value:'',label:'尚未填写'},{value:'0',label:'零'},{value:'false',label:'否'}]})+'<label><input data-domain-ui="field" type="checkbox" name="favorite">收藏</label><button data-domain-ui="button" type="submit" class="btn primary">保存</button></form>';
 const view=await mountReact(vite,Harness,{props:{html,prepare:root=>{form=root.querySelector('form');form.elements.text.oninput=()=>inputs++;form.onsubmit=event=>{event.preventDefault();submitted=[...new window.FormData(form)];};}}});
 try{
  const input=visible(view.host,'textarea')[0];input.focus();await view.input(input,'x'.repeat(81)+'你好');assert.equal(document.activeElement,input);assert.equal(form.elements.text.value,'x'.repeat(81)+'你好');assert.equal(inputs,1);assert.equal(visible(view.host,'textarea')[0],input);
  const checkbox=visible(view.host,'button[role=checkbox]')[0];await view.click(checkbox);assert.equal(form.elements.favorite.checked,true);
  await act(async()=>{form.elements.kind.value='false';});await view.flush();assert.match(visible(view.host,'button[role=combobox]')[0].textContent,/否/);
  const submit=visible(view.host,'button').find(node=>node.textContent==='保存');await view.click(submit);assert.deepEqual(submitted,[['text','x'.repeat(81)+'你好'],['kind','false'],['favorite','on']]);
  await act(async()=>{form.querySelector('[data-domain-ui=button]').disabled=true;});await view.flush();assert.equal(submit.disabled,true);
 }finally{await view.dispose();}
});
test('Collapsible keeps original nested controls and callbacks, reflects controller open state and removes ports on page replacement',async()=>{
 let root,detail,actions=0;const html='<details data-domain-ui="disclosure"><summary>技术详情</summary><section>'+Field({label:'理由',name:'reason',value:'原输入'})+'<button data-domain-ui="button" class="btn" data-action>重试原提交</button></section></details>';
 const view=await mountReact(vite,Harness,{props:{html,prepare:node=>{root=node;detail=node.querySelector('details');node.querySelector('[data-action]').onclick=()=>actions++;}}});
 try{
  let trigger=visible(view.host,'button').find(node=>node.textContent==='技术详情');assert.equal(trigger.getAttribute('aria-expanded'),'false');await view.click(trigger);assert.equal(detail.open,true);
  const retry=visible(view.host,'button').find(node=>node.textContent==='重试原提交');assert.ok(retry);await view.click(retry);assert.equal(actions,1);
  await act(async()=>{detail.open=false;});await view.flush();assert.equal(trigger.getAttribute('aria-expanded'),'false');
  await act(async()=>{root.innerHTML='<p>新页面</p>';});await view.flush();assert.equal(root.querySelectorAll('[data-domain-react-control]').length,0);assert.equal(root.textContent,'新页面');
 }finally{await view.dispose();}
});
test('radio selection keeps the original group checked state, change listener and single FormData value',async()=>{
 let form,point;const html='<form><label><input data-domain-ui="field" name="point" value="1" type="radio">第一个检查点</label><label><input data-domain-ui="field" name="point" value="2" type="radio">第二个检查点</label></form>';
 const view=await mountReact(vite,Harness,{props:{html,prepare:node=>{form=node.querySelector('form');form.querySelectorAll('input').forEach(input=>input.onchange=()=>point=input.value);}}});
 try{await view.click(visible(view.host,'input[type=radio]')[0]);assert.equal(point,'1');await view.click(visible(view.host,'input[type=radio]')[1]);assert.equal(point,'2');assert.equal(form.elements.point.value,'2');assert.deepEqual([...new window.FormData(form)],[['point','2']]);assert.equal(visible(view.host,'input[type=radio]')[0].checked,false);}finally{await view.dispose();}
});
test('controller redraw resolves the focused carrier and restores the same selection to the new React control',async()=>{
 let root;const html='<section>'+Field({label:'补充要求',name:'instruction',value:'保持描述'})+'</section>';
 const view=await mountReact(vite,Harness,{props:{html,prepare:node=>root=node}});
 try{const input=visible(view.host,'input')[0];input.focus();await view.input(input,'保留后续输入与光标');input.setSelectionRange(3,6);
  await act(async()=>{const original=document.activeElement.closest('[data-domain-react-control]').domainSource;const position=[original.selectionStart,original.selectionEnd];assert.equal(original.name,'instruction');root.innerHTML='<section>'+Field({label:'补充要求',name:'instruction',value:original.value})+'</section>';const next=root.querySelector('[name=instruction]');next.focus();next.setSelectionRange(...position);});await view.flush();const next=visible(view.host,'input')[0];assert.equal(document.activeElement,next);assert.equal(next.value,'保留后续输入与光标');assert.equal(next.selectionStart,3);assert.equal(next.selectionEnd,6);
 }finally{await view.dispose();}
});

test('rich disclosure summaries retain structured labels and settings Sheets reopen the same form after Escape',async()=>{
 let root;const html='<details data-domain-ui="disclosure"><summary><span class="schedule-time">09:00 — 10:00</span><b>阅读</b><span class="badge">计划</span></summary><p>真实安排</p></details><details data-domain-ui="disclosure" data-domain-surface="sheet"><summary>约定与日程设置</summary><form>'+Field({label:'偏好',name:'preference',value:'原设置'})+'</form></details>';
 const v=await mountReact(vite,Harness,{props:{html,prepare:node=>root=node}});try{const row=visible(v.host,'button').find(node=>node.textContent.includes('阅读'));assert.ok(row.querySelector('.schedule-time'));assert.ok(row.querySelector('b'));const trigger=visible(v.host,'button').find(node=>node.textContent==='约定与日程设置');await v.click(trigger);let input=document.querySelector('[role=dialog] input:not([data-domain-carrier])');assert.ok(input);await v.input(input,'保留新的偏好');await act(async()=>document.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Escape',bubbles:true})));await v.flush();assert.equal(document.querySelector('[role=dialog]'),null);await v.click(trigger);input=document.querySelector('[role=dialog] input:not([data-domain-carrier])');assert.ok(input);assert.equal(input.value,'保留新的偏好');assert.equal(root.querySelector('details[data-domain-surface=sheet]').open,true);}finally{await v.dispose();}
});

test('datetime fields use one original form value while date and time edits keep the local datetime contract',async()=>{
 let form,inputs=0,changes=0;const html='<form>'+Field({label:'期限',name:'due',type:'datetime-local',value:'2026-10-27T10:15:30.123'})+'</form>';
 const v=await mountReact(vite,Harness,{props:{html,prepare:node=>{form=node.querySelector('form');form.elements.due.oninput=()=>inputs++;form.elements.due.onchange=()=>changes++;}}});try{assert.equal(visible(v.host,'input[type=datetime-local]').length,0);assert.equal(visible(v.host,'button[role=combobox]').length,1);const time=visible(v.host,'input').find(node=>node.getAttribute('aria-label')==='期限时间');assert.ok(time);assert.equal(time.checkValidity(),true);assert.equal(form.elements.due.value,'2026-10-27T10:15:30.123');assert.equal(form.elements.due.checkValidity(),true);await v.input(time,'11:45');assert.equal(inputs,1);assert.equal(changes,1);assert.deepEqual([...new window.FormData(form)],[['due','2026-10-27T11:45']]);}finally{await v.dispose();}
});
