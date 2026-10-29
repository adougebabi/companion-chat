import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {act,installDOM,mountReact} from './helpers/react-dom.mjs';
import {reviewCommand} from './helpers/command-sync.mjs';
import {configurationPoints} from '../src/v3/proto/components/configuration-fields.js';

const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const load=path=>vite.ssrLoadModule('/src/'+path);
const button=(root,text)=>[...root.querySelectorAll('button')].find(node=>node.textContent.trim()===text);
const clone=value=>structuredClone(value);
function workspace(name='r19-owner',catalog={effective_values:{},versions:{},endpoints:[],bindings:[]}){
 const state=proxy({session:{actorId:name},selectedId:'',characters:[],operations:[],errors:{},managementTab:'配置中心',managementSection:'',runtimeCatalog:{}}),moves=[];
 return {state,moves,current:{value:{id:''}},subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),navigatePath:path=>moves.push(path),notify(){},client:{listStudioResources:async()=>({items:[],next_cursor:''})},services:{studioClient:{},configurationClient:{read:async()=>clone(catalog),catalog:async()=>({agents:[]}),models:async()=>({models:[]})},diagnosticsClient:{page:async()=>({items:[],next_cursor:''})}}};
}
async function tab(ui,label){await act(async()=>[...ui.host.querySelectorAll('[role=tab]')].find(node=>node.textContent.trim()===label).dispatchEvent(new window.MouseEvent('mousedown',{bubbles:true,button:0})));await ui.flush();}

test('r19 real settings menu navigates to tasks and diagnostic URLs drop configuration-only state',async()=>{
 const {default:Management}=await load('react-app/pages/ManagementPage.jsx'),w=workspace('r19-menu');w.state.managementSection='高级';
 let ui=await mountReact(vite,Management,{workspace:w});
 try{await ui.click(button(ui.host.querySelector('nav'),'诊断与任务'));assert.equal(w.moves.at(-1),'/diag?view=tasks');}finally{await ui.dispose();}
 w.state.managementTab='运行诊断';ui=await mountReact(vite,Management,{workspace:w});
 try{assert.equal([...ui.host.querySelectorAll('[role=tab]')].find(n=>n.textContent.trim()==='任务中心').getAttribute('aria-selected'),'true');assert.ok(!ui.host.querySelector('.replica-diagnostic-context'));await act(async()=>{window.location.hash='#/diag?section=%E9%AB%98%E7%BA%A7&returnTo='+encodeURIComponent('#/library/create?resume=original');window.dispatchEvent(new window.HashChangeEvent('hashchange'));});await ui.flush();await tab(ui,'模型调用记录');const url=new URL('http://r19.test'+w.moves.at(-1));assert.equal(url.searchParams.get('section'),null);assert.equal(url.searchParams.get('returnTo'),'#/library/create?resume=original');assert.ok(button(ui.host,'返回初始化流程'));await ui.click(button(ui.host.querySelector('nav'),'模型服务与用途'));const settingsURL=new URL('http://r19.test'+w.moves.at(-1));assert.equal(settingsURL.pathname,'/settings');assert.equal(settingsURL.searchParams.get('section'),'服务地址');assert.equal(settingsURL.searchParams.get('returnTo'),'#/library/create?resume=original');}finally{await ui.dispose();}
});

test('r19 purpose cards keep empty values and allow manual model identifiers without per-role disclosures or steppers',async()=>{
 const {default:Management}=await load('react-app/pages/ManagementPage.jsx'),w=workspace('r19-roles');w.state.managementSection='模型用途';
 const ui=await mountReact(vite,Management,{workspace:w});
 try{const roles=[...ui.host.querySelectorAll('.replica-model-role')];assert.equal(roles.length,4);for(const role of roles){assert.ok(!role.querySelector('[data-slot=collapsible-trigger]'));assert.equal(role.querySelector('input[aria-label="模型标识"]').value,'');const timeout=role.querySelector('input[type=number]');assert.equal(timeout.value,'');assert.equal(timeout.closest('label').querySelectorAll('button').length,0);}assert.ok(!ui.host.querySelector('[data-testid=config-save-bar]'));await ui.input(roles[0].querySelector('input[aria-label="模型标识"]'),'custom-model-without-directory');assert.ok(ui.host.querySelector('[data-testid=config-save-bar]'));assert.equal(roles[0].querySelector('[aria-label="模型服务"]').textContent.includes('尚未配置'),true);assert.equal(w.moves.length,0);}finally{await ui.dispose();}
});

