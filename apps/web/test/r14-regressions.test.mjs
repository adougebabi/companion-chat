import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {React,act,installDOM,mountReact} from './helpers/react-dom.mjs';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const load=path=>vite.ssrLoadModule('/src/'+path);

test('DetailDrawer exposes one close action and restores its connected trigger after Escape',async()=>{
 const {DetailDrawer}=await load('react-app/components.tsx');let closed=0;
 function View(){const [open,setOpen]=React.useState(false);return React.createElement(React.Fragment,null,React.createElement('button',{onClick:()=>setOpen(true)},'查看详情'),open&&React.createElement(DetailDrawer,{title:'运行详情',testId:'detail',onClose:()=>{closed++;setOpen(false);}},React.createElement('p',null,'保留的记录')));}
 const v=await mountReact(vite,View);try{const trigger=v.host.querySelector('button');trigger.focus();await v.click(trigger);let dialog=document.querySelector('[role=dialog]');assert.equal(dialog.querySelectorAll('button').length,1);assert.equal(dialog.querySelector('button').getAttribute('aria-label'),'关闭');await v.click(dialog.querySelector('button'));assert.equal(closed,1);assert.equal(document.activeElement,trigger);await v.click(trigger);await act(async()=>document.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Escape',bubbles:true})));await v.flush();assert.equal(closed,2);assert.equal(document.querySelector('[role=dialog]'),null);assert.equal(document.activeElement,trigger);}finally{await v.dispose();}
});

test('searchable choices retain exact IDs, clear values and custom model IDs; Escape never selects',async()=>{
 const {default:Combobox}=await load('react-app/shared/Combobox.tsx');let selected='';
 function View(){const [value,setValue]=React.useState('');return React.createElement(Combobox,{label:'模型',value,allowCustom:true,options:[{value:'',label:'尚未配置'},{value:'model-A',label:'同名模型'},{value:'model-B',label:'同名模型'}],onChange:next=>{selected=next;setValue(next);}});}
 const v=await mountReact(vite,View);try{const trigger=v.host.querySelector('[role=combobox]');await v.click(trigger);const input=()=>document.querySelector('[cmdk-input]');await v.input(input(),'model-B');await v.click([...document.querySelectorAll('[cmdk-item]')].find(node=>node.getAttribute('data-value')==='model-B 同名模型'));assert.equal(selected,'model-B');await v.click(trigger);await v.input(input(),'custom/model:v1');await v.click([...document.querySelectorAll('[cmdk-item]')].find(node=>node.textContent==='采用「custom/model:v1」'));assert.equal(selected,'custom/model:v1');await v.click(trigger);await v.input(input(),'未提交');await act(async()=>document.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Escape',bubbles:true})));await v.flush();assert.equal(selected,'custom/model:v1');await v.click(trigger);assert.equal(input().value,'');await v.click([...document.querySelectorAll('[cmdk-item]')].find(node=>node.textContent==='尚未配置'));assert.equal(selected,'');}finally{await v.dispose();}
});

test('collapsed sidebar keeps direct life pages reachable and preserves their navigation identity',async()=>{
 const {default:Navigation}=await load('react-app/shared/WorkspaceNavigation.tsx'),{SidebarProvider,Sidebar,SidebarRail}=await load('components/ui/sidebar.tsx');const moves=[];
 function View(){return React.createElement(SidebarProvider,{defaultOpen:false},React.createElement(Sidebar,{collapsible:'icon'},React.createElement(Navigation,{page:'日记',character:{id:'A',name:'很长的角色名'},lifeOpen:true,onLifeOpenChange(){},onNavigate:page=>moves.push(page)}),React.createElement(SidebarRail)));}
 const v=await mountReact(vite,View);try{assert.ok(v.host.querySelector('[data-collapsible=icon]'));const diary=v.host.querySelector('[data-testid="nav-日记"]');assert.ok(diary);assert.equal(diary.getAttribute('aria-current'),'page');assert.equal(diary.getAttribute('aria-label'),'日记');await v.click(diary);assert.deepEqual(moves,['日记']);assert.ok(v.host.querySelector('[data-testid="nav-此刻"]'));}finally{await v.dispose();}
});

test('preparation Trace keeps independent failures, unknown states and recovery actions',async()=>{
 const {default:Preparation}=await load('react-app/shared/PreparationSteps.jsx');const recovered=[],stages=[{key:'analysis',title:'资料',state:'ready',note:'已读取'},{key:'visual',title:'形象',state:'failed',note:'配置失败',retryable:true,configSection:'图像服务',href:'#/diag/task/T'},{key:'schedule',title:'日程',state:'unknown',note:'暂未读到'}];
 const v=await mountReact(vite,Preparation,{props:{stages,onRecover:stage=>recovered.push(stage.key)}});try{assert.ok(v.host.querySelector('[data-slot=trace]'));assert.ok(Math.abs(Number(v.host.querySelector('[role=progressbar]').getAttribute('aria-valuenow'))-100/3)<1e-10);assert.match(v.host.querySelector('[data-testid=prep-step-schedule]').textContent,/状态待确认/);assert.match(v.host.querySelector('[role=alert]').textContent,/配置失败/);await v.click([...v.host.querySelectorAll('button')].find(node=>node.textContent.trim()==='重试此环节'));assert.deepEqual(recovered,['visual']);assert.match(v.host.querySelector('a').getAttribute('href'),/section=/);await v.update({stages,busy:true,onRecover:stage=>recovered.push(stage.key)});assert.equal([...v.host.querySelectorAll('button')].find(node=>node.textContent.trim()==='处理中…').disabled,true);}finally{await v.dispose();}
});


test('left navigation shortcut never toggles the independent right preview provider',async()=>{
 const {SidebarProvider,useSidebar}=await load('components/ui/sidebar.tsx');
 function State({side}){const {open}=useSidebar();return React.createElement('span',{'data-side-state':side},String(open));}
 function View(){const [preview,setPreview]=React.useState(true);return React.createElement(SidebarProvider,null,React.createElement(State,{side:'left'}),React.createElement(SidebarProvider,{open:preview,onOpenChange:setPreview,keyboardShortcut:false},React.createElement(State,{side:'right'})));}
 const v=await mountReact(vite,View);try{await act(async()=>window.dispatchEvent(new window.KeyboardEvent('keydown',{key:'b',ctrlKey:true,bubbles:true})));await v.flush();assert.equal(v.host.querySelector('[data-side-state=left]').textContent,'false');assert.equal(v.host.querySelector('[data-side-state=right]').textContent,'true');}finally{await v.dispose();}
});
