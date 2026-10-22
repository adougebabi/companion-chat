import {errorMessage} from '../src/v3/domain-presentation.mjs';
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {userInstant} from '../src/v3/presentation-time.mjs';
import {persistentCommand} from '../src/v3/providers/persistent-command.mjs';

const comparisonSource=readFileSync(new URL('../src/v3/proto/components/studio-comparison.js',import.meta.url),'utf8').replace(/^import .+;\s*$/gm,'').replaceAll('export function ','function ');
const {StudioComparison,studioChanges}=Function('esc','DiffRows','semanticLabel',comparisonSource+'\nreturn {StudioComparison,studioChanges};')(String,JSON.stringify,key=>key);
const tick=()=>new Promise(resolve=>setImmediate(resolve));
function storage(){const values=new Map();return {values,getItem:k=>values.get(k)??null,setItem:(k,v)=>values.set(k,v),removeItem:k=>values.delete(k)};}
function host(){const nodes=new Map();return {isConnected:true,innerHTML:'',nodes,querySelector(selector){if(!nodes.has(selector)){const events=new Map();const node={value:'',disabled:false,events,addEventListener:(type,fn)=>events.set(type,fn),querySelector:()=>({disabled:false}),querySelectorAll:()=>[],contains:()=>false};nodes.set(selector,node);}return nodes.get(selector);},querySelectorAll:()=>[]};}
function component(bindings){
 const source=readFileSync(new URL('../src/v3/proto/components/studio-jobs.js',import.meta.url),'utf8').replace(/^import .+;\s*$/gm,'').replaceAll('export function ','function ');
 return Function(...Object.keys(bindings),`${source}\nreturn mountStudioJobs;`)(...Object.values(bindings));
}
function fixture(client){
 const saved=storage(),timers=[],listeners=new Map(),document={hidden:false,addEventListener:(name,fn)=>listeners.set(name,fn),removeEventListener:(name)=>listeners.delete(name)};const mount=component({studioClient:{referenceGrants:async()=>({items:[],next_cursor:''}),...client},referenceFromGrant:g=>({assetId:g.asset_id,purpose:g.purpose,grantId:g.id,grantRevision:g.revision}),StudioComparison,studioChanges,userInstant,errorMessage,mediaReferencePurposes:{person:'人物',object:'物品'},esc:String,confirm:async()=>true,requireOwner:async()=>true,toast:()=>{},Field:()=>'',TextComposer:()=>'',PromptChips:()=>'',DiffRows:JSON.stringify,MessageLog:()=>'',PreviewCard:p=>JSON.stringify(p),JsonDetails:JSON.stringify,persistentCommand,studioDraftStorage:()=>saved,history:{replaceState(){}},document,setTimeout:fn=>{timers.push(fn);return timers.length;},clearTimeout:()=>{}});
 return {saved,mount,timers,listeners,document};
}
test('late assistant completion displays a separate suggestion and preserves newer form edits until explicit adoption',async()=>{
 let finish;const detail=new Promise(resolve=>{finish=resolve;});let adopted=0;
 const {saved,mount}=fixture({job:()=>detail,jobs:async()=>({items:[],next_cursor:''}),materials:async()=>({items:[],next_cursor:''})});
 saved.setItem('fluctlight-studio:owner:resource:jobs:draft',JSON.stringify({selected:'job',instruction:'Original request'}));
 const draft={name:'Original name',body:{description:'Original draft'}},el=host();
 const panel=mount(el,{actor:'owner',resource:{id:'resource',revision:1},readDraft:()=>draft,adopt:value=>{adopted++;Object.assign(draft,value);},alive:()=>true});
 draft.name='Newer form edit';draft.body.description='Typed while model ran';
 finish({id:'job',resource_id:'resource',status:'completed',kind:'assistant',revision:3,latest_attempt:1,snapshot:{instruction:'Original request'},result:{disposition:'proposal',reply:'Suggested definition',name:'Generated name',body:{description:'Generated definition'},warnings:[]},attempts:[]});
 await tick();assert.equal(adopted,0);assert.equal(draft.name,'Newer form edit');assert.equal(draft.body.description,'Typed while model ran');
 await el.querySelector('[data-adopt]').onclick();assert.equal(adopted,1);assert.equal(draft.name,'Generated name');panel.dispose();
});
test('late accepted job after disposal and Owner storage cleanup cannot repaint or recreate the previous Owner draft',async()=>{
 let finish;const receipt=new Promise(resolve=>{finish=resolve;});
 const {saved,mount}=fixture({createJob:()=>receipt,jobs:async()=>({items:[],next_cursor:''}),materials:async()=>({items:[],next_cursor:''})});
 const el=host(),panel=mount(el,{actor:'owner',resource:{id:'resource',revision:1},readDraft:()=>({name:'Draft',body:{description:'Body'}}),alive:()=>true});
 await tick();const input=el.querySelector('[name=t]');input.value='Generate isolated preview';input.oninput();
 el.querySelector('[data-studio-compose]').onsubmit({preventDefault(){}});await tick();
 assert.ok([...saved.values.keys()].some(k=>k.endsWith(':command')));
 panel.dispose();saved.values.clear();el.innerHTML='Another Owner page';finish({id:'accepted-old-job'});await tick();await tick();
 assert.equal(el.innerHTML,'Another Owner page');assert.equal(saved.values.size,0);
});

