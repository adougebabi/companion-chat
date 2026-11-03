import test from 'node:test';
import assert from 'node:assert/strict';
import {fileURLToPath} from 'node:url';
import {createServer} from 'vite';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {installDOM,mountReact} from './helpers/react-dom.mjs';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const load=path=>vite.ssrLoadModule('/src/'+path);
const button=(root,text)=>[...root.querySelectorAll('button')].find(n=>n.textContent.trim()===text);
const pack={schema_version:1,resources:[{kind:'world',name:'日常世界',definition:{notes:'保留原文'}}],groups:[],preferences:{}};

test('r22 local backups create persistent Owner-scoped copies and fill an import draft without business writes',async()=>{
 const {default:Panel}=await load('react-app/shared/LocalBackupsPanel.tsx');let reads=0,imported;
 const props={owner:'r22-backup-owner',exportPack:async()=>{reads++;return {pack:structuredClone(pack)};},onImport:value=>{imported=value;}};
 localStorage.removeItem('fluctlight-local-backups:'+props.owner);
 let ui=await mountReact(vite,Panel,{props});
 try{assert.match(ui.host.textContent,/0 份本地备份/);assert.ok(ui.host.querySelector('[data-empty-variant=page] .lucide-leaf'));assert.equal(reads,0);await ui.click(button(ui.host,'创建备份'));assert.match(ui.host.textContent,/1 份本地备份/);assert.equal(reads,1);assert.deepEqual(JSON.parse(localStorage.getItem('fluctlight-local-backups:'+props.owner))[0].pack,pack);await ui.click(button(ui.host,'填入导入草稿'));assert.deepEqual(imported,pack);assert.match(ui.host.textContent,/先校验再确认导入/);await ui.dispose();ui=await mountReact(vite,Panel,{props});assert.match(ui.host.textContent,/1 份本地备份/);assert.equal(reads,1);await ui.update({...props,owner:'r22-other-owner'});assert.match(ui.host.textContent,/0 份本地备份/);assert.equal(JSON.parse(localStorage.getItem('fluctlight-local-backups:'+props.owner)).length,1);}
 finally{await ui.dispose();localStorage.removeItem('fluctlight-local-backups:'+props.owner);}
});

test('r22 backup failures preserve existing copies and unread storage cannot be overwritten',async()=>{
 const {default:Panel}=await load('react-app/shared/LocalBackupsPanel.tsx'),owner='r22-backup-failure',key='fluctlight-local-backups:'+owner;
 const original=[{id:'original',name:'原有备份',createdAt:'2026-10-30T10:00:00Z',pack}];let fail=false;
 const storage={getItem:()=>JSON.stringify(original),setItem(){throw Error('quota exceeded');}};
 const ui=await mountReact(vite,Panel,{props:{owner,storage,exportPack:async()=>{if(fail)throw Error('offline');return {pack};},onImport(){}}});
 try{await ui.click(button(ui.host,'创建备份'));assert.match(ui.host.querySelector('[role=alert]').textContent,/已有备份保持原样/);assert.match(ui.host.textContent,/1 份本地备份/);fail=true;await ui.click(button(ui.host,'创建备份'));assert.match(ui.host.textContent,/原有备份/);await ui.update({owner,storage:{getItem:()=>'{broken',setItem(){throw Error('must not write');}},exportPack:async()=>({pack}),onImport(){}});assert.equal(button(ui.host,'创建备份').disabled,true);assert.match(ui.host.textContent,/原记录已保留/);assert.equal(localStorage.getItem(key),null);}
 finally{await ui.dispose();}
});

test('r22 deleting a local backup requires confirmation and cancellation preserves the copy',async()=>{
 const {default:Panel}=await load('react-app/shared/LocalBackupsPanel.tsx'),saved=new Map(),storage={getItem:key=>saved.get(key)||null,setItem:(key,value)=>saved.set(key,value)};
 const ui=await mountReact(vite,Panel,{props:{owner:'r22-delete-backup',storage,exportPack:async()=>({pack}),onImport(){}}});
 try{await ui.click(button(ui.host,'创建备份'));const remove=()=>ui.host.querySelector('button[aria-label^="删除设定备份"]');await ui.click(remove());let dialog=document.querySelector('[role=alertdialog]');assert.match(dialog.textContent,/只删除所选的浏览器副本/);await ui.click(button(dialog,'取消'));assert.match(ui.host.textContent,/1 份本地备份/);await ui.click(remove());dialog=document.querySelector('[role=alertdialog]');await ui.click(button(dialog,'删除备份'));assert.match(ui.host.textContent,/0 份本地备份/);assert.deepEqual(JSON.parse([...saved.values()][0]),[]);}
 finally{await ui.dispose();}
});

test('r22 backup result from a previous Owner is discarded after switching Owner',async()=>{
 const {default:Panel}=await load('react-app/shared/LocalBackupsPanel.tsx');let finish;
 const storage=new Map(),adapter={getItem:key=>storage.get(key)||null,setItem:(key,value)=>storage.set(key,value)};
 const props={owner:'r22-backup-late',storage:adapter,exportPack:()=>new Promise(resolve=>{finish=resolve;}),onImport(){}};
 const ui=await mountReact(vite,Panel,{props});
 try{await ui.click(button(ui.host,'创建备份'));await ui.update({...props,owner:'r22-backup-new'});finish({pack});await ui.flush();assert.equal(storage.size,0);assert.match(ui.host.textContent,/0 份本地备份/);}
 finally{await ui.dispose();}
});

