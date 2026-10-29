import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {React,act,installDOM,mountReact} from './helpers/react-dom.mjs';
import {reusableWorlds} from '../src/approved-ui/world-sources.mjs';
import {finishCreationWorld} from '../src/v3/creation-world.mjs';
import {configurationPlan} from '../src/approved-ui/configuration-plan.mjs';
import {configurationPoints} from '../src/v3/proto/components/configuration-fields.js';
import {creationDraft} from '../src/v3/creation-draft.mjs';
import {memoryStorage} from './support/runtime.mjs';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const load=path=>vite.ssrLoadModule('/src/'+path),button=(root,text)=>[...root.querySelectorAll('button')].find(node=>node.textContent.trim()===text);
async function referencePanel(ui,name){await act(async()=>ui.host.querySelector('[aria-label="更多运行记录"]').dispatchEvent(new window.KeyboardEvent('keydown',{key:'ArrowDown',bubbles:true})));await ui.flush();await ui.click([...document.querySelectorAll('[role=menuitem]')].find(node=>node.textContent.trim()===name));}

function workspace(){const state=proxy({session:{actorId:'r16-owner'},selectedId:'A',characters:[{id:'A',name:'角色甲'}],managementTab:'运行诊断',operations:[],errors:{},runtimeCatalog:{}});return{state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),currentScope:()=> 'r16-owner-A',services:{configurationClient:{read:async()=>({effective_values:{},endpoints:[],bindings:[],versions:{}}),catalog:async()=>({agents:[]})},diagnosticsClient:{page:async()=>({items:[],next_cursor:''})}},notify(){},navigatePath(){}};}

test('world reads retain successful sources on failure, remove confirmed empty sources, and reject incomplete catalogs',async()=>{
 const previous=[{id:'W',fluctlightId:'A',characterName:'角色甲',revision:3,name:'真实世界'}],characters=[{id:'A',name:'角色甲'}];
 const failed=await reusableWorlds({worldCatalog:async()=>{throw Error('offline');}},characters,previous);assert.deepEqual(failed.items,previous);assert.equal(failed.unavailable,1);
 assert.deepEqual((await reusableWorlds({worldCatalog:async()=>({items:[]})},characters,previous)).items,[]);
 const incomplete=await reusableWorlds({worldCatalog:async()=>({items:[],next_cursor:'unread'})},characters,previous);assert.equal(incomplete.unavailable,1);assert.deepEqual(incomplete.items,previous);
});

test('world wizard requires an explicit real-source choice and keeps it through a failed refresh',async()=>{
 const {default:Wizard}=await load('react-app/pages/CharacterWizardPage.jsx'),w=workspace();let fail=false,directories=0;
 w.state.session.actorId='r16-wizard-owner';w.day=()=> '2026-10-27';w.refreshDirectory=async()=>{directories++;};w.services.configurationClient.read=async()=>({bindings:[]});
 w.client={listStudioResources:async()=>({items:[],next_cursor:''}),worldCatalog:async()=>{if(fail)throw Error('offline');return{items:[{id:'W',name:'日常世界',revision:2}]};}};
 const v=await mountReact(vite,Wizard,{workspace:w});try{assert.match(v.host.textContent,/日常世界/);assert.match(v.host.textContent,/来自 角色甲/);assert.doesNotMatch(v.host.textContent,/使用日常生活世界|创建世界设定/);assert.equal(v.host.querySelector('[data-testid=wizard-world-next]').disabled,true);await v.click(v.host.querySelector('.creation-world-card'));assert.equal(v.host.querySelector('[data-testid=wizard-world-next]').disabled,false);fail=true;await v.click(button(v.host,'重新读取已有世界'));assert.equal(directories,1);assert.match(v.host.textContent,/日常世界/);assert.match(v.host.textContent,/来自 角色甲/);assert.match(v.host.textContent,/暂未读到/);assert.doesNotMatch(v.host.textContent,/使用日常生活世界|创建世界设定/);}finally{await v.dispose();}
});

