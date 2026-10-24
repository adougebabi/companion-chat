import test from 'node:test';
import assert from 'node:assert/strict';
import {createRenderer,reactive,ref,nextTick,h,provide,createSSRApp} from 'vue';
import {renderToString} from 'vue/server-renderer';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {configurationPlan,weekdays} from '../src/approved-ui/configuration-plan.mjs';
import {wardrobePreparationStage} from '../src/approved-ui/wardrobe-preparation.mjs';
import {readApprovedCreationProgress} from '../src/approved-ui/creation-progress.mjs';
import {notificationGroups,notificationIssueGroups,notificationPath} from '../src/approved-ui/presentation.mjs';
import {personaPresentation,semanticEntries,portraitPresentation,workingPersonaPresentation} from '../src/approved-ui/profile-presentation.mjs';
import {profileFields,profileChanges} from '../src/approved-ui/profile-fields.mjs';
import {creationFeedback} from '../src/v3/creation-feedback.mjs';
import {errorMessage} from '../src/v3/domain-presentation.mjs';

test('optional model timeout uses 120 seconds while malformed explicit values stay invalid',()=>{
 const input={catalog:{versions:{},effective_values:{}},settings:[],endpoints:[{id:'e',name:'e',url:'https://model.example/v1',protocol:'openai-compatible'}],bindings:[{role:'generic_llm',name:'通用模型',endpoint:'e',model:'m',timeout:''}],quiet:{enabled:false,start:'',end:'',days:[...weekdays],behavior:'暂停主动联系',character:'全部角色'},characters:[]};
 assert.equal(configurationPlan(input).find(w=>w.operation==='binding').body.timeoutSeconds,120);
 for(const timeout of [0,-1,1.5,'not-a-number']){input.bindings[0].timeout=timeout;assert.throws(()=>configurationPlan(input),/超时/);}
});
test('exhausted wardrobe authority is shared by both pages and blocks successors visibly',async()=>{
 for(const status of ['failed','generating','queued'])assert.equal(wardrobePreparationStage({status,can_recover:true}).state,'failed');
 assert.equal(wardrobePreparationStage({status:'awaiting_wear'}).state,'running');
 const progress=await readApprovedCreationProgress({
 visualIdentityManagement:async()=>({fluctlight_id:'A',session:{status:'queued'}}),displayAvatar:async()=>({fluctlight_id:'A',avatar:null}),visualIdentityTimeline:async()=>({items:[]}),
 inventoryPreparation:async()=>({status:'generating',can_recover:true,task_id:'original',result:{failure_reason:'自动重试已经结束'}}),schedules:async()=>({days:[]}),lifecycleDiagnostics:async()=>({workflowIntents:[]}),world:async()=>({world:{id:'W'}}),detail:async()=>({persona_preparation:{status:'ready'}})
 },{activatedId:'A',worldRuntimeId:'W',groupConfirmed:true,activationDate:'2026-10-24',mode:'blank'});
 assert.equal(progress.blocker.key,'wardrobe');assert.equal(progress.complete,false);
 assert.equal(progress.stages[2].retryable,true);assert.match(progress.stages[2].href,/original/);
 for(const stage of progress.stages.slice(3)){assert.equal(stage.state,'blocked');assert.equal(stage.blockedBy,'wardrobe');assert.match(stage.note,/初始衣柜/);}
});
test('notice grouping preserves actual IDs, hash destinations and original failure records',()=>{
 const rows=[{id:'one',fluctlight_id:'A',link:'#/c/A/chat?conversation=C&message=M'},{id:'two',fluctlight_id:'A',link:'#/feed/moment'},{id:'three',fluctlight_id:'A',link:'#/c/A/chat?conversation=C&message=M'}];
 const groups=notificationGroups(rows,[{id:'A',name:'角色'}]);assert.equal(groups.length,1);assert.equal(groups[0].notices.length,3);
 assert.equal(notificationPath(rows[1].link),'/feed/moment');assert.equal(notificationPath(rows[0].link),'/c/A/chat?conversation=C&message=M');
 for(const link of ['//foreign','https://foreign','#//foreign','/\\foreign'])assert.equal(notificationPath(link),null);
 const issues=notificationIssueGroups([1,2,3].map(n=>({id:'attempt'+n,label:'角色 · 人格整理',code:'invalid_json',fid:'A',reason:'失败'})));
 assert.equal(issues.length,1);assert.equal(issues[0].items.length,3);
});
test('profile projection removes exact aliases while retaining conflicting and custom facts',()=>{
 assert.deepEqual(semanticEntries({agreeableness:.5,traits:{agreeableness:.5},custom:'保留'}),[['traits',{agreeableness:.5}],['custom','保留']]);
 assert.equal(semanticEntries({agreeableness:.2,traits:{agreeableness:.5}}).length,2);
 const shared={personality:{agreeableness:.5},identity:{name:'角色'}},profile={id:'p',name:'人格',...shared,personality:{agreeableness:.5,curiosity:.8},custom:'自定义'};
 assert.deepEqual(personaPresentation(profile,shared),{id:'p',name:'人格',personality:{curiosity:.8},custom:'自定义'});
 assert.equal(profile.personality.agreeableness,.5);
 assert.deepEqual(workingPersonaPresentation({status:'ready',working_persona:{profile_id:'p',working_persona:{profile_id:'p',portrait_text:'原画像'}}}),{status:'ready',profile_id:'p',portrait_text:'原画像'});
 assert.deepEqual(workingPersonaPresentation({status:'ready',working_persona:{status:'conflicting',portrait_text:'原画像'}}),{status:'ready',working_persona:{status:'conflicting',portrait_text:'原画像'}});
 const lines=portraitPresentation('角色\nagreeableness: 0.5\nagreeableness: 0.5\n原文里提到 agreeableness: 0.5');
 assert.equal(lines.length,3);assert.deepEqual(lines[1],{field:'agreeableness',value:.5});assert.match(lines[2].text,/原文/);
 const base={identity:{},behavioral_policy:{message_length:'short'},life_profile:{}};
 const fields=profileFields(base),length=fields.find(f=>f.path.at(-1)==='message_length');
 assert.equal(length.kind,'select');assert.equal(length.options.find(o=>o.value==='short').label,'简短');assert.deepEqual(profileChanges(base,fields),{});
 length.value='long';assert.deepEqual(profileChanges(base,fields),{behavioral_policy:{message_length:'long'}});
 const unknown=profileFields({...base,behavioral_policy:{message_length:'custom'}}).find(f=>f.path.at(-1)==='message_length');assert.ok(unknown.options.some(o=>o.value==='custom'));
});
test('model output errors do not blame user input and missing embedding explains saved memory',()=>{
 const feedback=creationFeedback({code:'initialization_response_invalid_json',details:{validation_error:{path:'provider_response'}}});
 assert.doesNotMatch(feedback.message,/修正原输入|问题字段/);assert.match(feedback.message,/原描述已保留/);
 const semantic=creationFeedback({code:'initialization_persona_invalid',details:{validation_error:{path:'initial_intentions[0].goal_index'}}});
 assert.doesNotMatch(semantic.message,/修正原输入/);assert.match(semantic.message,/模型返回.*字段校验/);assert.equal(semantic.path,'initial_intentions[0].goal_index');
 assert.match(errorMessage({code:'memory_embedding_unconfigured'}),/正文已保存.*向量索引未完成/);
});

