import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
import {createServer} from 'vite';
import {proxy, snapshot, subscribe} from 'valtio/vanilla';
import {React, installDOM, mountReact} from './helpers/react-dom.mjs';

const env=installDOM();
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const load=path=>vite.ssrLoadModule('/src/'+path);
const source=path=>readFile(new URL('../src/'+path,import.meta.url),'utf8');
test.after(async()=>{await vite.close();env.restore();});
function workspace(){
  localStorage.clear();sessionStorage.clear();
  const state=proxy({ready:true,session:{actorId:'r9-owner'},selectedId:'',characters:[],errors:{},loading:{}});
  let notificationReads=0;
  return {state,current:{value:{id:'',name:''}},get notificationReads(){return notificationReads;},subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),currentScope:()=>JSON.stringify(['r9-owner','']),
    services:{configurationClient:{read:async()=>({endpoints:[],bindings:[]})},diagnosticsClient:{page:async()=>({items:[],next_cursor:''})},operationsClient:{notifications:async()=>{notificationReads++;return {items:[],as_of:'r9-as-of',next_cursor:''};},pending:async()=>({groups:[]})},studioClient:{}},
    client:{listStudioResources:async()=>({items:[],next_cursor:''}),listActorGroups:async()=>[],worldCatalog:async()=>({items:[]})},
    refreshNotificationCount:async()=>{},refreshDirectory:async()=>{},navigatePath(){},navigate(){},select(){},load:async()=>{}};
}
function icon(node){assert.ok(node);assert.equal(node.dataset.size,'icon');assert.ok(node.classList.contains('size-9'));assert.ok(node.classList.contains('p-0'));}

test('asChild anchors retain shadcn focus utilities and outline theme cannot erase their ring',async()=>{
  const {Button}=await load('components/ui/button.tsx');
  const view=await mountReact(vite,()=>React.createElement('div',null,...['default','outline','secondary','ghost','link','destructive'].map(variant=>React.createElement(Button,{key:variant,variant,asChild:true},React.createElement('a',{href:'#/'+variant},variant)))));
  try{
    for(const link of view.host.querySelectorAll('a')){
      assert.equal(link.dataset.slot,'button');assert.equal(link.getAttribute('href'),'#/'+link.dataset.variant);
      assert.ok(link.classList.contains('focus-visible:ring-[3px]'));
      assert.ok(link.classList.contains('focus-visible:ring-ring/50')||link.classList.contains('focus-visible:ring-destructive/20'));
      link.focus();assert.equal(document.activeElement,link);
    }
    const css=await source('styles/workspace-replica.css');
    const outlines=[...css.matchAll(/([^{}]+\[data-variant="outline"\][^{}]*)\{([^{}]*)\}/g)];
    assert.ok(outlines.length>=2);
    for(const [,selector,body] of outlines)assert.doesNotMatch(body,/(?:box-shadow|--tw-ring-shadow)\s*:/,selector+' must leave the primitive focus ring intact');
  }finally{await view.dispose();}
});

test('home outline links and search/view icon actions use the actual shared Button',async()=>{
  const {default:Library}=await load('react-app/pages/LibraryPage.jsx'),w=workspace();
  localStorage.setItem('fluctlight-creation:r9-owner:draft',JSON.stringify({version:1,step:0,requestId:'r9-draft'}));
  const view=await mountReact(vite,Library,{workspace:w});
  try{
    for(const text of ['继续创建','先连接模型']){
      const link=[...view.host.querySelectorAll('a')].find(n=>n.textContent===text);assert.ok(link,text);assert.equal(link.dataset.variant,'outline');assert.ok(link.classList.contains('focus-visible:ring-[3px]'));
    }
    icon(view.host.querySelector('[aria-label="网格视图"]'));icon(view.host.querySelector('[aria-label="列表视图"]'));
    await view.input(view.host.querySelector('[aria-label="搜索名字、职业或标签"]'),'查找');icon(view.host.querySelector('[aria-label="清除搜索"]'));
    await view.click(view.host.querySelector('[aria-label="清除搜索"]'));assert.equal(view.host.querySelector('[aria-label="搜索名字、职业或标签"]').value,'');
  }finally{await view.dispose();}
});