test('malformed default-world reads never admit a create command',async()=>{
 for(const actual of [{},{available:false,world:null},{world:undefined},{world:{}},{world:{id:""}}]){let writes=0;let draft={requestId:'original',activatedId:'A',useDefaultWorld:true};const recovery={read:()=>structuredClone(draft),save:value=>draft=structuredClone(value)};
 await assert.rejects(finishCreationWorld({client:{world:async()=>actual,updateRuntimeWorld:async()=>{writes++;return{id:'new'};}},recovery,storage:memoryStorage(),owner:'owner'}),/读取结果不完整/);assert.equal(writes,0);assert.equal(draft.activatedId,'A');}
});

test('decision-point controls preserve actual booleans and invalid maps or workflow nodes cannot produce writes',async()=>{
 const {default:SettingsField}=await load('react-app/shared/SettingsField.tsx');const points=Object.fromEntries(configurationPoints.map(point=>[point,true])),field={label:'启用的决策点',type:'boolean-map',value:structuredClone(points),_initial:points,_settingKey:'kev',_key:'points',_definition:{type:'boolean-map'}};
 const v=await mountReact(vite,SettingsField,{props:{field}});try{assert.equal(v.host.querySelector('textarea'),null);await v.click(v.host.querySelector('[role=checkbox]'));assert.equal(typeof Object.values(field.value)[0],'boolean');assert.equal(Object.values(field.value).filter(value=>value===false).length,1);}finally{await v.dispose();}
 const options=fields=>({catalog:{versions:{},effective_values:{'media.comfyui':{baseUrl:'https://fixture.invalid',workflow:{'1':{class_type:'Model',inputs:{}}}},kev:{points},'product.autonomy':{quiet_hours:{}}}},settings:[{fields}],endpoints:[],bindings:[],quiet:{enabled:false,start:'',end:'',days:['周日','周一','周二','周三','周四','周五','周六'],character:'全部角色'},characters:[]});
 assert.throws(()=>configurationPlan(options([{...field,value:{...points,'tools.select':'true'}}])),/有效的决策场景/);
 assert.throws(()=>configurationPlan(options([{...field,value:{'tools.select':true}}])),/有效的决策场景/);
 assert.throws(()=>configurationPlan(options([{type:'json',label:'默认 API 工作流',value:'{"1":{"class_type":"Model","inputs":[]}}',_initial:undefined,_definition:{type:'workflow'},_settingKey:'media.comfyui',_key:'workflow'}])),cause=>{assert.match(cause.message,/class_type 和 inputs/);assert.deepEqual(cause.settingField,{settingKey:'media.comfyui',key:'workflow'});return true;});
});

test('diagnostic panels preserve distinct identities and discard old panel rows; pagination uses the opaque cursor',async()=>{
 const {default:Management}=await load('react-app/pages/ManagementPage.jsx'),w=workspace(),calls=[],details=[];let finishReference,referenceReads=0;
 const record=(ref,kind='agent-runs')=>({ref,kind,occurred_at:'2026-10-27T01:00:00Z',data:{agent_id:'wake_up',status:'failed',text:'保留的原文'}});
 w.services.diagnosticsClient={page:async(kind,options)=>{calls.push({kind,...options});if(kind==='model-runs')return{items:[{...record(options.cursor?'model-2':'model-1',kind),data:{scenario:'wake_up',status:'failed',response:null}}],next_cursor:options.cursor?'':'opaque-original'};if(kind==='agent-runs'){if(options.cursor){referenceReads++;return new Promise(resolve=>finishReference=()=>resolve({items:[record('run-three')],next_cursor:''}));}return{items:[record('run-one'),record('run-two')],next_cursor:'reference-original'};}return{items:[],next_cursor:''};},record:async(kind,ref)=>{details.push({kind,ref});return record(ref,kind);}};
 const v=await mountReact(vite,Management,{workspace:w});const panel=name=>referencePanel(v,name);try{await act(async()=>[...v.host.querySelectorAll('[role=tab]')].find(n=>n.textContent.trim()==='模型调用记录').dispatchEvent(new window.MouseEvent('mousedown',{bubbles:true,button:0})));await v.flush();await v.click(button(v.host,'加载更多调用'));assert.ok(calls.some(call=>call.cursor==='opaque-original'));await panel('逻辑运行');assert.equal([...v.host.querySelectorAll('button')].filter(node=>node.textContent.includes('查看详情')).length,2);await v.click([...v.host.querySelectorAll('button')].find(node=>node.textContent.includes('查看详情')));assert.deepEqual(details,[{kind:'agent-runs',ref:'run-one'}]);await v.click(document.querySelector('[data-testid=reference-drawer-close]'));const more=button(v.host,'加载更多诊断记录');await act(async()=>{more.click();more.click();});assert.equal(referenceReads,1);await act(async()=>finishReference());await v.flush();assert.equal([...v.host.querySelectorAll('button')].filter(node=>node.textContent.includes('查看详情')).length,3);await panel('重试耗尽的后台任务');assert.match(v.host.textContent,/当前没有对应记录/);assert.equal([...v.host.querySelectorAll('button')].filter(node=>node.textContent.includes('查看详情')).length,0);await panel('逻辑运行');assert.equal([...v.host.querySelectorAll('button')].filter(node=>node.textContent.includes('查看详情')).length,2);}finally{await v.dispose();}
});