function draftHelpers(){
 const source=readFileSync(new URL('../src/v3/proto/components/studio-working-draft.js',import.meta.url),'utf8').replace(/^import .+;\s*$/gm,'').replaceAll('export function ','function ').replaceAll('export const ','const ');
 return Function(`${source}\nreturn {normalizeStudioDraft};`)();
}
test('preview initializes a full resource draft and preserves an existing CAS baseline and local edits',()=>{
 const {normalizeStudioDraft}=draftHelpers(),resource={kind:'event_template',name:'Saved',draft_version:'v5',body:{schema:'world-event.v1'}};
 const fresh=normalizeStudioDraft(null,resource);
 assert.equal(fresh.kind,'event_template');assert.equal(fresh.expectedVersion,'v5');assert.equal(fresh.reason,'');assert.equal(fresh.target,'');
 const recovered=normalizeStudioDraft({name:'Local',body:{custom:true},expectedVersion:'v2',reason:'Keep reason',target:'chosen'},resource);
 assert.equal(recovered.expectedVersion,'v2');assert.equal(recovered.name,'Local');assert.deepEqual(recovered.body,{custom:true});assert.equal(recovered.target,'chosen');
 const partial=normalizeStudioDraft({name:'Adopted',body:{custom:true}},resource);assert.equal(partial.expectedVersion,'v5');assert.equal(partial.kind,'event_template');
 assert.throws(()=>normalizeStudioDraft([],resource),/格式损坏/);assert.throws(()=>normalizeStudioDraft({body:[]},resource),/格式损坏/);
 const created=normalizeStudioDraft(null,null,{kind:'place',body:{description:''}});assert.equal(created.kind,'place');assert.equal(created.expectedVersion,undefined);
});
test('needs_input never exposes an adoption action or mutates the working draft',async()=>{
 let adopted=0;const {mount}=fixture({job:async()=>({id:'j',resource_id:'resource',kind:'assistant',status:'completed',result:{disposition:'needs_input',reply:'请选择地点',warnings:[]}}),jobs:async()=>({items:[]}),materials:async()=>({items:[]})});
 const el=host(),draft={name:'Keep',body:{}};const panel=mount(el,{actor:'owner',resource:{id:'resource'},jobId:'j',readDraft:()=>draft,adopt:()=>adopted++,alive:()=>true});await tick();
 assert.match(el.querySelector('[data-job-result]').innerHTML,/需要补充创作输入/);assert.doesNotMatch(el.querySelector('[data-job-result]').innerHTML,/data-adopt/);
 await el.querySelector('[data-adopt]').onclick();assert.equal(adopted,0);assert.equal(draft.name,'Keep');panel.dispose();
});
test('background observation preserves older task pages, composer node and selection, and pauses while hidden',async()=>{
 let jobs=0,materials=0,details=0;
 const f=fixture({job:async()=>{details++;return {id:'j',resource_id:'resource',status:'running',kind:'assistant'};},jobs:async({cursor})=>{jobs++;return {items:[{id:cursor?'older':'first',kind:'assistant',created_at:'now',status:'running'}],next_cursor:cursor?'':'older-cursor'};},materials:async()=>{materials++;return {items:[]};}});
 const el=host(),panel=f.mount(el,{actor:'owner',resource:{id:'resource'},jobId:'j',readDraft:()=>({name:'Keep',body:{}}),alive:()=>true});await tick();
 const input=el.querySelector('[name=t]');input.value='中文输入尚未提交';input.selectionStart=3;input.oninput();
 await el.querySelector('[data-more]').events.get('click')();await tick();assert.match(el.querySelector('[data-job-directory]').innerHTML,/older/);
 const shell=el.innerHTML;await f.timers.at(-1)();await tick();assert.equal(jobs,2);assert.equal(materials,1);assert.ok(details>=2);
 assert.equal(el.innerHTML,shell);assert.equal(el.querySelector('[name=t]'),input);assert.equal(input.value,'中文输入尚未提交');assert.equal(input.selectionStart,3);assert.match(el.querySelector('[data-job-directory]').innerHTML,/older/);
 f.document.hidden=true;f.listeners.get('visibilitychange')();const before=details;await f.timers.at(-1)();await tick();assert.equal(details,before);
 f.document.hidden=false;f.listeners.get('visibilitychange')();await tick();assert.equal(details,before+1);assert.equal(jobs,2);panel.dispose();assert.equal(f.listeners.size,0);
});

