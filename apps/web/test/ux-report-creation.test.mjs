import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {worldDefinitionError,newWorldDefinition} from '../src/v3/proto/components/world-definition.js';
test('world source rejects meaningless shells without inventing defaults and accepts legacy authored narrative',()=>{
 assert.match(worldDefinitionError({}),/空世界底稿/);
 assert.match(worldDefinitionError(newWorldDefinition()),/空世界底稿/);
 assert.match(worldDefinitionError({background:'  ',rules:'\n'}),/空世界底稿/);
 assert.equal(worldDefinitionError({background:'山间小城，居民过着平静生活',custom:{unknown:'preserved'}}),'');
 assert.deepEqual(newWorldDefinition(),{background:'',rules:''});
});
test('world framework validates actual category/place identity, limits and forbidden runtime references',()=>{
 const body={background:'由所有者编写的世界',framework:{schema:'world-framework.v1',venues:[{key:'home',name:'居所',places:[{key:'room',name:'卧室',description:'安静的房间'}]}]}};
 assert.equal(worldDefinitionError(body),'');
 const change=fn=>{const draft=structuredClone(body);fn(draft.framework);return worldDefinitionError(draft);};
 assert.match(change(f=>f.schema='invented'),/world-framework.v1/);
 assert.match(change(f=>f.venues=[]),/1–32/);
 assert.match(change(f=>f.venues[0].key=123),/唯一/);
 assert.match(change(f=>f.venues[0].places[0].name=456),/唯一/);
 assert.match(change(f=>f.venues.push(structuredClone(f.venues[0]))),/唯一/);
 assert.match(change(f=>f.venues[0].places=[]),/1–200/);
 assert.match(change(f=>f.venues[0].places[0].place_id='runtime-place'),/运行世界ID/);
 assert.match(change(f=>f.venues[0].places[0].description='字'.repeat(8001)),/描述/);
 assert.equal(change(f=>f.venues[0].places[0].atmosphere='字'.repeat(2000)),'');
});

function npcFixture(){
 const text=readFileSync(new URL('../src/v3/proto/pages/npc-management.js',import.meta.url),'utf8');
 const code=text.slice(text.indexOf('async function renderNPCDefinitionPage'),text.indexOf('\nexport function renderNPCEditor'));
 // Keep this fixture scoped to the real definition renderer only.
 const functionSource=code.slice(0,code.indexOf('\n}',code.indexOf('if(draft.charId)'))+2);
 const saved={id:'npc',kind:'npc',type:'NPC',name:'原名称',body:{role:'原简介',background:'原背景'},draftVersion:'v1',draft_version:'v1',draft:{},published:'v1',versions:{v1:{}},version:1};
 let resource=structuredClone(saved),lastDraft,finishSave;
 const nodes=new Map(),inputs=['name','role','bio'].map(key=>({dataset:{draft:key},value:''}));
 const node=selector=>{if(!nodes.has(selector))nodes.set(selector,{textContent:'',innerHTML:'',value:'',disabled:false,focus(){},addEventListener(type,handler){this['on'+type]=handler;},querySelectorAll:()=>[],reportValidity:()=>true,elements:{name:{focus(){}}}});return nodes.get(selector);};
 const el={innerHTML:'',querySelector:node,querySelectorAll:selector=>selector==='[data-draft]'?inputs:[]};
 const bindings={store:{get:key=>key==='session'?{actorId:'owner'}:null,list:()=>[],isMock:true},mockClient:{resource:async()=>structuredClone(resource),saveNPCDraft:()=>new Promise(resolve=>finishSave=resolve)},mockWorld:{refresh:async()=>{}},readStudioDraft:()=>({name:saved.name,body:structuredClone(saved.body),expectedVersion:'v1'}),writeStudioDraft:(a,k,d)=>{lastDraft=structuredClone(d);},studioWorkingDraftKey:()=>'',studioDraftStorage:()=>({removeItem(){}}),persistentCommand:({execute})=>({pending:()=>null,submit:value=>execute(value,'same-key')}),requireOwner:async()=>true,esc:String,icon:()=>'',badge:()=>'',Field:()=>'',Button:()=>'',DiffView:JSON.stringify,mountStudioJobs:()=>({dispose(){},updateDraft(){}}),mountResourceLifecycle:()=>{},toast:()=>{},confirm:async()=>true,errorMessage:error=>error.message,placeOptions:()=>[]};
 const render=Function(...Object.keys(bindings),`${functionSource};return renderNPCDefinitionPage;`)(...Object.values(bindings));
 return {render,el,inputs,node,draft:()=>lastDraft,accept(){resource={...saved,draftVersion:'v2',name:'新名称'};finishSave(structuredClone(resource));}};
}
test('NPC input updates the actual local diff, reverting clears dirty, and whitespace name sends no save',async()=>{
 const f=npcFixture();await f.render(f.el,{params:{resId:'npc'},query:{},charId:'c'});
 f.inputs[0].value='新名称';f.inputs[0].oninput();assert.equal(f.draft().dirty,true);assert.match(f.node('[data-npc-diff]').innerHTML,/新名称/);
 f.inputs[0].value='原名称';f.inputs[0].oninput();assert.equal(f.draft().dirty,false);
 f.inputs[0].value='   ';f.inputs[0].oninput();f.node('[data-definition]').onsubmit({preventDefault(){},target:f.node('[data-definition]')});assert.match(f.node('[data-definition-error]').textContent,/不能只输入空格/);
});
test('NPC save settles its saved baseline while edits typed during the pending save remain dirty',async()=>{
 const tick=()=>new Promise(resolve=>setImmediate(resolve));
 for(const newer of [false,true]){
  const f=npcFixture();await f.render(f.el,{params:{resId:'npc'},query:{},charId:'c'});
  f.inputs[0].value='新名称';f.inputs[0].oninput();
  f.node('[data-definition]').onsubmit({preventDefault(){},target:f.node('[data-definition]')});await tick();
  if(newer){f.inputs[0].value='保存期间的新名称';f.inputs[0].oninput();}
  f.accept();await tick();await tick();
  assert.equal(f.draft().expectedVersion,'v2');assert.equal(f.draft().dirty,newer);assert.equal(f.draft().name,newer?'保存期间的新名称':'新名称');
 }
});
