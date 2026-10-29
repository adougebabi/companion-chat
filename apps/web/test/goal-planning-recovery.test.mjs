import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {installDOM} from './helpers/react-dom.mjs';
import {goalPlanningFixture} from './fixtures/goal-planning-fixture.mjs';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const {renderRealGoalPlanning}=await vite.ssrLoadModule('/src/v3-domain/goal-planning.js'),{configureDomainService}=await vite.ssrLoadModule('/src/v3/proto/services.js'),{_debug}=await vite.ssrLoadModule('/src/v3/proto/store.js');
_debug.data.session={actorId:'planning-owner'};
test.after(async()=>{await vite.close();env.restore();});
const tick=async()=>{for(let n=0;n<5;n++)await new Promise(r=>setTimeout(r,0));};
async function setup(scenario='normal'){
 localStorage.clear();sessionStorage.clear();const fixture=goalPlanningFixture({scenario}),host=document.createElement('div');document.body.append(host);
 async function http(resource,method='GET',body){let value,status;await fixture.handle({path:'/api/fluctlights/A/'+resource,method,body,res:{},json:(_,v,s=200)=>{value=v;status=s;}});if(status>=400)throw Object.assign(Error(value.message),{status});return value;}
 configureDomainService('life',{goalSet:()=>http('goal-set'),planningHistory:()=>http('goal-planning'),updateGoalSet:(_,base,changes,reason,key)=>http('goal-set','PUT',{...changes,reason,idempotencyKey:key||'legacy-'+fixture.calls.length,expectedVersion:base.revision,expectedFactsRevision:base.facts_revision}),plan:(_,key)=>http('goal-planning','POST',{idempotencyKey:key})});
 let dispose=renderRealGoalPlanning(host,{charId:'A'});await tick();const form=()=>host.querySelector('[data-policy]'),submit=async()=>{form().dispatchEvent(new window.Event('submit',{bubbles:true,cancelable:true}));await tick();};
 const input=(name,value)=>{const node=form().elements[name];node.value=value;node.dispatchEvent(new window.Event('input',{bubbles:true}));};
 return {host,fixture,form,input,submit,http,remount:async()=>{dispose();dispose=renderRealGoalPlanning(host,{charId:'A'});await tick();},close:()=>{dispose();host.remove();}};
}
test('manual policy sends only active goal order, preserves candidate rows and saves advanced/dependency values',async()=>{const v=await setup();try{
 v.input('orderingMode','manual');v.input('max','4');v.input('reason','保留高级策略与依赖');v.input('recoveryPolicy',JSON.stringify({merge_window_seconds:8,retry_backoff_seconds:45,max_attempts:4,lease_seconds:180,candidate_limit:12}));v.form().elements.order.value='active-two\ncandidate-one\nactive-one\ncandidate-two';v.form().elements.dependencies.value=JSON.stringify({'active-two':['active-one']});await v.submit();
 const put=v.fixture.calls.find(c=>c.method==='PUT');assert.deepEqual(put.body.order,['active-two','active-one']);assert.equal(v.fixture.state('A').max_active_goals,4);assert.equal(v.fixture.state('A').max_attempts,4);assert.equal(v.fixture.state('A').dependencies[0].prerequisite_id,'active-one');assert.deepEqual(v.fixture.state('A').extension,{keep:['opaque']});await v.remount();assert.equal(v.form().elements.max.value,'4');assert.match(v.form().elements.recoveryPolicy.value,/180/);
 }finally{v.close();}});
test('accepted write with failed readback recovers with GET only after remount and keeps later draft',async()=>{const v=await setup('readback-failure');try{
 v.input('max','4');v.input('reason','原保存');await v.submit();assert.match(v.host.textContent,/本次提交已保存，最新状态读取失败/);assert.equal(v.fixture.calls.filter(c=>c.method==='PUT').length,1);v.input('reason','后来草稿');await v.submit();assert.equal(v.fixture.calls.filter(c=>c.method==='PUT').length,1);await v.remount();v.host.querySelector('[data-command-recover]').click();await tick();assert.equal(v.fixture.calls.filter(c=>c.method==='PUT').length,1);assert.equal(v.form().elements.reason.value,'后来草稿');assert.equal(v.fixture.state('A').revision,2);
 }finally{v.close();}});