test('r19 runtime defaults remain truthful, unchanged values do not become edits and saving a threshold preserves defaults and extensions',async()=>{
 const {default:Management}=await load('react-app/pages/ManagementPage.jsx');
 const initial={enabled:false,media_enabled:true,endpoint:'http://127.0.0.1:8010/v1/systemone',model:'Jakevin/kev-4b-ternary-mlx',model_revision:'unknown',points:Object.fromEntries(configurationPoints.map(key=>[key,true])),timeout_ms:10000,budget_ms:10000,concurrency:1,batch_size:8,response_bytes:1048576,request_bytes:262144,strategy:'choice_argmax',min_probability:0,min_margin:0,max_deferrals:3,deferral_seconds:60,retention_days:7,custom_preserved:{keep:false}};
 const catalog={effective_values:{kev:initial},versions:{kev:0},endpoints:[],bindings:[],value_sources:{kev:'runtime_default'}},w=workspace('r19-default',catalog),writes=[];w.state.managementSection='高级';
 w.services.configurationClient.command=async()=>({status:'applied',applied_versions:{kev:1}});
 w.services.configurationClient.save=async body=>{writes.push(clone(body));Object.assign(catalog.effective_values,clone(body.values));catalog.versions.kev=1;catalog.value_sources.kev='stored';return {applied_versions:{kev:1}};};
 const ui=await mountReact(vite,Management,{workspace:w});
 try{assert.match(ui.host.textContent,/系统默认 · 尚未保存/);assert.ok(!ui.host.querySelector('[data-testid=config-save-bar]'));assert.equal(ui.host.querySelectorAll('#config-高级 [role=slider]').length,2);await ui.input(ui.host.querySelector('[aria-label="最低置信概率精确值"]'),'0.63');await ui.click(button(ui.host,'保存设置'));assert.equal(writes.length,1);assert.equal(writes[0].expectedVersions.kev,0);assert.equal(writes[0].values.kev.min_probability,0.63);assert.equal(writes[0].values.kev.endpoint,initial.endpoint);assert.equal(writes[0].values.kev.model,initial.model);assert.deepEqual(writes[0].values.kev.custom_preserved,{keep:false});assert.doesNotMatch(ui.host.textContent,/系统默认 · 尚未保存/);}finally{await ui.dispose();}
});

test('r19 world directory preferences cannot overwrite the Owner studio directory filter',async()=>{
 const {default:Directory}=await load('react-app/pages/StudioDirectory.jsx'),w=workspace('r19-directories'),key='fluctlight-studio:r19-directories:directory-ui';sessionStorage.setItem(key,JSON.stringify({kind:'item',query:'',search:'',archived:false}));
 let ui=await mountReact(vite,Directory,{workspace:w,props:{kind:'world',route:{path:'/worlds',query:new URLSearchParams()}}});
 try{assert.match(ui.host.querySelector('h1').textContent,/^世界$/);assert.equal(JSON.parse(sessionStorage.getItem(key)).kind,'item');}finally{await ui.dispose();}
 ui=await mountReact(vite,Directory,{workspace:w,props:{route:{path:'/studio',query:new URLSearchParams()}}});
 try{assert.match(ui.host.querySelector('[aria-label="资源类型"]').textContent,/物品/);assert.equal(JSON.parse(sessionStorage.getItem(key)).kind,'item');}finally{await ui.dispose();}
});

test('r19 synchronization is secondary normally, explicit for a conflict and absent during an unknown submission',async()=>{
 const {default:Sync}=await load('react-app/shared/CommandSyncAction.tsx');let reviews=0;
 const ui=await mountReact(vite,Sync,{props:{state:{busy:false},onReview:()=>{reviews++;}}});
 try{assert.equal(button(ui.host,'同步资料，保留输入'),undefined);await reviewCommand(ui,ui.host);assert.equal(reviews,1);await ui.update({state:{busy:false,needsRebase:true},placement:'recovery',onReview:()=>{reviews++;}});await ui.click(button(ui.host,'同步资料，保留输入'));assert.equal(reviews,2);await ui.update({state:{busy:false,pending:{rejection:null}},placement:'recovery',onReview:()=>{reviews++;}});assert.equal(button(ui.host,'同步资料，保留输入'),undefined);assert.ok(!ui.host.querySelector('[aria-label="更多表单操作"]'));assert.equal(reviews,2);}finally{await ui.dispose();}
});

test('r19 preview renders wardrobe enums, booleans and relation metrics in Chinese without rewriting the source',async()=>{
 const {default:Tree}=await load('react-app/shared/SemanticTree.jsx'),value={wardrobe_items:[{description:'棉质衬衫',category:'upper',ownership:'owned',available:true,seasons:['spring','autumn'],customWardrobe:{flag:false,count:0}}],initial_relationships:[{role:'朋友',metrics:{familiarity:0.1,trust:0.15}}],custom:{flag:false,count:0,text:'true'}},before=clone(value);
 const ui=await mountReact(vite,Tree,{props:{value}});
 try{assert.match(ui.host.textContent,/上装/);assert.match(ui.host.textContent,/已拥有/);assert.match(ui.host.textContent,/可用/);assert.match(ui.host.textContent,/春/);assert.match(ui.host.textContent,/秋/);assert.match(ui.host.textContent,/熟悉程度/);assert.match(ui.host.textContent,/10%/);assert.match(ui.host.textContent,/信任/);assert.match(ui.host.textContent,/15%/);assert.match(ui.host.textContent,/否/);assert.match(ui.host.textContent,/true/);assert.doesNotMatch(ui.host.textContent,/"familiarity"|\bowned\b|\bupper\b|undefined%/);assert.deepEqual(value,before);}finally{await ui.dispose();}
});