test('inbox and diagnostic reloads are icon Buttons and preserve their read actions',async()=>{
  const {default:Inbox}=await load('react-app/pages/InboxPage.jsx'),w=workspace();
  let view=await mountReact(vite,Inbox,{workspace:w});
  try{const reload=view.host.querySelector('[aria-label="重新读取通知"]');icon(reload);const before=w.notificationReads;await view.click(reload);assert.equal(w.notificationReads,before+1);}finally{await view.dispose();}
  const {default:Diagnostics}=await load('react-app/shared/DiagnosticsWorkspace.jsx');
  view=await mountReact(vite,Diagnostics,{workspace:workspace(),props:{view:{openTaskDiagnostics(){}}}});
  try{icon(view.host.querySelector('[aria-label="重新读取运行任务"]'));}finally{await view.dispose();}
});

test('world refresh/query are icon Buttons and archived checkbox remains a Radix control',async()=>{
  const {default:Studio}=await load('react-app/pages/StudioDirectory.jsx');
  const view=await mountReact(vite,Studio,{workspace:workspace(),props:{kind:'world'}});
  try{
    icon(view.host.querySelector('[aria-label="刷新创作资源"]'));icon(view.host.querySelector('[aria-label="查询"]'));
    const archived=view.host.querySelector('[aria-label="已归档资源"]');assert.equal(archived.dataset.slot,'checkbox');assert.equal(archived.getAttribute('aria-checked'),'false');
    await view.click(archived);assert.equal(archived.getAttribute('aria-checked'),'true');
  }finally{await view.dispose();}
});

test('mobile hit targets use transparent pseudo elements, without changing radio/checkbox visual dimensions',async()=>{
  const css=await source('styles/react.css');
  const hitRule=css.match(/@media \(max-width: 700px\) \{[\s\S]*?\[data-slot="radio-group-item"\]::after,[\s\S]*?\.replica-library \.tag-filter > button::after \{([^}]+)\}/)?.[1];
  assert.ok(hitRule,'mobile rules must cover icon buttons, radio, checkbox and home filters');
  for(const rule of ['content: ""','width: max(100%, 44px)','height: max(100%, 44px)','background: transparent','position: absolute'])assert.ok(hitRule.includes(rule),rule);
  const {RadioGroup,RadioGroupItem}=await load('components/ui/radio-group.tsx'),{Checkbox}=await load('components/ui/checkbox.tsx');
  const view=await mountReact(vite,()=>React.createElement('div',null,React.createElement(RadioGroup,{'aria-label':'r9-radio'},React.createElement(RadioGroupItem,{value:'daily','aria-label':'日常'})),React.createElement(Checkbox,{'aria-label':'归档'})));
  try{for(const node of view.host.querySelectorAll('[role=radio],[role=checkbox]')){assert.ok(node.classList.contains('size-4'));assert.ok(node.classList.contains('relative'));}}finally{await view.dispose();}
});

test('mobile quick navigation uses icon size, and its dialog close uses the same shared size',async()=>{
  const {default:QuickJump}=await load('react-app/shared/WorkspaceQuickJump.tsx');
  const previous=window.innerWidth;
  try{
    for(const width of [390,1440]){
      let destination;
      const view=await mountReact(vite,()=>{window.innerWidth=width;return React.createElement(QuickJump,{characters:[],hasCharacter:false,onNavigate:value=>destination=value,onCharacter(){}});});
      try{
        const trigger=view.host.querySelector('[aria-label="快速前往"]');
        if(width===390)icon(trigger);else assert.equal(trigger.dataset.size,'default');
        await view.click(trigger);
        const dialog=document.querySelector('[role=dialog]');assert.ok(dialog);icon(dialog.querySelector('[data-slot="dialog-close"]'));
        await view.click([...dialog.querySelectorAll('.replica-command-results button')].find(node=>node.textContent==='角色库'));
        assert.equal(destination,'角色库');assert.equal(document.querySelector('[role=dialog]'),null);
      }finally{await view.dispose();}
    }
  }finally{window.innerWidth=previous;}
});

test('number stepper icon Buttons retain numeric stepping without height overrides',async()=>{
  const {FieldInput}=await load('react-app/controls.tsx');
  const view=await mountReact(vite,()=>{const [value,setValue]=React.useState('0');return React.createElement(FieldInput,{type:'number','aria-label':'数量',value,onChange:event=>setValue(event.target.value)});});
  try{
    const down=view.host.querySelector('[aria-label="减少数量"]'),up=view.host.querySelector('[aria-label="增加数量"]');
    for(const node of [down,up]){icon(node);assert.equal(node.classList.contains('h-auto'),false);assert.equal(node.classList.contains('touch-target'),false);}
    await view.click(up);assert.equal(view.host.querySelector('input').value,'1');await view.click(down);assert.equal(view.host.querySelector('input').value,'0');
  }finally{await view.dispose();}
});