test('unknown result replays original body and key while later draft stays separate',async()=>{const v=await setup('lost-response');try{
 v.input('max','4');v.input('reason','原保存');await v.submit();v.input('max','3');v.input('reason','后来修改');v.host.querySelector('[data-command-recover]').click();await tick();const puts=v.fixture.calls.filter(c=>c.method==='PUT');assert.equal(puts.length,2);assert.deepEqual(puts[0].body,puts[1].body);assert.equal(v.fixture.state('A').revision,2);assert.equal(v.form().elements.max.value,'3');assert.equal(v.form().elements.reason.value,'后来修改');
 }finally{v.close();}});
test('first CAS rejection preserves the stale draft and does not silently adopt server baseline',async()=>{const v=await setup('conflict');try{
 v.input('max','4');v.input('reason','冲突草稿');await v.submit();await v.remount();assert.ok(v.host.querySelector('[data-policy-rebase]'));assert.equal(v.form().elements.max.value,'4');await v.submit();assert.equal(v.fixture.calls.filter(c=>c.method==='PUT').length,1);assert.match(v.host.textContent,/请先核对并采用最新策略版本/);
 }finally{v.close();}});
test('fixture CAS, facts, candidate decisions and source reviews are atomic and survive readback',async()=>{const f=goalPlanningFixture();async function req(body){let out,status;await f.handle({path:'/api/fluctlights/A/goal-set',method:'PUT',body,res:{},json:(_,v,s=200)=>{out=v;status=s;}});return {out,status};}
 const base={expectedVersion:1,expectedFactsRevision:'facts-planning-1',idempotencyKey:'decision',reason:'确认候选'};
 assert.equal((await req({...base,expectedFactsRevision:'old'})).status,409);assert.equal(f.state('A').revision,1);
 const accepted=await req({...base,changes:[{id:'candidate-one',operation:'resume',expected_revision:1}]});assert.equal(accepted.status,200);assert.equal(f.state('A').goals.find(g=>g.id==='candidate-one').status,'active');
 const review=await req({...base,expectedVersion:2,idempotencyKey:'review',sourceReviews:[{source_id:'source-one',expected_revision:1,operation:'dismiss'}],reviewedGoalIds:['active-one']});assert.equal(review.status,200);assert.deepEqual(review.out.source_reviews,[]);assert.equal(f.state('A').source_reviews[0].raw_source.extension,true);assert.equal(f.state('A').goals[0].context_review_required,false);
 const rejected=await req({...base,expectedVersion:3,idempotencyKey:'reject',changes:[{id:'candidate-two',operation:'cancel',expected_revision:1}]});assert.equal(rejected.status,200);assert.ok(!rejected.out.goals.some(g=>g.id==='candidate-two'));
});
test('life provider preserves explicitly supplied command identities at the BrowserClient boundary',async()=>{
 const {createLifeProvider}=await vite.ssrLoadModule('/src/v3/providers/life.ts');_debug.data.characters.A={id:'A',status:'active'};const calls=[],life=createLifeProvider({updateGoalSet:async(fid,body)=>{calls.push({fid,body});return {};},requestGoalPlanning:async(fid,key)=>{calls.push({fid,key});return {};}});
 await life.updateGoalSet('A',{revision:3,facts_revision:'facts-3'},{maxActiveGoals:4},'原修改原因','original-set-key');await life.plan('A','original-plan-key');assert.deepEqual(calls,[{fid:'A',body:{maxActiveGoals:4,expectedVersion:3,expectedFactsRevision:'facts-3',reason:'原修改原因',idempotencyKey:'original-set-key'}},{fid:'A',key:'original-plan-key'}]);
});
test('accepted planning request keeps pending history recovery read-only and never claims completed',async()=>{
 const v=await setup();let writes=0,reads=0;configureDomainService('life',{goalSet:()=>v.http('goal-set'),planningHistory:async()=>{reads++;if(writes&&reads===1)throw Error('history unavailable');return [{status:'pending'}];},plan:async(_,key)=>{writes++;return {status:'requested',idempotency_key:key};}});
 try{v.host.querySelector('[data-plan]').click();await tick();assert.equal(writes,1);assert.match(v.host.textContent,/规划请求已受理，任务历史读取失败/);await v.remount();v.host.querySelector('[data-command-recover]').click();await tick();assert.equal(writes,1);assert.match(v.host.textContent,/已受理不代表规划完成/);}finally{v.close();}
});
test('newer policy edits made during accepted readback survive the acknowledgement',async()=>{
 const v=await setup();let finish,hold=true;configureDomainService('life',{goalSet:async()=>{if(hold)await new Promise(r=>finish=r);return v.http('goal-set');},planningHistory:()=>v.http('goal-planning'),updateGoalSet:(_,base,changes,reason,key)=>v.http('goal-set','PUT',{...changes,reason,idempotencyKey:key,expectedVersion:base.revision,expectedFactsRevision:base.facts_revision})});
 try{v.input('reason','原策略');v.input('max','4');v.form().dispatchEvent(new window.Event('submit',{bubbles:true,cancelable:true}));await tick();v.input('reason','等待回读期间的原因');v.input('max','3');hold=false;finish();await tick();assert.equal(v.form().elements.reason.value,'等待回读期间的原因');assert.equal(v.form().elements.max.value,'3');assert.equal(v.fixture.state('A').max_active_goals,4);assert.equal(v.fixture.calls.filter(c=>c.method==='PUT').length,1);}finally{v.close();}
});