function hostRenderer(){
 const create=type=>({type,tagName:type.toUpperCase(),children:[],props:{},parent:null,text:'',value:'',addEventListener(){},removeEventListener(){},querySelectorAll(){return []},getAttribute(k){return this.props[k]},setAttribute(k,v){this.props[k]=v},removeAttribute(k){delete this.props[k]},get options(){return this.children.filter(n=>n.type==='option')},get multiple(){return !!this.props.multiple}});
 const host={createElement:create,createText:text=>({...create('#text'),text}),createComment:text=>({...create('#comment'),text}),setText:(n,t)=>n.text=t,setElementText:(n,t)=>{n.text=t;n.children=[]},parentNode:n=>n.parent,nextSibling:n=>n.parent?.children[n.parent.children.indexOf(n)+1],patchProp:(n,k,old,v)=>{n.props[k]=v;if(k==='value'){n._value=v;n.value=v}},insert(n,p,anchor){if(n.parent)n.parent.children.splice(n.parent.children.indexOf(n),1);n.parent=p;const i=anchor?p.children.indexOf(anchor):-1;if(i<0)p.children.push(n);else p.children.splice(i,0,n)},remove(n){n.parent?.children.splice(n.parent.children.indexOf(n),1)}};
 const root=create('root'),renderer=createRenderer(host),all=n=>[n,...n.children.flatMap(all)],text=n=>n.text+n.children.map(text).join('');
 return {root,renderer,all:()=>all(root),text};
}