test('image request freezes selected retained reference and keeps image choices out of text submissions',async()=>{
 const submitted=[];const f=fixture({createJob:async b=>{submitted.push(structuredClone(b));return {id:'accepted'};},job:async()=>null,jobs:async()=>({items:[]}),materials:async()=>({items:[]})});
 const el=host(),panel=f.mount(el,{actor:'owner',resource:{id:'resource',revision:4},readDraft:()=>({name:'Scene',body:{customField:'unchanged'}}),alive:()=>true});await tick();
 const instruction=el.querySelector('[name=t]');instruction.value='Render frozen source';instruction.oninput();
 el.querySelector('[name=kind]').onchange({target:{value:'image_preview'}});
 el.querySelector('[name=referenceAsset]').onchange({target:{value:'asset-original'}});
 el.querySelector('[data-studio-compose]').onsubmit({preventDefault(){}});await tick();assert.equal(submitted.length,0);assert.match(el.querySelector('[data-job-feedback]').innerHTML,/明确选择参考用途/);
 el.querySelector('[name=referencePurpose]').onchange({target:{value:'scene'}});
 el.querySelector('[data-studio-compose]').onsubmit({preventDefault(){}});await tick();await tick();
 assert.equal(submitted[0].kind,'image_preview');assert.deepEqual(submitted[0].referenceAsset,{assetId:'asset-original',purpose:'scene'});assert.deepEqual(submitted[0].body,{customField:'unchanged'});assert.equal(submitted[0].resourceRevision,4);
 el.querySelector('[name=kind]').onchange({target:{value:'text_preview'}});
 el.querySelector('[data-studio-compose]').onsubmit({preventDefault(){}});await tick();await tick();
 assert.equal(submitted[1].kind,'text_preview');assert.equal(Object.hasOwn(submitted[1],'referenceAsset'),false);panel.dispose();
});

test('image pending state shows actual workflow evidence and only ready result exposes image and explicit material save',async()=>{
 let ready=false,saves=0;const savedImage={id:'mat',owner_actor_id:'owner',resource_id:'resource',job_id:'j',asset_id:'asset-ready',text:null,label:'虚构图片素材'};
 const f=fixture({mediaUrl:id=>`/api/media/${encodeURIComponent(id)}`,job:async()=>({id:'j',owner_actor_id:'owner',resource_id:'resource',kind:'image_preview',status:ready?'completed':'running',revision:3,latest_attempt:1,result:ready?{asset_id:'asset-ready',url:'/api/media/asset-ready',label:'虚构创作预览·不进入生活'}:{media_intent_id:'intent'},media_requests:[{id:'intent',status:ready?'ready':'pending',workflow_id:'actual-workflow',provider_job_id:'actual-provider-job',workflows:[{workflow_id:'actual-child',status:'started'}]}]}),jobs:async()=>({items:[]}),materials:async()=>({items:saves?[savedImage]:[]}),material:async id=>{assert.equal(id,'mat');return savedImage;},saveMaterial:async(id,body)=>{saves++;assert.equal(id,'j');assert.equal(body.expectedRevision,3);return {id:'mat',asset_id:'asset-ready',status:'saved'};}});
 const el=host(),panel=f.mount(el,{actor:'owner',resource:{id:'resource'},jobId:'j',readDraft:()=>({name:'Scene',body:{}}),alive:()=>true});await tick();
 let result=el.querySelector('[data-job-result]').innerHTML;assert.match(result,/actual-workflow/);assert.match(result,/actual-provider-job/);assert.match(result,/actual-child/);assert.doesNotMatch(result,/data-material|"img"/);
 ready=true;el.querySelector('[data-refresh]').onclick();await tick();await tick();result=el.querySelector('[data-job-result]').innerHTML;assert.match(result,/"img":"\/api\/media\/asset-ready"/);assert.match(result,/存为资源图片素材/);
 await el.querySelector('[data-material]').onclick();await tick();await tick();assert.equal(saves,1);assert.match(el.querySelector('[data-job-materials]').innerHTML,/\/api\/media\/asset-ready/);assert.match(el.querySelector('[data-job-materials]').innerHTML,/查看原任务/);assert.match(el.querySelector('[name=referenceAsset]').innerHTML,/asset-ready/);panel.dispose();
});

