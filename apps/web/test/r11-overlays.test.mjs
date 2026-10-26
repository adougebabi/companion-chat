import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {React,act,mountReact,installDOM} from './helpers/react-dom.mjs';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const load=path=>vite.ssrLoadModule('/src/'+path);
const {default:Host}=await load('react-app/shared/OverlayHost.tsx'),{confirm,drawer,closeOverlays}=await load('v3/proto/components/overlay.js');
const {Button}=await load('components/ui/button.tsx');
test('shared confirmation uses one Radix AlertDialog, Escape cancels once and restores its connected trigger',async()=>{
 const decisions=[];
 function View(){const [busy,setBusy]=React.useState(false);return React.createElement(React.Fragment,null,React.createElement(Button,{disabled:busy,onClick:async()=>{setBusy(true);decisions.push(await confirm({title:'核对 <b>资源</b>',text:'保留 &lt;用户正文&gt;',okText:'采用'}));setBusy(false);}},'打开确认'),React.createElement(Host));}
 const v=await mountReact(vite,View);try{const trigger=v.host.querySelector('button');trigger.focus();await v.click(trigger);const dialog=document.querySelector('[role=alertdialog]');assert.ok(dialog);assert.equal(document.querySelectorAll('[role=dialog]').length,0);assert.match(dialog.textContent,/核对 资源/);assert.match(dialog.textContent,/保留 <用户正文>/);act(()=>dialog.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Escape',bubbles:true})));await v.flush();assert.deepEqual(decisions,[false]);assert.ok(document.activeElement===trigger,'confirmation restores its source button after it is re-enabled');assert.equal(document.querySelector('[role=alertdialog]'),null);}finally{act(()=>closeOverlays());await v.dispose();}
});
test('domain drawer retains its original content node and handlers in the application Sheet, including nested confirmation',async()=>{
 let api,connected=false,closed=0,decision;
 function View(){return React.createElement(React.Fragment,null,React.createElement(Button,{onClick:()=>{api=drawer({title:'原提交',body:'<label>原理由<input name="reason" value="保留草稿"></label><button type="button" data-check>进一步确认</button>',onClose:()=>closed++});connected=api.el.isConnected;api.el.querySelector('[data-check]').onclick=async()=>{decision=await confirm({text:'确认核对原提交'});};}},'打开抽屉'),React.createElement(Host));}
 const v=await mountReact(vite,View);try{await v.click(v.host.querySelector('button'));assert.equal(connected,true);const input=api.el.querySelector('input');await v.input(input,'随后编辑的草稿');api.setFooter('<span>原提交保持</span>');assert.equal(api.el.querySelector('input'),input);assert.equal(input.value,'随后编辑的草稿');await v.click(api.el.querySelector('[data-check]'));assert.equal(document.querySelectorAll('[role=dialog]').length,1);assert.ok(document.querySelector('[role=alertdialog]'));await v.click([...document.querySelectorAll('[role=alertdialog] button')].find(n=>n.textContent==='取消'));assert.equal(decision,false);assert.equal(api.el.querySelector('input'),input);await v.click(document.querySelector('[data-slot=sheet-close]'));assert.equal(closed,1);assert.equal(api.el.isConnected,false);}finally{act(()=>closeOverlays());await v.dispose();}
});