test('real Management page stays busy through command readback and keeps a local saved receipt',async()=>{
 const previous={document:globalThis.document,window:globalThis.window,sessionStorage:globalThis.sessionStorage,localStorage:globalThis.localStorage};
 const values=new Map(),storage={getItem:k=>values.get(k)??null,setItem:(k,v)=>values.set(k,v),removeItem:k=>values.delete(k)};
 globalThis.document=Object.assign(new EventTarget(),{hidden:false,getElementById:()=>null});globalThis.window={location:{origin:'http://test.local'},addEventListener(){},removeEventListener(){}};globalThis.sessionStorage=storage;globalThis.localStorage=storage;
 const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
 let mounted;
 try{
  const {WorkspaceKey}=await vite.ssrLoadModule('/src/approved-ui/workspace.mjs'),{default:Management}=await vite.ssrLoadModule('/src/approved-ui/pages/ManagementPage.mjs');
  // The Vite SSR loader supplies SSR-only SFCs. This client-renderer test owns
  // Management's real native form and save lifecycle; isolate decorative SFCs.
  for(const path of ['/src/approved-ui/shared/ModelDirectory.vue','/src/approved-ui/shared/SettingsField.vue']){const {default:component}=await vite.ssrLoadModule(path);component.setup=()=>()=>h('span');}
  let finishWrite,finishRead,calls=0;
  const catalog={endpoints:[{id:'service',base_url:'https://model.example',kind:'openai-compatible'}],bindings:[{role:'generic_llm',endpoint_id:'service',model_id:'old',timeout_seconds:120}],versions:{},effective_values:{}};
  const workspace={state:reactive({session:{actorId:'owner'},managementTab:'配置中心',characters:[],operations:[],managementSection:'',runtimeCatalog:{}}),services:{configurationClient:{read:async()=>structuredClone(catalog),models:async()=>({models:[]}),catalog:async()=>({agents:[]}),binding:async()=>{calls++;return new Promise(resolve=>finishWrite=resolve)},command:async()=>new Promise(resolve=>finishRead=resolve)},diagnosticsClient:{page:async()=>({items:[]})}},notify(){},navigatePath(){}};
  const view=hostRenderer();mounted=view.renderer.createApp({setup(){provide(WorkspaceKey,workspace);return()=>h(Management)}});mounted.mount(view.root);
  const flush=async()=>{for(let i=0;i<5;i++){await new Promise(resolve=>setImmediate(resolve));await nextTick()}};await flush();
  const model=view.all().find(n=>n.type==='input'&&n.props.list==='models-generic_llm');assert.ok(model);model.props['onUpdate:modelValue']('new');
  const form=view.all().find(n=>n.type==='form');const submitted=form.props.onSubmit({preventDefault(){}});await flush();
  const button=()=>view.all().find(n=>n.props['data-testid']==='config-save'),status=()=>view.all().find(n=>n.props['data-testid']==='config-save-status');
  assert.equal(calls,1);assert.equal(button().props.disabled,true);assert.match(view.text(button()),/保存中/);assert.match(view.text(status()),/等待服务端确认/);
  model.props['onUpdate:modelValue']('newer-unsaved');finishWrite({});await flush();assert.equal(button().props.disabled,true);assert.ok(finishRead);
  finishRead({status:'applied',applied_versions:{'binding:generic_llm':1}});await submitted;await flush();assert.equal(button().props.disabled,false);assert.match(view.text(status()),/已保存/);assert.equal(model.value,'newer-unsaved');
 }finally{mounted?.unmount();await vite.close();for(const [k,v]of Object.entries(previous)){if(v===undefined)delete globalThis[k];else globalThis[k]=v}}
});

test('actual profile and notice components render Chinese traits and collapsed groups',async()=>{
 const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
 try{
  const {default:Semantic}=await vite.ssrLoadModule('/src/approved-ui/shared/SemanticTree.vue'),{default:Notices}=await vite.ssrLoadModule('/src/approved-ui/shared/NotificationGroup.vue');
  const html=await renderToString(createSSRApp({render:()=>h(Semantic,{field:'portrait_text',value:'agreeableness: 0.5\nagreeableness: 0.5\n自定义经历保留'})}));
  assert.doesNotMatch(html,/agreeableness|0\.5/);assert.equal((html.match(/亲和程度适中/g)||[]).length,1);assert.match(html,/自定义经历保留/);
  const notice=await renderToString(createSSRApp({render:()=>h(Notices,{group:{name:'角色',notices:[{id:'reply',body:'回复'},{id:'moment',body:'动态'}]}})}));
  assert.match(notice,/2 条未读/);assert.doesNotMatch(notice,/<details[^>]*\bopen\b/);assert.match(notice,/回复/);assert.match(notice,/动态/);
 }finally{await vite.close()}
});