test('unconfigured image failure shows actionable status without fabricated image or save action',async()=>{
 const f=fixture({job:async()=>({id:'j',resource_id:'resource',kind:'image_preview',status:'failed',error_code:'studio_media_not_configured',result:{media_intent_id:'accepted-only'}}),jobs:async()=>({items:[]}),materials:async()=>({items:[{id:'foreign',asset_id:'foreign-image',owner_actor_id:'other',resource_id:'resource',job_id:'other'}]})});
 // The reference picker must never offer another Owner's material.
 const el=host(),panel=f.mount(el,{actor:'owner',resource:{id:'resource'},jobId:'j',readDraft:()=>({name:'Scene',body:{}}),alive:()=>true});await tick();
 const result=el.querySelector('[data-job-result]').innerHTML;assert.match(result,/图片生成尚未配置/);assert.match(result,/data-retry/);assert.doesNotMatch(result,/"img"|data-material/);assert.doesNotMatch(el.querySelector('[name=referenceAsset]').innerHTML,/foreign-image/);panel.dispose();
});

test('lost image acceptance replays original reference and key while newer reference choices remain a separate draft',async()=>{
 const calls=[];let first=true;const f=fixture({createJob:async body=>{calls.push(structuredClone(body));if(first){first=false;throw new Error('transport lost');}return {id:'accepted-original'};},job:async()=>null,jobs:async()=>({items:[]}),materials:async()=>({items:[]})});
 const el=host(),panel=f.mount(el,{actor:'owner',resource:{id:'resource',revision:2},initialKind:'image_preview',readDraft:()=>({name:'Frozen',body:{description:'Scene'}}),alive:()=>true});await tick();
 const input=el.querySelector('[name=t]');input.value='Render original';input.oninput();
 el.querySelector('[name=referenceAsset]').onchange({target:{value:'original-asset'}});el.querySelector('[name=referencePurpose]').onchange({target:{value:'object'}});
 el.querySelector('[data-studio-compose]').onsubmit({preventDefault(){}});await tick();await tick();
 el.querySelector('[name=referenceAsset]').onchange({target:{value:'newer-asset'}});el.querySelector('[name=referencePurpose]').onchange({target:{value:'animal'}});
 await el.querySelector('[data-replay]').events.get('click')();await tick();await tick();
 assert.equal(calls.length,2);assert.deepEqual(calls[1],calls[0]);assert.deepEqual(calls[1].referenceAsset,{assetId:'original-asset',purpose:'object'});
 const draft=JSON.parse(f.saved.getItem('fluctlight-studio:owner:resource:jobs:draft'));assert.equal(draft.referenceAssetId,'newer-asset');assert.equal(draft.referencePurpose,'animal');panel.dispose();
});

test('an explicit reference choice freezes actual grant identity and revision into an image task',async()=>{
 const calls=[],grant={id:'owner-grant',asset_id:'uploaded-image',revision:3,purpose:'person',status:'active'};
 const f=fixture({referenceGrants:async()=>({items:[grant],next_cursor:''}),createJob:async body=>{calls.push(body);return {id:'real-job'};},job:async()=>null,jobs:async()=>({items:[]}),materials:async()=>({items:[]})});
 const el=host(),panel=f.mount(el,{actor:'owner',resource:{id:'resource',revision:2},initialKind:'image_preview',readDraft:()=>({name:'Frozen',body:{description:'Fictional illustration'}}),alive:()=>true});await tick();
 el.querySelector('[name=referenceAsset]').onchange({target:{value:'grant:owner-grant'}});assert.equal(el.querySelector('[name=referencePurpose]').disabled,true);
 const input=el.querySelector('[name=t]');input.value='Use only explicitly allowed image';input.oninput();el.querySelector('[data-studio-compose]').onsubmit({preventDefault(){}});await tick();await tick();
 assert.deepEqual(calls[0].referenceAsset,{assetId:'uploaded-image',purpose:'person',grantId:'owner-grant',grantRevision:3});panel.dispose();
});