test('r22 returning to an Owner cannot admit an export from its previous lifecycle',async()=>{
 const {default:Panel}=await load('react-app/shared/LocalBackupsPanel.tsx');let finish;
 const storage=new Map(),adapter={getItem:key=>storage.get(key)||null,setItem:(key,value)=>storage.set(key,value)};
 const props={owner:'r22-backup-return',storage:adapter,exportPack:()=>new Promise(resolve=>{finish=resolve;}),onImport(){}};
 const ui=await mountReact(vite,Panel,{props});
 try{await ui.click(button(ui.host,'创建备份'));await ui.update({...props,owner:'r22-backup-away'});await ui.update(props);finish({pack});await ui.flush();assert.equal(storage.size,0);assert.match(ui.host.textContent,/0 份本地备份/);assert.equal(button(ui.host,'创建备份').disabled,false);}
 finally{await ui.dispose();}
});

test('r22 blocked browser storage reports an error without crashing the backup panel',async()=>{
 const {default:Panel}=await load('react-app/shared/LocalBackupsPanel.tsx');
 const props={owner:'r22-backup-storage-denied',exportPack:async()=>({pack}),onImport(){}};
 const ui=await mountReact(vite,Panel,{props:{...props,storage:{getItem:()=>null}}});
 const descriptor=Object.getOwnPropertyDescriptor(globalThis,'localStorage');
 try{Object.defineProperty(globalThis,'localStorage',{configurable:true,get(){throw new DOMException('Access denied','SecurityError');}});await ui.update(props);assert.match(ui.host.querySelector('[role=alert]').textContent,/本地备份未能读取/);assert.equal(button(ui.host,'创建备份').disabled,true);assert.match(ui.host.textContent,/本地备份数量未读到/);}
 finally{Object.defineProperty(globalThis,'localStorage',descriptor);await ui.dispose();}
});

test('r22 production data route mounts shadcn local backups and restores into the existing validation draft',async()=>{
 const {default:Page}=await load('react-app/pages/DomainPage.tsx'),{renderRealData}=await load('v3-domain/data-tools.js'),{configureDomainService}=await load('v3/proto/services.js'),store=await load('v3/proto/store.js');
 const owner='r22-data-route',state=proxy({session:{actorId:owner},selectedId:'',characters:[]});let reads=0;
 localStorage.removeItem('fluctlight-local-backups:'+owner);store.replaceSnapshot({...store.get(),session:{actorId:owner},characters:{},groups:{}});
 configureDomainService('dataTools',{backups:async()=>({items:[],next_cursor:''}),exportPack:async()=>{reads++;return {pack};}});
 const w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),navigatePath(){}},route={path:'/data',query:new URLSearchParams(),feature:{title:'数据管理',params:{},render:'renderRealData',load:async()=>renderRealData}};
 const ui=await mountReact(vite,Page,{workspace:w,props:{route}});
 try{await ui.flush();const panel=ui.host.querySelector('[aria-label="本地备份"]');assert.ok(panel);assert.equal(button(panel,'创建备份').getAttribute('data-slot'),'button');await ui.click(button(panel,'创建备份'));assert.equal(reads,1);await ui.click(button(panel,'填入导入草稿'));assert.deepEqual(JSON.parse(ui.host.querySelector('[data-input]').value),pack);const visibleInput=ui.host.querySelector('[data-slot="textarea"]');assert.deepEqual(JSON.parse(visibleInput.value),pack);assert.equal(document.activeElement,visibleInput);assert.equal(ui.host.querySelector('[data-import]'),null);assert.match(ui.host.textContent,/业务数据备份/);}
 finally{await ui.dispose();localStorage.removeItem('fluctlight-local-backups:'+owner);}
});

test('r22 advanced settings render authoritative guarded defaults and retain saved zero and media choices',async()=>{
 const {default:Management}=await load('react-app/pages/ManagementPage.jsx');
 const kev={enabled:false,media_enabled:false,endpoint:'http://127.0.0.1:8010/v1/systemone',strategy:'choice_guarded',min_probability:0.65,min_margin:0.15},catalog={effective_values:{kev},versions:{kev:0},value_sources:{kev:'runtime_default'},endpoints:[],bindings:[]};
 const state=proxy({session:{actorId:'r22-kev-defaults'},selectedId:'',characters:[],operations:[],managementTab:'配置中心',managementSection:'高级',runtimeCatalog:{}}),w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),notify(){},navigatePath(){},services:{configurationClient:{read:async()=>structuredClone(catalog),catalog:async()=>({agents:[]}),models:async()=>({models:[]})},diagnosticsClient:{page:async()=>({items:[],next_cursor:''})}}};
 let ui=await mountReact(vite,Management,{workspace:w});
 try{const panel=ui.host.querySelector('.settings-kev-card');assert.equal(panel.querySelector('[aria-label="最低置信概率精确值"]').value,'0.65');assert.equal(panel.querySelector('[aria-label="领先候选最小差值精确值"]').value,'0.15');assert.equal(panel.querySelector('[aria-label="用于媒体决策"]').getAttribute('aria-checked'),'false');assert.match(panel.textContent,/带置信门槛的最优选择/);assert.match(panel.textContent,/将评分扩展到图片生成相关的候选行动/);assert.equal(ui.host.querySelector('[data-testid=config-save]').disabled,true);await ui.dispose();Object.assign(kev,{media_enabled:true,strategy:'choice_argmax',min_probability:0,min_margin:0});catalog.value_sources.kev='stored';catalog.versions.kev=1;ui=await mountReact(vite,Management,{workspace:w});assert.equal(ui.host.querySelector('[aria-label="最低置信概率精确值"]').value,'0');assert.equal(ui.host.querySelector('[aria-label="用于媒体决策"]').getAttribute('aria-checked'),'true');}
 finally{await ui.dispose();}
});
