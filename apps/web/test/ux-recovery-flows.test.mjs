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
test('first setup confirms the exact password before calling authentication',async()=>{
 const host=element(),form=host.querySelector('form'),result=host.querySelector('[data-result]');let calls=0,focused=false;
 form.elements={token:{value:'controlled-setup-token'},password:{value:' exact password '},confirmPassword:{value:'mismatch',focus(){focused=true;}}};
 form.querySelector=selector=>selector==='button'?{disabled:false}:result;
 const render=load('pages/real-core.js',{store:{get:()=>null},auth:{setupStatus:async()=>({setupAvailable:true}),setup:async(token,password)=>{calls++;assert.equal(token,'controlled-setup-token');assert.equal(password,' exact password ');}},
 Field:()=>'',errorMessage,document:{},toast(){},esc},'renderRealAuth');
 const dispose=render(host,{navigate(){},refresh(){}});
 await tick();
 await form.onsubmit({preventDefault(){},target:form});
 assert.equal(calls,0);assert.equal(focused,true);assert.match(result.textContent,/两次输入的密码不一致/);
 form.elements.confirmPassword.value=form.elements.password.value;
 await form.onsubmit({preventDefault(){},target:form});
 assert.equal(calls,1);dispose();
});
const storage=()=>{const values=new Map();return {getItem:k=>values.get(k)??null,setItem:(k,v)=>values.set(k,v),removeItem:k=>values.delete(k)};};
function load(file,bindings,name){const text=readFileSync(new URL('../src/v3/proto/'+file,import.meta.url),'utf8').replace(/^import .+;\s*$/gm,'').replace(/^export \{.+;\s*$/gm,'').replaceAll('export ','');return Function(...Object.keys(bindings),text+'\nreturn '+name)(...Object.values(bindings));}
function element(){const nodes=new Map();return {isConnected:true,innerHTML:'',querySelector(s){if(!nodes.has(s))nodes.set(s,{isConnected:true,innerHTML:'',textContent:'',hidden:false,disabled:false,insertAdjacentHTML(_where,text){this.innerHTML+=text;},addEventListener(){}});return nodes.get(s);},querySelectorAll:()=>[]};}

function creationPage({analyze,activated=false}={}){
 const saved=storage(),recovery=creationDraft({owner:'owner',storage:saved,timezone:'Asia/Shanghai'}),el=element();let activations=0;
 const query=el.querySelector.bind(el);el.querySelector=s=>{const n=query(s);n.addEventListener=(type,fn)=>{n['on'+type]=fn;};n.querySelectorAll=()=>[];n.focus=()=>n.focused=true;return n;};
 const form=el.querySelector('[data-source]');form.elements={};form.reportValidity=()=>true;
 recovery.save({...recovery.read(),bio:'她喜欢看书',...(activated?{activatedId:'original-role',groupConfirmed:true,worldRuntimeId:'original-world',worldSelectionConfirmed:true}:{})});
 const render=load('pages/real-create.js',{client:{},store:{get:k=>k==='session'?{actorId:'owner'}:{status:'正常'},list:()=>[]},creationDraft,checkpointCreation,checkpointCreationAnalysis,selectCreationWorld,creationAnalysisRequest,parseCreationAnalysis,persistentCommand,studioDraftStorage:()=>saved,esc,Field:()=>'',CreationPreview:()=>'',creationFeedback:c=>({message:c.message}),JsonDetails:()=>'',document:{hidden:false,addEventListener(){},removeEventListener(){}},setTimeout:()=>1,clearTimeout(){},requireOwner:async()=>true,directory:{analyze,activate:async()=>{activations++;return {id:'created'};}},activationFromAnalysis:()=>({}),readCreationProgress:async()=>({complete:true,stages:[],worldConfirmed:true}),localDate:()=> '2026-10-23'},'renderRealCreate');
 const dispose=render(el,{query:{},navigate(){}});return {el,recovery,dispose,activations:()=>activations};
}
const validAnalysis={analysis_id:'initialization_source_'+'a'.repeat(32),correlation_id:'real-analysis-request',schema_version:1,core_persona:{},developing_self:{},extensions:{},initial_goals:[],initial_intentions:[],initial_relationships:[]};

test('a stale analysis response cannot continue activation using a replacement draft',async()=>{
 let release;const response=new Promise(resolve=>release=resolve),page=creationPage({analyze:()=>response});
 try{page.el.querySelector('[data-start]').onclick();await tick();const newer={...page.recovery.read(),requestId:'replacement',name:'新草稿',bio:'保留新输入',analysis:{unrelated:true},wizardStep:1};page.recovery.save(newer);release(validAnalysis);await tick();assert.equal(page.activations(),0);assert.deepEqual(page.recovery.read(),newer);}finally{page.dispose();}
});
test('analysis without a name returns to editable input and a second creation starts with description',async()=>{
 const page=creationPage({analyze:async()=>validAnalysis});
 try{page.el.querySelector('[data-start]').onclick();await tick();assert.equal(page.recovery.read().wizardStep,1);assert.equal(page.el.querySelector('[data-name-options]').open,true);assert.equal(page.el.querySelector('[name="name"]').focused,true);assert.equal(page.activations(),0);}finally{page.dispose();}
 const next=creationPage({activated:true});
 try{await tick();next.el.querySelector('[data-new-creation]').onclick();assert.equal(next.recovery.read().wizardStep,1);assert.equal(next.recovery.read().useDefaultWorld,true);assert.match(next.el.innerHTML,/这是一个什么样的人/);}finally{next.dispose();}
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

test('library shows durable creation continuation and characters whose group has not loaded',()=>{
 const saved=storage(),recovery=creationDraft({owner:'owner',storage:saved,timezone:'Asia/Shanghai'});recovery.save({...recovery.read(),name:'小林',wizardStep:2});
 const role={id:'role',groupId:'unloaded-group',name:'晚晴',status:'正常'};const el=element();
 const render=load('pages/real-core.js',{store:{get:k=>k==='session'?{actorId:'owner'}:null,list:k=>k==='characters'?[role]:[]},creationDraft,studioDraftStorage:()=>saved,esc,CharacterCard:c=>`<article>${esc(c.name)}</article>`,bindAvatars(){},Field(){},Tabs(){},randomId(){},directory:{},auth:{},toast(){},drawer(){},confirm(){},requireOwner(){}},'renderRealLibrary');
 render(el,{query:{},navigate(){}});assert.match(el.innerHTML,/小林 · 正在诞生/);assert.match(el.innerHTML,/继续创建与查看进度/);assert.match(el.innerHTML,/<article>晚晴<\/article>/);
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
