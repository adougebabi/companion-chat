import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {esc,badge} from '../src/v3/proto/components/controls.js';
import {domainName,stateLabel,errorMessage} from '../src/v3/domain-presentation.mjs';
import {requestDecisionText,RequestDecisionSource,RequestFailureLink} from '../src/v3/proto/components/request-failure.js';
import {creationDraft,checkpointCreation,checkpointCreationAnalysis,selectCreationWorld} from '../src/v3/creation-draft.mjs';
import {creationAnalysisRequest,parseCreationAnalysis} from '../src/v3/creation-json.mjs';
import {persistentCommand} from '../src/v3/providers/persistent-command.mjs';
const tick=()=>new Promise(resolve=>setImmediate(resolve));
import {h,defineComponent} from 'vue';
import {setupSFC,memoryStorage} from './support/sfc-harness.mjs';
import {creationFixture,validAnalysis} from './support/creation-fixture.mjs';
import {migrateCreationRecovery,creationHistoryKey} from '../src/approved-ui/creation-recovery.mjs';
import {ActionButton,Surface} from '../src/approved-ui/shared/primitives.mjs';
const storage=()=>{const values=new Map();return {getItem:k=>values.get(k)??null,setItem:(k,v)=>values.set(k,v),removeItem:k=>values.delete(k)};};
function load(file,bindings,name){const text=readFileSync(new URL('../src/v3/proto/'+file,import.meta.url),'utf8').replace(/^import .+;\s*$/gm,'').replace(/^export \{.+;\s*$/gm,'').replaceAll('export ','');return Function(...Object.keys(bindings),text+'\nreturn '+name)(...Object.values(bindings));}
function element(){const nodes=new Map();return {isConnected:true,innerHTML:'',querySelector(s){if(!nodes.has(s))nodes.set(s,{isConnected:true,innerHTML:'',textContent:'',hidden:false,disabled:false,insertAdjacentHTML(_where,text){this.innerHTML+=text;},addEventListener(){}});return nodes.get(s);},querySelectorAll:()=>[]};}


test('active Vue authentication preserves exact credentials and rejects mismatched confirmation before any write',async()=>{
 let calls=0,focused=false,navigated='';
 const w={client:{setupStatus:async()=>({setupAvailable:true}),setup:async(token,password)=>{calls++;assert.equal(token,'controlled-setup-token');assert.equal(password,' exact password ');}},bootstrap:async()=>{},navigatePath:path=>navigated=path};
 const view=setupSFC(new URL('../src/approved-ui/pages/AuthPage.vue',import.meta.url),{useWorkspace:()=>w,errorMessage},['password','confirmation','confirmationInput','token','setup','error','busy','submit']);
 await view.mount();const c=view.state;c.password.value=' exact password ';c.confirmation.value='mismatch';c.token.value='controlled-setup-token';c.confirmationInput.value={focus(){focused=true;}};
 await c.submit();assert.equal(calls,0);assert.equal(focused,true);assert.match(c.error.value,/两次输入的密码不一致/);assert.match(await view.html(),/两次输入的密码不一致/);
 c.confirmation.value=c.password.value;await c.submit();assert.equal(calls,1);assert.equal(navigated,'/library');assert.equal(c.password.value,'');assert.equal(c.token.value,'');view.unmount();
});
test('active creation ignores a stale analysis without activating or overwriting the replacement owner draft',async()=>{
 let finish;const response=new Promise(resolve=>finish=resolve),f=creationFixture(()=>response);f.c.draft.bio='原始描述';f.c.next();const waiting=f.c.analyze();await tick();
 const key='fluctlight-creation:owner:draft',newer={...JSON.parse(f.saved.getItem(key)),requestId:'replacement',name:'新草稿',bio:'保留新输入',analysis:null};f.saved.setItem(key,JSON.stringify(newer));
 finish(validAnalysis());await waiting;assert.equal(f.calls.length,0);assert.deepEqual(JSON.parse(f.saved.getItem(key)),newer);f.c.dispose();
});
test('active creation retains editable unnamed preview, blocks activation, then creates once and starts a fresh draft',async()=>{
 const f=creationFixture();f.c.draft.bio='人物原始描述';f.c.next();await f.c.analyze();assert.equal(f.c.step.value,'preview');assert.equal(f.c.draft.bio,'人物原始描述');assert.equal(f.calls.length,0);
 await f.c.activate();assert.equal(f.calls.length,0);assert.match(f.c.error.value,/填写角色名字/);
 const name=f.c.previewFields.value.find(field=>JSON.stringify(field.path)==='["identity","name"]');assert.ok(name);name.value='周屿';f.c.persistPreview();await f.c.activate();
 assert.equal(f.calls.length,1);assert.equal(f.c.draft.activatedId,'created');assert.equal(f.calls[0].name,'周屿');f.c.newDraft();assert.equal(f.c.step.value,'world');assert.equal(f.c.draft.mode,'description');assert.equal(f.c.draft.activatedId,null);assert.equal(f.c.draft.bio,'');f.c.dispose();
});
test('inline progress follows original request, localizes tasks, keeps state on read failure and ignores changed Owner',async()=>{
 const previous=globalThis.document;const handlers=new Map();globalThis.document={hidden:false,addEventListener:(kind,fn)=>handlers.set(kind,fn),removeEventListener:kind=>handlers.delete(kind)};
 let actor='owner',request={id:'saved-request',status:'pending',tasks:[{kind:'cognition.processing',status:'started',task_id:'task'}]},fail=false,finished=0;const reads=[];
 const host=element();const requestReceipt=load('components/async-page.js',{get:()=>({actorId:actor}),esc,drawer:()=>{throw Error('must stay inline');},domainName,stateLabel,errorMessage,requestDecisionText,RequestDecisionSource,RequestFailureLink,InventoryEffects:()=>'',setTimeout:()=>1,clearTimeout(){},lifeClient:{request:async(fid,id)=>{reads.push([fid,id]);if(fail)throw Error('fetch failed');return request;}}},'requestReceipt');
 try{
  const panel=requestReceipt('role',{request},()=>finished++,{container:host});
  assert.match(host.querySelector('[data-request]').innerHTML,/回应处理.*已开始/);
  fail=true;host.querySelector('[data-query]').onclick();await tick();assert.match(host.querySelector('[data-request]').innerHTML,/更新失败，上次状态已保留/);assert.match(host.querySelector('[data-request]').innerHTML,/等她回应/);
  fail=false;request={...request,status:'completed'};host.querySelector('[data-query]').onclick();await tick();assert.equal(finished,1);assert.deepEqual(reads,[['role','saved-request'],['role','saved-request']]);
  const html=host.querySelector('[data-request]').innerHTML;actor='other';request={...request,status:'failed'};host.querySelector('[data-query]').onclick();await tick();assert.equal(host.querySelector('[data-request]').innerHTML,html);panel.dispose();assert.equal(handlers.size,0);
 }finally{globalThis.document=previous;}
});


test('active Vue library renders the durable continuation and a role whose group has not loaded',async()=>{
 const saved=memoryStorage(),session=memoryStorage(),recovery=creationDraft({owner:'owner',storage:saved,timezone:'Asia/Shanghai'});recovery.save({...recovery.read(),name:'小林',wizardStep:2});
 const role={id:'role',groupId:'unloaded-group',name:'晚晴',status:'正常',tags:[],job:'',bio:''};
 const w={state:{session:{actorId:'owner'},characters:[role]},services:{configurationClient:{read:async()=>({bindings:[]})}},select(){},navigate(){},navigatePath(){}};
 const view=setupSFC(new URL('../src/approved-ui/pages/LibraryPage.vue',import.meta.url),{useWorkspace:()=>w,localStorage:saved,sessionStorage:session,migrateCreationRecovery,creationHistoryKey},['w','search','filters','configured','saved','error','characters','groups','rows','progressHref','details','choose','subtitle']);
 await view.mount();await tick();assert.deepEqual(view.state.rows.value,[role]);assert.equal(view.state.saved.value.name,'小林');
 const html=await view.html({ActionButton,Surface,FilterChips:defineComponent(()=>()=>h('div')),CharacterAvatar:defineComponent(()=>()=>h('span')),Gu:defineComponent(()=>()=>h('svg'))});assert.match(html,/继续上次的相识|继续创建/);assert.match(html,/继续创建与查看进度/);assert.match(html,/晚晴/);view.unmount();
});

test('outfit action reads fresh wardrobe and outfit versions at click time',async()=>{
 const box=element(),wear={isConnected:true,dataset:{wear:'plan'},disabled:false};box.querySelectorAll=s=>s==='[data-wear]'?[wear]:[];
 let currentReads=0,submitted;
 const mount=load('components/outfit-library.js',{inventoryClient:{tags:async()=>({tags:[]}),current:async()=>({revision:++currentReads===1?1:9}),outfits:async()=>({profile_id:'profile',items:[{id:'plan',revision:2}],next_cursor:''}),outfit:async()=>({id:'plan',revision:8,active_profile_id:'profile'})},esc,drawer(){},toast(){},OutfitPreview:()=>'',requestInventoryAction:(...args)=>submitted=args},'mountOutfitLibrary');
 const panel=mount(box,{fid:'role'});await tick();await wear.onclick();assert.equal(currentReads,2);assert.equal(submitted[3],9);assert.deepEqual(submitted[2],{outfitId:'plan',expectedOutfitRevision:8,profileId:'profile'});panel.dispose();
});

test('slow restoration of an old suggestion cannot replace a freshly submitted progress panel',async()=>{
 const saved=storage();saved.setItem('fluctlight-studio:owner:moment:suggest:global:last',JSON.stringify({id:'old',fid:'role'}));
 let release;const delayed=new Promise(resolve=>release=resolve),shown=[];
 const host=element(),form=host.querySelector('form');form.elements={text:host.querySelector('[name=text]'),fid:{value:'role'}};form.reportValidity=()=>true;
 const mount=load('components/moment-composer.js',{get:()=>({actorId:'owner'}),list:()=>[],studioDraftStorage:()=>saved,persistentCommand:()=>({pending:()=>null,submit:async()=>({request:{id:'new',status:'pending'}})}),esc,Field:()=>'',errorMessage,stateLabel,toast(){},requestReceipt:(_fid,envelope)=>{shown.push(envelope.request.id);return {dispose(){}};},lifeClient:{request:()=>delayed},momentsClient:{},mediaClient:{},mountCursorRecords(){},drawer(){},confirm(){}},'mountMomentComposer');
 const panel=mount(host,{mode:'suggest'});form.elements.fid.onchange();form.elements.text.value='新建议';form.elements.text.oninput();form.onsubmit({preventDefault(){}});await tick();assert.deepEqual(shown,['new']);release({id:'old',status:'failed'});await tick();assert.deepEqual(shown,['new']);panel.dispose();
});
