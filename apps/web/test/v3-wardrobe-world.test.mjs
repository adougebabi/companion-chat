import test from 'node:test';
import {errorMessage} from '../src/v3/domain-presentation.mjs';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {persistentCommand} from '../src/v3/providers/persistent-command.mjs';
import {beginDraftSubmission,settleDraftSubmission} from '../src/v3/proto/components/draft-submission.js';
import {WorldFrameworkPreview,WorldVenueDirectory} from '../src/v3/proto/components/world-venues.js';
function load(file,bindings,exports){const source=readFileSync(new URL(`../src/v3/proto/components/${file}`,import.meta.url),'utf8').replace(/^import .+;\s*$/gm,'').replaceAll('export function ','function ');return Function('errorMessage',...Object.keys(bindings),`${source}\nreturn {${exports}};`)(errorMessage,...Object.values(bindings));}
const storage=()=>{const m=new Map();return {getItem:k=>m.get(k)??null,setItem:(k,v)=>m.set(k,v),removeItem:k=>m.delete(k)};};
function drawerFactory(){const opened=[];function drawer(){const fields=new Map();const el={isConnected:true,querySelector:s=>{if(!fields.has(s))fields.set(s,{value:s==='[name=reason]'?'保存修改':'',reportValidity:()=>true,hidden:false,disabled:false,textContent:''});return fields.get(s);}};const value={el,body:{innerHTML:''},close(){el.isConnected=false;}};opened.push(value);return value;}return {drawer,opened};}
test('wardrobe metadata command preserves exact unknown request through drawer remount and module reload',async()=>{
 const store=storage(),ui=drawerFactory(),calls=[];let fail=true,ack;
 const bindings={...ui,esc:String,confirm:async()=>true,toast(){},Field:()=>'',JsonDetails:JSON.stringify,get:()=>({actorId:'owner-one'}),subscribe(){},persistentCommand,studioDraftStorage:()=>store,beginDraftSubmission,settleDraftSubmission};
 const options={persistent:true,scope:['fid','profile-a','rename','outfit'],title:'rename',note:'save',revision:1,target:{name:'A'},execute:async(body,target)=>{calls.push({body,target});if(fail)throw Object.assign(new Error('network lost'),{status:502});return {saved:true};},onSaved:(_r,target)=>ack=target};
 load('governance-command.js',bindings,'governanceCommand').governanceCommand(options);await ui.opened.at(-1).el.querySelector('[data-submit]').onclick();
 fail=false;load('governance-command.js',bindings,'governanceCommand').governanceCommand({...options,target:{name:'new draft B'}});await ui.opened.at(-1).el.querySelector('[data-submit]').onclick();
 assert.equal(calls.length,2);assert.deepEqual(calls[0],calls[1]);assert.ok(calls[0].body.idempotencyKey);assert.deepEqual(ack,{name:'A'});
});
test('first explicit wardrobe CAS rejection requires a fresh read and keeps the intended name',async()=>{
 const store=storage(),ui=drawerFactory();let count=0,read=0,last;
 const {governanceCommand}=load('governance-command.js',{...ui,esc:String,confirm:async()=>true,toast(){},Field:()=>'',JsonDetails:JSON.stringify,get:()=>({actorId:'owner-cas'}),subscribe(){},persistentCommand,studioDraftStorage:()=>store,beginDraftSubmission,settleDraftSubmission},'governanceCommand');
 governanceCommand({persistent:true,scope:['fid','profile-a','rename','outfit'],revision:1,target:{name:'保留名称',expectedOutfitRevision:2},read:async()=>{read++;return {revision:7,outfit:{revision:8}};},rebaseTarget:(latest,value)=>({...value,expectedOutfitRevision:latest.outfit.revision}),execute:async(body,target)=>{if(++count===1)throw Object.assign(new Error('stale'),{status:409,code:'revision_conflict'});last={body,target};return {saved:true};}});
 const d=ui.opened.at(-1);await d.el.querySelector('[data-submit]').onclick();await d.el.querySelector('[data-rebase]').onclick();await d.el.querySelector('[data-submit]').onclick();assert.equal(read,1);assert.equal(last.body.expectedRevision,7);assert.deepEqual(last.target,{name:'保留名称',expectedOutfitRevision:8});
});
test('wardrobe presentation drafts stay profile scoped and old acknowledgement cannot clear a newer drawer draft',()=>{
 const store=storage();const {wardrobeDraft}=load('wardrobe-drafts.js',{get:()=>({actorId:'owner'}),studioDraftStorage:()=>store},'wardrobeDraft');
 const a=wardrobeDraft(['fid','a','tags'],{rows:[]});a.set({rows:['A']});const b=wardrobeDraft(['fid','b','tags'],{rows:[]});assert.deepEqual(b.get(),{rows:[]});
 const newer=wardrobeDraft(['fid','a','tags'],{rows:[]});newer.set({rows:['B']});a.acknowledge({rows:['A']});assert.deepEqual(wardrobeDraft(['fid','a','tags'],{}).get(),{rows:['B']});newer.acknowledge({rows:['B']});assert.deepEqual(wardrobeDraft(['fid','a','tags'],{rows:[]}).get(),{rows:[]});
});
test('name-only library save uses PATCH provider and preserves archived combination membership',()=>{
 const ui=drawerFactory();let config;
 const {renameOutfit}=load('outfit-library.js',{...ui,Field:()=>'',wardrobeDraft:()=>({get:()=>({name:'原名称'}),set(){},acknowledge(){}}),governanceCommand:c=>config=c,inventoryClient:{renameOutfit:(_f,_id,body)=>body},requestInventoryAction(){},OutfitPreview(){},esc:String,toast(){}},'renameOutfit');
 renameOutfit('fid',{id:'plan',name:'原名称',profile_id:'profile-a',revision:4,items:[{id:'archived',archived_at:'old'}],tags:['在家']},'profile-a',9,()=>{});
 ui.opened.at(-1).el.querySelector('[name=name]').value='新名称';ui.opened.at(-1).el.querySelector('[data-save]').onclick();
 assert.deepEqual(config.target,{name:'新名称',profileId:'profile-a',expectedOutfitRevision:4});const body=config.execute({expectedRevision:9,reason:'改名',idempotencyKey:'same'},config.target);assert.equal(body.name,'新名称');assert.equal(body.itemIds,undefined);assert.equal(body.tags,undefined);assert.equal(body.idempotencyKey,'same');
});
test('world preview is human-readable and grouped categories do not become concrete navigation points',()=>{
 const html=WorldFrameworkPreview({venues:[{name:'家',places:[{name:'卧室',furniture:[{name:'木床',description:'窗边'},'书桌'],floor:'二层',layout:'窗朝东'}]}]});assert.match(html,/木床|书桌/);assert.doesNotMatch(html,/\[object Object\]/);assert.match(html,/<details/);assert.doesNotMatch(html,/<details[^>]*open/);
 const directory=WorldVenueDirectory({categories:[{id:'home',name:'家'}],places:[{id:'bedroom',name:'卧室',venueCategoryId:'home'},{id:'old',name:'旧地点'}],renderPlace:p=>`<a href="#/${p.id}">${p.name}</a>`});assert.match(directory,/固定场地分类/);assert.match(directory,/原有未归类地点/);assert.doesNotMatch(directory,/href="#\/home"/);assert.match(directory,/href="#\/bedroom"/);
});
