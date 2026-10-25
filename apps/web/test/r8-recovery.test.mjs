import test from 'node:test';
import assert from 'node:assert/strict';
import {createRenderer,createSSRApp,h,provide,nextTick} from 'vue';
import {renderToString} from 'vue/server-renderer';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {profileFields,profileChanges} from '../src/approved-ui/profile-fields.mjs';
import {readApprovedCreationProgress} from '../src/approved-ui/creation-progress.mjs';
import {chatFailureText,chatWaitingPresentation} from '../src/v3/chat-feedback.mjs';
import {ProfileGroups} from '../src/v3/proto/components/profile.js';

function hostRenderer(){
 const create=type=>({type,tagName:type.toUpperCase(),children:[],props:{},parent:null,text:'',value:'',focus(){},addEventListener(){},removeEventListener(){},querySelectorAll(){return []},getAttribute(k){return this.props[k]},setAttribute(k,v){this.props[k]=v},removeAttribute(k){delete this.props[k]},get options(){return this.children.filter(n=>n.type==='option')},get multiple(){return !!this.props.multiple}});
 const host={createElement:create,createText:text=>({...create('#text'),text}),createComment:text=>({...create('#comment'),text}),setText:(n,t)=>n.text=t,setElementText:(n,t)=>{n.text=t;n.children=[]},parentNode:n=>n.parent,nextSibling:n=>n.parent?.children[n.parent.children.indexOf(n)+1],patchProp:(n,k,old,v)=>{n.props[k]=v;if(k==='value'){n._value=v;n.value=v}},insert(n,p,anchor){if(n.parent)n.parent.children.splice(n.parent.children.indexOf(n),1);n.parent=p;const i=anchor?p.children.indexOf(anchor):-1;if(i<0)p.children.push(n);else p.children.splice(i,0,n)},remove(n){n.parent?.children.splice(n.parent.children.indexOf(n),1)}};
 const root=create('root');host.querySelector=()=>root;
 const renderer=createRenderer(host),all=n=>[n,...n.children.flatMap(all)],text=n=>n.text+n.children.map(text).join('');
 return {root,renderer,all:()=>all(root),text};
}
const flush=async()=>{for(let i=0;i<4;i++){await new Promise(done=>setImmediate(done));await nextTick()}};

test('actual world category save persists through reload and world edits keep their identity',async()=>{
 const keys=['window','document','location','sessionStorage','localStorage'],previous=Object.fromEntries(keys.map(k=>[k,globalThis[k]])),saved=new Map();
 const storage={getItem:k=>saved.get(k)??null,setItem:(k,v)=>saved.set(k,String(v)),removeItem:k=>saved.delete(k)};
 globalThis.window={location:{origin:'http://r8.test'},addEventListener(){},removeEventListener(){}};globalThis.document=Object.assign(new EventTarget(),{hidden:false});globalThis.location={hash:''};globalThis.localStorage=storage;globalThis.sessionStorage=storage;
 const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
 let workspace,mounted;const originals={};
 try {
  const {createWorkspace,WorkspaceKey}=await vite.ssrLoadModule('/src/approved-ui/workspace.mjs'),store=await vite.ssrLoadModule('/src/v3/proto/store.js'),services=await vite.ssrLoadModule('/src/v3/proto/services.js'),{default:World}=await vite.ssrLoadModule('/src/approved-ui/pages/WorldPage.mjs');
  let server={id:'W',name:'原世界',revision:1,flavor:'日常',active:true,venueCategories:[]},writes=0;
  services.configureDomainService('world',{worlds:async()=>[structuredClone(server)],places:async()=>[]});
  workspace=createWorkspace();store.replaceSnapshot({...store.get(),session:{actorId:'owner'},characters:{A:{id:'A',name:'周屿'}},ui:{charId:'A'}});
  await workspace.loadWorlds();
  originals.update=workspace.client.updateCatalogWorld;originals.create=workspace.client.createCatalogWorld;
  workspace.client.createCatalogWorld=async()=>{throw Error('existing world incorrectly recreated')};
  workspace.client.updateCatalogWorld=async(fid,id,body)=>{assert.equal(fid,'A');assert.equal(id,'W');assert.equal(body.expectedRevision,server.revision);writes++;server={...server,name:body.name,revision:server.revision+1,...(body.venueCategories?{venueCategories:structuredClone(body.venueCategories)}:{})};return {id:'W',revision:server.revision}};
  const view=hostRenderer();mounted=view.renderer.createApp({setup(){provide(WorkspaceKey,workspace);return()=>h(World)}});mounted.mount(view.root);
  const button=text=>view.all().find(n=>n.type==='button'&&view.text(n)===text);
  button('进入').props.onClick();await flush();button('新增场地分类').props.onClick();await flush();
  const input=view.all().find(n=>n.type==='input'&&n.props['onUpdate:modelValue']&&n.parent?.type==='label');input.props['onUpdate:modelValue']('商店');
  await view.all().find(n=>n.type==='form').props.onSubmit({preventDefault(){}});await flush();
  assert.equal(writes,1);assert.equal(server.venueCategories.length,1);assert.equal(server.venueCategories[0].name,'商店');assert.match(view.text(view.root),/场地 · 商店/);
  workspace.state.worlds=[];await workspace.loadWorlds();await flush();assert.match(view.text(view.root),/场地 · 商店/);
  button('编辑世界信息').props.onClick();await flush();
  const name=view.all().find(n=>n.props['data-testid']==='world-name');name.props['onUpdate:modelValue']('更新世界');
  await view.all().find(n=>n.type==='form').props.onSubmit({preventDefault(){}});await flush();assert.equal(writes,2);assert.equal(server.name,'更新世界');assert.equal(server.venueCategories.length,1);
 } finally {mounted?.unmount();if(workspace){workspace.client.updateCatalogWorld=originals.update;workspace.client.createCatalogWorld=originals.create;workspace.dispose()}await vite.close();for(const [k,v]of Object.entries(previous)){if(v===undefined)delete globalThis[k];else globalThis[k]=v}}
});

