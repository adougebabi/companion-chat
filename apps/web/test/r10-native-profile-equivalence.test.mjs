import test from 'node:test';
import assert from 'node:assert/strict';
import {nativeProfileFixture,profileDetail} from './support/native-profile-fixture.mjs';
test('native profile sends both revisions and one idempotent switch then confirms actual current state',async()=>{
 let changed=false,sent;const f=await nativeProfileFixture({read:async()=>profileDetail({active:changed?'focused':'warm',revision:changed?3:2}),switchPersona:async(fid,body)=>{assert.equal(fid,'fixture-role');sent=body;changed=true;return {status:'completed',active_profile_id:'focused'};}});
 try{await f.read();f.draft();await f.c.switchPersona();assert.equal(sent.expectedRevision,4);assert.equal(sent.expectedRuntimeRevision,2);assert.equal(sent.targetPersonaId,'focused');assert.equal(sent.confirm,true);assert.ok(sent.idempotencyKey);assert.match(f.c.personaFeedback.value,/已生效/);assert.equal(f.c.snapshot.value.activeProfileId,'focused');assert.match(await f.html(),/专注人格/);}finally{await f.dispose();}
});
test('native profile keeps the same key, target and reason through unresolved version conflict',async()=>{
 const keys=[];const f=await nativeProfileFixture({switchPersona:async(_fid,body)=>{keys.push(body.idempotencyKey);throw Object.assign(Error('conflict'),{status:409});}});
 try{await f.read();f.draft();await f.c.switchPersona();await f.c.switchPersona();assert.equal(keys.length,2);assert.equal(keys[0],keys[1]);assert.equal(f.c.reason.value,'专注完成当前工作');assert.equal(f.c.target.value,'focused');assert.match(f.c.error.value,/版本已变化/);assert.doesNotMatch(f.c.personaFeedback.value,/已生效/);assert.match(await f.html(),/版本已变化/);}finally{await f.dispose();}
});
test('native profile retains accepted switch input on readback failure and recovers by reading without a new switch',async()=>{
 let reads=0,writes=0,recovered=false;const f=await nativeProfileFixture({read:async()=>{if(reads++&& !recovered)throw Error('network');return profileDetail({active:recovered?'focused':'warm'});},switchPersona:async()=>{writes++;return {status:'completed',active_profile_id:'focused'};}});
 try{await f.read();f.draft();await f.c.switchPersona();assert.equal(writes,1);assert.equal(f.c.personaReadbackPending.value,true);assert.equal(f.c.target.value,'focused');assert.equal(f.c.reason.value,'专注完成当前工作');assert.match(f.c.error.value,/命令已提交.*读取失败/);assert.match(await f.html(),/重新读取人格切换结果/);await f.c.switchPersona();assert.equal(writes,1);recovered=true;await f.c.readPersonaOutcome();assert.equal(writes,1);assert.equal(f.c.personaReadbackPending.value,false);assert.equal(f.c.snapshot.value.activeProfileId,'focused');}finally{await f.dispose();}
});
test('native profile discards a read after unmount and exposes a retry after an initial failure',async()=>{
 let finish;const f=await nativeProfileFixture({read:()=>new Promise(resolve=>finish=resolve)});const pending=f.read();await f.dispose();finish(profileDetail({name:'过时角色'}));await pending;assert.equal(f.c.detail.value,null);
 const next=await nativeProfileFixture({read:async()=>{throw Error('initial failure');}});try{assert.equal(await next.read(),false);assert.match(await next.html(),/重新读取/);}finally{await next.dispose();}
});
test('native profile hides retired switches and rejects direct writes while active stays editable',async()=>{
 for(const status of ['retired','archived','active']){let writes=0;const f=await nativeProfileFixture({read:async()=>profileDetail({status}),switchPersona:async()=>{writes++;return {status:'completed',active_profile_id:'focused'};}});try{await f.read();f.draft();assert.equal(f.c.snapshot.value.readOnly,status!=='active');const html=await f.html();if(status!=='active'){assert.doesNotMatch(html,/确认切换/);assert.match(html,/data-testid="detail-edit"[^>]*disabled/);f.c.edit();assert.equal(f.c.editing.value,false);await f.c.switchPersona();assert.equal(writes,0);}else{assert.match(html,/确认切换/);assert.doesNotMatch(html,/data-testid="detail-edit"[^>]*disabled/);f.c.edit();assert.equal(f.c.editing.value,true);}}finally{await f.dispose();}}
});
