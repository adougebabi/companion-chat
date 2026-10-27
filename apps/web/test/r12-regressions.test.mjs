import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {React,act,installDOM,mountReact} from './helpers/react-dom.mjs';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const load=path=>vite.ssrLoadModule('/src/'+path);
const {default:Navigation}=await load('react-app/shared/WorkspaceNavigation.tsx');
const {SidebarProvider,useSidebar}=await load('components/ui/sidebar.tsx');
const {FieldInput}=await load('react-app/controls.tsx');
function NavigationHarness({page='记忆',character={id:'A',name:'测试角色'},onNavigate}){
 window.matchMedia=()=>({matches:false,addEventListener(){},removeEventListener(){}});
 const [open,setOpen]=React.useState(true);
 return React.createElement(SidebarProvider,null,React.createElement(Navigation,{page,character,lifeOpen:open,onLifeOpenChange:setOpen,onNavigate}));
}
test('native sidebar lists preserve active navigation, life collapse, real buttons and empty-directory scope',async()=>{
 const moves=[];const v=await mountReact(vite,NavigationHarness,{props:{onNavigate:page=>moves.push(page)}});
 try{assert.ok(v.host.querySelector('[data-slot=sidebar-menu]'));assert.ok(v.host.querySelector('[data-slot=sidebar-group]'));const selected=v.host.querySelector('[data-testid="nav-记忆"]');assert.equal(selected.tagName,'BUTTON');assert.equal(selected.getAttribute('aria-current'),'page');assert.equal(selected.dataset.active,'true');assert.equal(v.host.querySelector('[data-testid="nav-聊天"]').dataset.active,'false');
  const life=v.host.querySelector('[aria-label="生活"]');await v.click(life);assert.equal(life.getAttribute('aria-expanded'),'false');assert.equal(v.host.querySelector('[data-testid="nav-日记"]'),null);await v.click(life);await v.click(v.host.querySelector('[data-testid="nav-日记"]'));assert.deepEqual(moves,['日记']);
  await v.update({onNavigate:page=>moves.push(page),character:{name:'我的空间'},page:'角色库'});for(const name of ['聊天','记忆','人物关系'])assert.equal(v.host.querySelector('[data-testid="nav-'+name+'"]').disabled,true);for(const name of ['世界','动态','媒体','通知与待处理','角色库','创作中心','管理中心'])assert.equal(v.host.querySelector('[data-testid="nav-'+name+'"]').disabled,false);await v.click(v.host.querySelector('[data-testid="nav-世界"]'));assert.equal(moves.at(-1),'世界');
 }finally{await v.dispose();}
});
test('number controls update real values with decimal steps, boundaries, step any, and direct typing',async()=>{
 let current;
 function Number({step='0.5',min='1',max='2'}){const [value,setValue]=React.useState('1.5');current=value;return React.createElement(FieldInput,{type:'number','aria-label':'数值',value,step,min,max,onChange:event=>setValue(event.target.value)});}
 const v=await mountReact(vite,Number);
 try{await v.click(v.host.querySelector('[aria-label="增加数值"]'));assert.equal(current,'2');await v.click(v.host.querySelector('[aria-label="增加数值"]'));assert.equal(current,'2');await v.click(v.host.querySelector('[aria-label="减少数值"]'));assert.equal(current,'1.5');await v.input(v.host.querySelector('input'),'1');assert.equal(current,'1');await v.click(v.host.querySelector('[aria-label="减少数值"]'));assert.equal(current,'1');await v.update({step:'any'});await v.click(v.host.querySelector('[aria-label="增加数值"]'));assert.equal(current,'2');}finally{await v.dispose();}
});
test('time defaults survive undefined carrier props and enforce the actual visible format',async()=>{
 function Time(){const [value,setValue]=React.useState('09:15');return React.createElement(FieldInput,{type:'time',value,pattern:undefined,placeholder:undefined,onChange:event=>setValue(event.target.value)});}
 const v=await mountReact(vite,Time);
 try{const input=v.host.querySelector('input');assert.equal(input.type,'text');assert.equal(input.placeholder,'HH:mm');await v.input(input,'25:01');assert.equal(input.checkValidity(),false);await v.input(input,'00:00:30');assert.equal(input.checkValidity(),true);}finally{await v.dispose();}
});
test('diary generation records distinguish unknown progress, failure and absent body without fabricated percentage',async()=>{
 const {diaryGenerationRecord}=await load('v3-domain/diary.js');for(const status of ['pending','running','failed','inactive']){const html=diaryGenerationRecord('A',{id:'D',date:'2026-10-26',status});assert.doesNotMatch(html,/<progress|aria-valuenow|data-value=/);assert.match(html,/2026-10-26/);assert.match(html,/#\/c\/A\/life\/diary\/D/);if(['pending','running'].includes(status)){assert.match(html,/data-domain-ui="skeleton"/);assert.match(html,/查看生成进度/);}else if(status==='failed'){assert.match(html,/查看原因与恢复/);assert.match(html,/role="alert"/);}else assert.match(html,/还没有形成日记正文/);}
});
const {default:Confirm}=await load('react-app/shared/ConfirmationFrame.tsx');
const {AlertDialogAction,AlertDialogCancel}=await load('components/ui/alert-dialog.tsx');
test('semantic async confirmation retains failures and invokes only one explicit action; cancel never writes',async()=>{
 let writes=0,resolve;
 function View(){const [open,setOpen]=React.useState(true),[busy,setBusy]=React.useState(false),[error,setError]=React.useState('');return open?React.createElement(Confirm,{title:'删除测试对象？',description:error||'原对象及提交会保留',onClose:()=>setOpen(false)},React.createElement(AlertDialogCancel,null,'取消'),React.createElement(AlertDialogAction,{variant:'destructive',disabled:busy,onClick:event=>{event.preventDefault();writes++;setBusy(true);new Promise(done=>resolve=done).then(ok=>{setBusy(false);if(ok)setOpen(false);else setError('保存失败，当前操作保留');});}},'确认删除')):React.createElement('p',null,'已关闭');}
 const v=await mountReact(vite,View);try{const action=document.querySelector('[data-slot=alert-dialog-action]');assert.ok(document.querySelector('[data-slot=alert-dialog-footer]'));await v.click(action);assert.equal(writes,1);assert.equal(action.disabled,true);assert.ok(document.querySelector('[role=alertdialog]'));await act(async()=>resolve(false));await v.flush();assert.match(document.querySelector('[role=alertdialog]').textContent,/保存失败/);await v.click(document.querySelector('[data-slot=alert-dialog-footer] [data-slot=alert-dialog-cancel]'));assert.equal(writes,1);assert.equal(document.querySelector('[role=alertdialog]'),null);}finally{await v.dispose();}
});
test('date fields use the real calendar, enforce date bounds and submit the selected civil date once',async()=>{
 const {default:DateField}=await load('react-app/shared/DateField.tsx');let selected;
 function View(){const [date,setDate]=React.useState('2026-10-20');selected=date;return React.createElement('form',null,React.createElement(DateField,{name:'localDate',value:date,min:'2026-10-20',max:'2026-10-25',onChange:event=>setDate(event.target.value)}));}
 const v=await mountReact(vite,View);try{assert.equal(v.host.querySelector('input[type=date]'),null);await v.click(v.host.querySelector('[role=combobox]'));const day=text=>[...document.querySelectorAll('button[data-day]')].find(node=>node.textContent===text);assert.equal(day('19').disabled,true);assert.equal(day('26').disabled,true);await v.click(day('21'));assert.equal(selected,'2026-10-21');assert.deepEqual([...new window.FormData(v.host.querySelector('form'))],[['localDate','2026-10-21']]);}finally{await v.dispose();}
});
test('wardrobe preparation retains readable progress without serializing old disclosure props onto its Card',async()=>{
 const {default:Preparation}=await load('react-app/shared/WardrobePreparation.jsx');const state={session:{actorId:'r12-prep-owner'},selectedId:'A',operations:[]};const workspace={state,subscribe:()=>()=>{},getSnapshot:()=>state,currentScope:()=> 'r12-preparation',services:{inventoryClient:{preparation:async()=>({fluctlight_id:'A',status:'failed',can_recover:true,task_id:'T',result:{failure_reason:'原准备失败原因'}})}}};
 const v=await mountReact(vite,Preparation,{workspace});try{const card=v.host.querySelector('.approved-preparation');assert.ok(card);assert.equal(card.getAttribute('title'),null);assert.equal(card.hasAttribute('open'),false);assert.match(card.textContent,/初始衣柜/);assert.match(card.textContent,/原准备失败原因/);assert.doesNotMatch(v.host.outerHTML,/\[object Object\]/);}finally{await v.dispose();}
});
test('settings chapter navigation opens the actual Disclosure and collapse preserves edited configuration fields',async()=>{
 const {default:Management}=await load('react-app/pages/ManagementPage.jsx');const state={session:{actorId:'r12-settings-owner'},managementTab:'配置中心',characters:[],operations:[],managementSection:'',runtimeCatalog:{}};let writes=0;const workspace={state,subscribe:()=>()=>{},getSnapshot:()=>state,services:{configurationClient:{read:async()=>({endpoints:[],bindings:[],versions:{},effective_values:{}}),models:async()=>({models:[]}),catalog:async()=>({agents:[]}),settings:async()=>{writes++;}},diagnosticsClient:{page:async()=>({items:[]})}},notify(){},navigatePath(){}};
 const v=await mountReact(vite,Management,{workspace});try{const section=v.host.querySelector('[id="config-资源与预算"]');assert.ok(section);assert.equal(section.getAttribute('title'),null);const trigger=section.querySelector('[data-slot=collapsible-trigger]');assert.ok(trigger);assert.equal(trigger.getAttribute('aria-expanded'),'false');await v.click(v.host.querySelector('[data-testid="config-anchor-资源与预算"]'));assert.equal(trigger.getAttribute('aria-expanded'),'true');const input=section.querySelector('[aria-label="生成模型并发"]');assert.ok(input);await v.input(input,'3');await v.click(trigger);assert.equal(trigger.getAttribute('aria-expanded'),'false');await v.click(trigger);assert.equal(section.querySelector('[aria-label="生成模型并发"]').value,'3');assert.equal(writes,0);assert.doesNotMatch(v.host.outerHTML,/\[object Object\]/);}finally{await v.dispose();}
});