test('male unknown body fields stay hidden while explicit stored values remain editable and readable',async()=>{
 const core={identity:{name:'周屿',gender:'male'},behavioral_policy:{},life_profile:{appearance:{physical_features:{chest_cup:null,hair_color:'黑色'}}}};
 const fields=profileFields(core);assert.ok(fields.some(f=>f.path.at(-1)==='hair_color'));assert.ok(!fields.some(f=>f.path.at(-1)==='chest_cup'));assert.deepEqual(profileChanges(core,fields),{});assert.equal(core.life_profile.appearance.physical_features.chest_cup,null);
 const legacy=ProfileGroups({groups:[{key:'identity',value:core.identity},{key:'current',value:{appearance:{body_fields:{chest_cup:{status:'unknown',value:null}}}}}]});
 assert.doesNotMatch(legacy.split('<details class="card profile-technical">')[0],/胸部尺寸/);
 const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
 try{
  const {default:Tree}=await vite.ssrLoadModule('/src/approved-ui/shared/SemanticTree.vue');
  const render=value=>renderToString(createSSRApp({render:()=>h(Tree,{value,identity:core.identity})}));
  let html=await render({body_fields:{chest_cup:{status:'unknown',value:null},hair_color:{status:'known',value:'黑色'}}});assert.doesNotMatch(html,/胸部尺寸/);assert.match(html,/黑色/);
  core.life_profile.appearance.physical_features.chest_cup='用户原有记录';assert.ok(profileFields(core).some(f=>f.path.at(-1)==='chest_cup'));
  html=await render({chest_cup:'用户原有记录'});assert.match(html,/用户原有记录/);
 }finally{await vite.close()}
});

test('initial schedule precedes image stages and image failure does not relabel it blocked',async()=>{
 const progress=await readApprovedCreationProgress({visualIdentityManagement:async()=>({session:{status:'failed',error_code:'image_unconfigured'}}),displayAvatar:async()=>({avatar:null}),visualIdentityTimeline:async()=>({items:[]}),inventoryPreparation:async()=>({status:'completed'}),schedules:async()=>({days:[]}),lifecycleDiagnostics:async()=>({workflowIntents:[{intentType:'schedule.current_day',status:'pending',intentId:'original-schedule'}]}),world:async()=>({world:{id:'W'}}),detail:async()=>({persona_preparation:{status:'ready'}})},{activatedId:'A',worldRuntimeId:'W',groupConfirmed:true,activationDate:'2026-10-25',mode:'blank'});
 assert.deepEqual(progress.stages.map(s=>s.key),['analysis','world','wardrobe','schedule','reference','sheet','avatar']);assert.equal(progress.stages.find(s=>s.key==='schedule').state,'pending');
 for(const code of ['agent_partial_committed','request_timeout','unavailable','unknown'])assert.doesNotMatch(chatFailureText(code),/她/);
 assert.equal(chatWaitingPresentation({sending:true},0,40000,'周屿').detail,'周屿还在想…');
});