test('model transcript reads provider text and structured responses and does not invent an empty JSON reply',async()=>{
 const {default:Transcript}=await load('react-app/shared/ModelTranscript.tsx');
 const v=await mountReact(vite,Transcript,{props:{record:{id:'r',status:'success',_record:{data:{prompt:[{role:'user',content:[{type:'text',text:'用户的完整输入'}]}],response:{text:'**真实回复**\n下一行',structured:{result:'已核对'}}}}}}});try{assert.match(v.host.textContent,/真实回复/);assert.equal(v.host.querySelector('pre'),null);await v.update({record:{id:'empty',status:'error',_record:{data:{response:null}}}});assert.match(v.host.textContent,/尚未记录响应正文/);assert.doesNotMatch(v.host.textContent,/\{\}/);}finally{await v.dispose();}
});

test('r18 auxiliary diagnostic pagination retains correlation and cannot adopt a late page from another task',async()=>{
 const {default:Management}=await load('react-app/pages/ManagementPage.jsx'),previous=window.location.hash,w=workspace(),calls=[];let finishOld;
 const row=ref=>({ref,kind:'agent-runs',occurred_at:'2026-10-27T01:00:00Z',data:{title:ref,agent_id:'wake_up',status:'failed'}});
 w.services.diagnosticsClient={page:async(kind,options)=>{calls.push({kind,...options});if(kind!=='agent-runs')return{items:[],next_cursor:''};if(options.cursor)return new Promise(resolve=>finishOld=()=>resolve({items:[row('late-first-task')],next_cursor:'stale-cursor'}));return{items:[row(options.correlationId)],next_cursor:options.correlationId==='first-task'?'original-first-cursor':''};}};
 const v=await mountReact(vite,Management,{workspace:w}),panel=()=>referencePanel(v,'逻辑运行');
 try{await act(async()=>{window.location.hash='#/diag?view=calls&correlationId=first-task';window.dispatchEvent(new window.HashChangeEvent('hashchange'));});await v.flush();await panel();await act(async()=>button(v.host,'加载更多诊断记录').click());assert.equal(calls.filter(c=>c.kind==='agent-runs').at(-1).correlationId,'first-task');assert.equal(calls.filter(c=>c.kind==='agent-runs').at(-1).cursor,'original-first-cursor');await act(async()=>{window.location.hash='#/diag?view=calls&correlationId=second-task';window.dispatchEvent(new window.HashChangeEvent('hashchange'));});await v.flush();await panel();assert.equal(calls.filter(c=>c.kind==='agent-runs').at(-1).correlationId,'second-task');assert.equal(calls.filter(c=>c.kind==='agent-runs').at(-1).cursor,undefined);await act(async()=>finishOld());await v.flush();assert.equal(button(v.host,'加载更多诊断记录'),undefined);assert.equal([...v.host.querySelectorAll('button')].filter(n=>n.textContent.includes('查看详情')).length,1);}finally{await v.dispose();window.location.hash=previous;}
});

