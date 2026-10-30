import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {installDOM,mountReact} from './helpers/react-dom.mjs';
import {portraitPresentation} from '../src/approved-ui/profile-presentation.mjs';

const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const {default:Directory}=await vite.ssrLoadModule('/src/react-app/pages/StudioDirectory.jsx');
function workspace(owner){
 const state=proxy({session:{actorId:owner},selectedId:'',characters:[],operations:[],errors:{}}),reads=[];
 return {state,reads,current:{value:{id:''}},subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),navigatePath(){},client:{listStudioResources:async options=>{reads.push(options);return {items:[],next_cursor:''};}},services:{studioClient:{}}};
}
const directoryProps=(extra={})=>({route:{path:'/studio',query:new URLSearchParams()},...extra});

test('r20 portrait scalar interpretation preserves prose, unknowns, different values and exact source occurrence counts',()=>{
 const source='response_style: warm\npunctuation_style: natural\nhumor_style: gentle\nconflict_style: direct\nrefusal_style: clear\nintimacy_expression: reserved\nconflict_style: direct\nconflict_style: gentle\nunknown: direct\n原文 conflict_style: direct\n conflict_style: direct\nconflict_style: direct 后面的原文';
 const rows=portraitPresentation(source,{includeOccurrences:true});
 assert.deepEqual(rows.find(row=>row.field==='conflict_style'&&row.value==='direct'),{field:'conflict_style',value:'direct',occurrences:2});
 assert.equal(rows.filter(row=>row.field==='conflict_style').length,3);
 for(const field of ['response_style','punctuation_style','humor_style','refusal_style','intimacy_expression'])assert.equal(rows.find(row=>row.field===field).occurrences,1);
 for(const text of ['unknown: direct','原文 conflict_style: direct',' conflict_style: direct'])assert.ok(rows.some(row=>row.text===text));
 assert.equal(portraitPresentation('conflict_style: direct\nconflict_style: direct').length,1);
 assert.deepEqual(portraitPresentation('conflict_style: direct')[0],{field:'conflict_style',value:'direct'});
 assert.equal(portraitPresentation('conflict_style: direct',{includeOccurrences:true})[0].occurrences,1);
 assert.equal(source.split('\n').length,12);
});

test('r20 Studio library leave resets filters, editor return and refresh preserve unsent search, explicit queries retain priority',async()=>{
 const owner='r20-studio',key=`fluctlight-studio:${owner}:directory-ui`,worldKey=key+':world',commandKey=`fluctlight-studio:${owner}:editor:original:command`,w=workspace(owner);
 sessionStorage.setItem(key,JSON.stringify({kind:'item',query:'已提交',search:'未提交',archived:true}));sessionStorage.setItem(worldKey,'{"kind":"world"}');sessionStorage.setItem(commandKey,'original request');
 let ui=await mountReact(vite,Directory,{workspace:w,props:directoryProps()});
 try{
  window.location.hash='#/studio';
  assert.equal(ui.host.querySelector('[aria-label="搜索资源名称"]').value,'未提交');
  assert.equal(ui.host.querySelector('[aria-label="已归档资源"]').getAttribute('aria-checked'),'true');
  await ui.input(ui.host.querySelector('[aria-label="搜索资源名称"]'),'更新后未提交');
  await ui.click(ui.host.querySelector('[aria-label="刷新创作资源"]'));
  assert.equal(w.reads.at(-1).q,'已提交');assert.equal(w.reads.at(-1).kind,'item');
  assert.equal(ui.host.querySelector('[aria-label="搜索资源名称"]').value,'更新后未提交');
  window.location.hash='#/studio/edit/resource';
 }finally{await ui.dispose();}
 assert.equal(JSON.parse(sessionStorage.getItem(key)).search,'更新后未提交');
 ui=await mountReact(vite,Directory,{workspace:w,props:directoryProps()});
 try{assert.equal(ui.host.querySelector('[aria-label="搜索资源名称"]').value,'更新后未提交');window.location.hash='#/library';}finally{await ui.dispose();}
 assert.equal(sessionStorage.getItem(key),null);assert.equal(sessionStorage.getItem(worldKey),'{"kind":"world"}');assert.equal(sessionStorage.getItem(commandKey),'original request');
 ui=await mountReact(vite,Directory,{workspace:w,props:directoryProps()});
 try{window.location.hash='#/studio';assert.equal(ui.host.querySelector('[aria-label="搜索资源名称"]').value,'');assert.equal(w.reads.at(-1).kind,undefined);assert.equal(w.reads.at(-1).archived,false);}finally{await ui.dispose();}
 ui=await mountReact(vite,Directory,{workspace:w,props:directoryProps({route:{path:'/studio',query:new URLSearchParams('q=明确查询&kind=npc&archived=true')}})});
 try{assert.equal(ui.host.querySelector('[aria-label="搜索资源名称"]').value,'明确查询');assert.equal(w.reads.at(-1).kind,'npc');assert.equal(w.reads.at(-1).archived,true);}finally{await ui.dispose();}
});

test('r20 Studio preference cleanup tolerates unavailable storage',async()=>{
 const w=workspace('r20-storage'),ui=await mountReact(vite,Directory,{workspace:w,props:directoryProps()}),descriptor=Object.getOwnPropertyDescriptor(globalThis,'sessionStorage');
 try{window.location.hash='#/library';Object.defineProperty(globalThis,'sessionStorage',{configurable:true,value:{removeItem(){throw Error('storage blocked');}}});await ui.dispose();}finally{Object.defineProperty(globalThis,'sessionStorage',descriptor);}
});

test('r20 Advanced media selection explains master switch without changing saved false, true or zero values',async()=>{
 const {default:Sections}=await vite.ssrLoadModule('/src/react-app/shared/SettingsSections.jsx');
 const fields=[{_key:'enabled',value:false},{_key:'media_enabled',value:true},{_key:'min_probability',value:0},{_key:'min_margin',value:0}],group={title:'高级',fields,valueSources:{kev:'stored'}},view={SettingsField:({field})=>String(field.value),sectionId:()=> 'advanced',catalogReady:{value:true},test(){}};
 const ui=await mountReact(vite,Sections,{props:{group,view}});
 try{assert.match(ui.host.textContent,/媒体方案补充的选择已保留，启用辅助决策后生效/);assert.deepEqual(fields.map(field=>field.value),[false,true,0,0]);fields[0].value=true;await ui.update({group,view});assert.doesNotMatch(ui.host.textContent,/启用辅助决策后生效/);fields[0].value=false;fields[1].value=false;await ui.update({group,view});assert.doesNotMatch(ui.host.textContent,/启用辅助决策后生效/);}finally{await ui.dispose();}
});
