import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {React,act,mountReact,installDOM} from './helpers/react-dom.mjs';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const load=path=>vite.ssrLoadModule('/src/'+path);
const {useController}=await load('react-app/page.tsx');
const {FieldInput,Textarea,ChoiceSelect}=await load('react-app/controls.tsx');
const {default:ProfileField}=await load('react-app/shared/ProfileField.tsx');
const workspace={subscribe:()=>()=>{}};
function nativeInput(node,value){Object.getOwnPropertyDescriptor(node.tagName==='TEXTAREA'?window.HTMLTextAreaElement.prototype:window.HTMLInputElement.prototype,'value').set.call(node,value);node.dispatchEvent(new window.Event('input',{bubbles:true}));}
test('profile text controls stay mounted and focused crossing 80 characters in either direction',async()=>{
 let field;
 function View(){return useController((_p,_e,c)=>{field=c.model({path:['identity','occupation'],kind:'text',label:'职业',value:'a'.repeat(80)});return()=>React.createElement(ProfileField,{field});},{});}
 const v=await mountReact(vite,View,{workspace});try{const input=v.host.querySelector('input');input.focus();await v.input(input,'a'.repeat(81));assert.equal(v.host.querySelector('input'),input);assert.equal(document.activeElement,input);assert.equal(input.value.length,81);await v.input(input,'a'.repeat(79));assert.equal(v.host.querySelector('input'),input);assert.equal(document.activeElement,input);field.value='外部更新';await v.flush();assert.equal(input.value,'外部更新');}finally{await v.dispose();}
});
test('controller inputs preserve the immediate native value and composition across background notifications',async()=>{
 for(const Component of [FieldInput,Textarea]){
  let cell,background;
  function View(){return useController((_p,_e,c)=>{cell=c.cell('abc');background=c.cell(0);return()=>React.createElement('div',null,React.createElement(Component,{value:cell.value,onChange:e=>cell.value=e.target.value}),String(background.value));},{});}
  const v=await mountReact(vite,View,{workspace});try{const input=v.host.querySelector('input,textarea');input.focus();act(()=>{input.dispatchEvent(new window.CompositionEvent('compositionstart',{bubbles:true}));nativeInput(input,'abcni');assert.equal(input.value,'abcni');background.value++;});await v.flush();assert.equal(input.value,'abcni');assert.equal(document.activeElement,input);act(()=>{nativeInput(input,'abc你好');input.dispatchEvent(new window.CompositionEvent('compositionend',{bubbles:true,data:'你好'}));});await v.flush();assert.equal(input.value,'abc你好');assert.equal(cell.value,'abc你好');cell.value='';await v.flush();assert.equal(input.value,'');assert.equal(v.host.querySelector('input,textarea'),input);}finally{await v.dispose();}
 }
});
test('select normalization removes duplicate empty choices while preserving exact domain value types',async()=>{
 let chosen;
 function View(){return React.createElement(ChoiceSelect,{value:'',onChange:e=>chosen=e.target.value},['','',false,0].map((value,index)=>React.createElement('option',{key:index,value},String(value)||'尚未填写')));}
 const v=await mountReact(vite,View);try{await v.click(v.host.querySelector('[role=combobox]'));const options=[...document.querySelectorAll('[role=option]')];assert.equal(options.filter(n=>n.textContent==='尚未填写').length,1);await v.click(options.find(n=>n.textContent==='0'));assert.equal(chosen,0);}finally{await v.dispose();}
});
test('diagnostic records use real table cells with separate actions rather than a button owning the row',async()=>{
 const {default:Records}=await load('react-app/shared/DiagnosticRecords.tsx');let selected,retired;
 const record={id:'run',label:'模型调用',character:'尚未关联角色',status:'error',summary:'保留的失败原因'.repeat(25),durationLabel:'120 秒',start:Date.now()};
 const view={rows:{value:[record]},recordTime:()=> '完整时间',statusLabel:()=> '失败',selectRecord:r=>selected=r.id,retry:r=>retired=r.id,clear(){}};
 const v=await mountReact(vite,Records,{props:{view}});try{const row=v.host.querySelector('tbody tr');assert.ok(row);assert.equal(row.querySelectorAll('td').length,7);assert.equal(row.querySelector('.diagnostic-summary').closest('button'),null);assert.match(row.textContent,/120 秒/);assert.doesNotMatch(row.textContent,/尚未关联角色/);await v.click(v.host.querySelector('[data-testid=diag-open-run]'));assert.equal(selected,'run');await v.click(v.host.querySelector('[data-testid=diag-retry-run]'));assert.equal(retired,'run');}finally{await v.dispose();}
});
test('malformed Studio world structure stays editable as complete JSON instead of crashing or overwriting its data',async()=>{
 const {default:Definition}=await vite.ssrLoadModule('/src/react-app/shared/StudioDefinitionForm.tsx');const original={schema:'world-definition.v1',background:'原背景',framework:{venues:[null]}};let changed;
 const v=await mountReact(vite,Definition,{props:{kind:'world',value:original,onChange:value=>changed=value}});
 try{assert.match(v.host.querySelector('[role=alert]').textContent,/完整 JSON/);const add=[...v.host.querySelectorAll('button')].find(node=>node.textContent==='添加场地分类');assert.equal(add.disabled,true);assert.deepEqual(original.framework.venues,[null]);assert.equal(changed,undefined);}finally{await v.dispose();}
});