test('profile presentation preserves custom English, false and zero while removing unknown-only placeholder nesting',async()=>{
 const {default:Tree}=await load('react-app/shared/SemanticTree.jsx'),original={hair_color:{status:'unknown'},hair_style:{status:'unknown'},customFact:'Architect',customFlag:false,customCount:0,wardrobe_items:[{description:'长裤',category:'bottom',slot:'bottom',ownership:'owned',availability:'available'}]};const before=structuredClone(original);
 const v=await mountReact(vite,Tree,{props:{value:original}});try{assert.match(v.host.textContent,/Architect/);assert.match(v.host.textContent,/否/);assert.match(v.host.textContent,/0/);assert.doesNotMatch(v.host.textContent,/补充设定|尚未确定|下装 · 下装/);assert.deepEqual(original,before);}finally{await v.dispose();}
});

test('media thumbnail opens the Demo detail in place and keeps source and management actions reachable',async()=>{
 const {default:Media}=await load('react-app/pages/MediaPage.tsx'),w=workspace(),moves=[];
 const asset={id:'image',kind:'image',origin:'owner_upload',title:'素材样本',byte_size:10240,readable:true,read_state:'readable',status:'ready',asset_revision:1,metadata_revision:0,hidden:false};
 w.services.mediaClient={list:async()=>({items:[asset],next_cursor:''}),asset:async()=>asset,url:id=>'/media/'+id};w.navigatePath=path=>moves.push(path);
 const v=await mountReact(vite,Media,{workspace:w,props:{route:{query:new URLSearchParams()}}});try{const preview=v.host.querySelector('[aria-label="查看画面 素材样本"]');await v.click(preview);const detail=document.querySelector('[role=dialog]');assert.ok(detail.querySelector('img'));assert.match(detail.textContent,/编辑库信息|引用授权与删除管理/);assert.deepEqual(moves,[]);await act(async()=>document.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Escape',bubbles:true})));await v.flush();assert.equal(document.querySelector('[role=dialog]'),null);assert.equal(document.activeElement,preview);assert.deepEqual(moves,[]);}finally{await v.dispose();}
});

test('new creation drafts default to Beijing while a stored explicit timezone is retained',()=>{
 const storage=memoryStorage(),drafts=creationDraft({owner:'r16-timezone',storage});assert.equal(drafts.read().timezone,'Asia/Shanghai');const saved=drafts.read();saved.timezone='America/Los_Angeles';drafts.save(saved);assert.equal(creationDraft({owner:'r16-timezone',storage}).read().timezone,'America/Los_Angeles');
});

test('unsaved configuration has explicit dirty feedback and discard restores confirmed fields without a write',async()=>{
 const {default:Management}=await load('react-app/pages/ManagementPage.jsx'),w=workspace();w.state.session.actorId='r16-discard-owner';w.state.managementTab='配置中心';let reads=0,writes=0;
 w.services.configurationClient={read:async()=>{reads++;return{effective_values:{},versions:{'binding:generic_llm':2},endpoints:[{id:'saved',kind:'openai-compatible',base_url:'https://fixture.invalid/v1',secret_configured:false}],bindings:[{role:'generic_llm',endpoint_id:'saved',model_id:'original-model',timeout_seconds:120}]};},models:async()=>({models:['original-model','new-model']}),catalog:async()=>({agents:[]}),binding:async()=>{writes++;}};
 const v=await mountReact(vite,Management,{workspace:w});try{const choose=v.host.querySelector('input[aria-label="模型标识"]');await v.input(choose,'new-model');assert.match(v.host.querySelector('[data-testid=config-save-status]').textContent,/有未保存修改/);await v.click(button(v.host,'放弃未保存修改'));assert.equal(reads,2);assert.equal(writes,0);assert.equal(v.host.querySelector('input[aria-label="模型标识"]').value,'original-model');assert.doesNotMatch(v.host.querySelector('[data-testid=config-save-status]').textContent,/有未保存修改/);}finally{await v.dispose();}
});


test('structured readers translate declared status fields without rewriting quoted user or assistant words',async()=>{
 const {default:Reader}=await load('react-app/shared/RecordView.tsx');const value={status:'failed',text:'failed',message:'assistant',count:0,enabled:false};const v=await mountReact(vite,Reader,{props:{value}});try{assert.match(v.host.textContent,/失败/);assert.match(v.host.textContent,/failed/);assert.match(v.host.textContent,/assistant/);assert.match(v.host.textContent,/0/);assert.match(v.host.textContent,/否/);}finally{await v.dispose();}
});

test('failed reads cannot reveal a default world; confirmed empty catalogs require an explicit source choice',async()=>{
 const {default:Wizard}=await load('react-app/pages/CharacterWizardPage.jsx'),w=workspace();w.state.session.actorId='r16-empty-world-owner';w.refreshDirectory=async()=>{};w.day=()=> '2026-10-27';let fail=true,creates=0;
 w.services.configurationClient.read=async()=>({bindings:[]});w.client={worldCatalog:async()=>{if(fail)throw Error('offline');return{items:[]};},listStudioResources:async()=>({items:[],next_cursor:''}),updateRuntimeWorld:async()=>{creates++;}};
 const v=await mountReact(vite,Wizard,{workspace:w});try{assert.doesNotMatch(v.host.textContent,/使用日常生活世界|创建世界设定/);assert.equal(v.host.querySelector('[data-testid=wizard-world-next]').disabled,true);fail=false;await v.click(button(v.host,'重新读取已有世界'));assert.match(v.host.textContent,/已确认没有已有生活世界/);assert.match(v.host.textContent,/创建新的世界底稿/);assert.equal(v.host.querySelector('[data-testid=wizard-world-next]').disabled,true);await act(async()=>[...v.host.querySelectorAll('[role=tab]')].find(n=>n.textContent==='日常生活世界').dispatchEvent(new window.MouseEvent('mousedown',{bubbles:true,button:0,ctrlKey:false})));await v.flush();await v.click(button(v.host,'使用日常生活世界'));assert.equal(v.host.querySelector('[data-testid=wizard-world-next]').disabled,false);assert.equal(creates,0);}finally{await v.dispose();}
});

test('workspace read failures are scoped and distinguish first failure from preserved successful data across role switches',async()=>{
 const {createWorkspace}=await load('state/workspace.mjs'),store=await load('v3/proto/store.js'),services=await load('v3/proto/services.js'),{workspaceReadFeedback}=await load('approved-ui/read-feedback.mjs');const w=createWorkspace(),originalDetail=w.client.detail;let fail=true;
 try{store.replaceSnapshot({...store.get(),session:{actorId:'r16-read-owner'},characters:{A:{id:'A',name:'角色甲'},B:{id:'B',name:'角色乙'}},ui:{charId:'A'}});w.client.detail=async()=>({identity:{name:'角色',timezone:'Asia/Shanghai'}});services.configureDomainService('life',{schedules:async()=>({days:[]}),goals:async()=>({items:[],next_cursor:''}),promises:async()=>({items:[],next_cursor:''})});services.configureDomainService('diary',{list:async()=>({items:[],next_cursor:''})});services.configureDomainService('memory',{list:async()=>{if(fail)throw Error('记忆列表离线');return{items:[],next_cursor:''};}});
  await w.loadLife();let feedback=workspaceReadFeedback(w.state,'记忆',w.currentScope());assert.equal(feedback.initialFailure,true);assert.equal(feedback.errors[0].message,'记忆列表离线');fail=false;await w.loadLife();assert.equal(workspaceReadFeedback(w.state,'记忆',w.currentScope()).errors.length,0);fail=true;await w.loadLife();assert.equal(workspaceReadFeedback(w.state,'记忆',w.currentScope()).initialFailure,false);w.select('B');assert.equal(workspaceReadFeedback(w.state,'记忆',w.currentScope()).errors.length,0);await w.loadLife();assert.equal(workspaceReadFeedback(w.state,'记忆',w.currentScope()).initialFailure,true);w.select('A');await w.loadLife();assert.equal(workspaceReadFeedback(w.state,'记忆',w.currentScope()).initialFailure,false);store.replaceSnapshot({...store.get(),session:{actorId:'r16-another-owner'},characters:{},ui:{charId:''}});assert.equal(Object.keys(w.state.readSuccesses).length,0);
 }finally{w.client.detail=originalDetail;w.dispose();}
});