for(const mismatch of ['older-than-receipt','same-revision-fields'])test('accepted readback rejects '+mismatch+' and recovers by GET only',async()=>{
 const v=await setup();let stale=true,writes=0;
 configureDomainService('life',{goalSet:async()=>{const latest=await v.http('goal-set');if(stale){if(mismatch==='older-than-receipt')latest.revision=2;else latest.max_active_goals=5;}return latest;},planningHistory:()=>v.http('goal-planning'),updateGoalSet:async(_,base,changes,reason,key)=>{writes++;const receipt=await v.http('goal-set','PUT',{...changes,reason,idempotencyKey:key,expectedVersion:base.revision,expectedFactsRevision:base.facts_revision});if(mismatch==='older-than-receipt'){receipt.revision=4;v.fixture.state('A').revision=4;}return receipt;}});
 try{v.input('max','4');v.input('reason','回执下界');await v.submit();assert.equal(writes,1);assert.match(v.host.textContent,/本次提交已保存，最新状态读取失败/);const pending=JSON.parse(sessionStorage.getItem('fluctlight-studio:planning-owner:goal-planning:A')).pending;assert.equal(pending.receiptRevision,mismatch==='older-than-receipt'?4:2);assert.equal(v.host.querySelector('[data-command-recover]').textContent,'只读恢复已保存提交结果');stale=false;v.host.querySelector('[data-command-recover]').click();await tick();assert.equal(writes,1);assert.equal(JSON.parse(sessionStorage.getItem('fluctlight-studio:planning-owner:goal-planning:A')).pending,undefined);}finally{v.close();}
});

for(const command of ['policy','plan'])test('late '+command+' acknowledgement preserves reopened editor and original accepted identity',async()=>{
 const v=await setup();let release,writes=0;
 configureDomainService('life',{goalSet:()=>v.http('goal-set'),planningHistory:()=>v.http('goal-planning'),updateGoalSet:async(_,base,changes,reason,key)=>{writes++;const receipt=await v.http('goal-set','PUT',{...changes,reason,idempotencyKey:key,expectedVersion:base.revision,expectedFactsRevision:base.facts_revision});await new Promise(r=>release=r);return receipt;},plan:async(_,key)=>{writes++;await new Promise(r=>release=r);return {idempotency_key:key,status:'requested'};}});
 try{v.input('reason','旧页面');if(command==='policy')v.form().dispatchEvent(new window.Event('submit',{bubbles:true,cancelable:true}));else v.host.querySelector('[data-plan]').click();await tick();await v.remount();v.input('reason','重新打开后的新草稿');v.input('max','3');release();await tick();const saved=JSON.parse(sessionStorage.getItem('fluctlight-studio:planning-owner:goal-planning:A'));assert.equal(saved.policy.reason,'重新打开后的新草稿');assert.equal(saved.policy.max,'3');assert.equal(saved[command==='policy'?'pending':'planPending'].accepted,true);assert.equal(v.form().elements.reason.value,'重新打开后的新草稿');v.host.querySelector('[data-command-recover]').click();await tick();assert.equal(writes,1);assert.equal(v.form().elements.reason.value,'重新打开后的新草稿');assert.equal(v.form().elements.max.value,'3');}finally{v.close();}
});
