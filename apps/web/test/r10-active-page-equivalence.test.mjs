import test from 'node:test';
import assert from 'node:assert/strict';
import {h,defineComponent} from 'vue';
import {setupSFC,memoryStorage,tick} from './support/sfc-harness.mjs';
import {creationFixture,validAnalysis} from './support/creation-fixture.mjs';
import {migrateCreationRecovery,creationHistoryKey} from '../src/approved-ui/creation-recovery.mjs';
import {ActionButton,Surface} from '../src/approved-ui/shared/primitives.mjs';
import {creationDraft} from '../src/v3/creation-draft.mjs';
import {errorMessage} from '../src/v3/domain-presentation.mjs';

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
test('active Vue library renders the durable continuation and a role whose group has not loaded',async()=>{
 const saved=memoryStorage(),session=memoryStorage(),recovery=creationDraft({owner:'owner',storage:saved,timezone:'Asia/Shanghai'});recovery.save({...recovery.read(),name:'小林',wizardStep:2});
 const role={id:'role',groupId:'unloaded-group',name:'晚晴',status:'正常',tags:[],job:'',bio:''};
 const w={state:{session:{actorId:'owner'},characters:[role]},services:{configurationClient:{read:async()=>({bindings:[]})}},select(){},navigate(){},navigatePath(){}};
 const view=setupSFC(new URL('../src/approved-ui/pages/LibraryPage.vue',import.meta.url),{useWorkspace:()=>w,localStorage:saved,sessionStorage:session,migrateCreationRecovery,creationHistoryKey},['w','search','filters','configured','saved','error','characters','groups','rows','progressHref','details','choose','subtitle']);
 await view.mount();await tick();assert.deepEqual(view.state.rows.value,[role]);assert.equal(view.state.saved.value.name,'小林');
 const html=await view.html({ActionButton,Surface,FilterChips:defineComponent(()=>()=>h('div')),CharacterAvatar:defineComponent(()=>()=>h('span')),Gu:defineComponent(()=>()=>h('svg'))});assert.match(html,/继续上次的相识|继续创建/);assert.match(html,/继续创建与查看进度/);assert.match(html,/晚晴/);view.unmount();
});
