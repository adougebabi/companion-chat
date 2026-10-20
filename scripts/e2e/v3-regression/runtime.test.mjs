import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,readFile,writeFile,stat,readdir} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {spawn} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import {RegressionContext,RegressionError,optionsFrom,redact,hash,assertGoResults,importCookies,runBoundedProcess} from './runtime.mjs';
import {readStream,modelRunFamily,visualGenerationComplete,visualGenerationAsset,diaryIsolationRows} from './domain.mjs';
import {runnerArguments,caseOutcome} from './cli.mjs';
import {cases} from './cases.mjs';
import {assertPromiseReschedule,assertLivePromiseFulfillment} from './promise-proof.mjs';
import {verifyNPCWorldIsolation} from './npc-isolation.mjs';

test('Diary isolation ignores scheduler pending placeholders and keeps written rows',()=>{
 const pending={id:'diary-pending',revision:1,status:'pending',body:null};
 const ready={id:'diary-ready',revision:2,status:'ready',body:'今天'};
 assert.deepEqual(diaryIsolationRows([pending,ready,{id:'diary-inactive',revision:2,status:'inactive'}]),[{id:'diary-ready',revision:2},{id:'diary-inactive',revision:2}]);
});
test('Promise reschedule acceptance verifies both authorities instead of any terminal result',()=>{
 const current={goal_id:'g',intention_id:'i',commitment_id:'c',window_start:'2026-10-19T12:00:00Z',window_end:'2026-10-19T13:00:00Z'},end='2026-10-19T14:00:00Z';
 const after={...current,window_end:end,commitment:{window_end:end},intention:{expiration:end}};
 const done={status:'completed',mutation_committed:true,effects:[{capability_name:'intention.decide',output:{status:'rescheduled',commitment_id:'c',intention_id:'i'}}]};
 assert.doesNotThrow(()=>assertPromiseReschedule(current,after,done,end));
 assert.throws(()=>assertPromiseReschedule(current,after,{status:'failed'},end));
 assert.throws(()=>assertPromiseReschedule(current,after,{...done,mutation_committed:false},end));
 assert.throws(()=>assertPromiseReschedule(current,{...after,intention:{expiration:current.window_end}},done,end));
 assert.doesNotThrow(()=>assertPromiseReschedule(current,current,{status:'rejected'},end));
 assert.throws(()=>assertPromiseReschedule(current,after,{status:'rejected'},end));
});
test('Live Promise fulfillment requires an actual accepted message source and Commitment evaluation',()=>{
 const p={request_id:'p',goal_id:'g',commitment_id:'c',lifecycle:'fulfilled',fulfillment_authority:'goal_commitment_evaluation',commitment:{status:'completed'},evaluations:[{id:'e',commitment_id:'c',impact:'completed',evidence_refs:['source:7']}]};
 const evidence=[{status:'confirmed',source:{id:7,source_kind:'message',source_id:'actual-reply'}}],messages=[{id:'actual-reply',kind:'assistant',text:'我喜欢阅读科幻作品。'}];
 assert.equal(assertLivePromiseFulfillment(p,evidence,messages).evaluationId,'e');
 assert.throws(()=>assertLivePromiseFulfillment({...p,lifecycle:'accepted',intention:{status:'completed'}},evidence,messages));
 assert.throws(()=>assertLivePromiseFulfillment({...p,evaluations:[]},evidence,messages));
 assert.throws(()=>assertLivePromiseFulfillment(p,[{...evidence[0],status:'candidate'}],messages));
 assert.throws(()=>assertLivePromiseFulfillment(p,evidence,[{...messages[0],id:'other-reply'}]));
});

async function context(fetcher,{live=false,...extra}={}){
 const runDir=await mkdtemp(join(tmpdir(),'fluctlight-regression-harness-'));
 const ctx=new RegressionContext('L00','Controlled harness fixture',{live,preflight:false,unit:true,runDir,timeoutMs:30,requestTimeoutMs:30,pollMs:1},{FLUCTLIGHT_E2E_ISOLATED:'1',FLUCTLIGHT_E2E_PASSWORD:'private-test-password',...extra},fetcher);await ctx.init();return ctx;
}
async function npcIsolationContext({persistSelection=true,sharedActor=false,copiedEncounter=false,missingSource=false}={}){
 const calls=[],source={id:'npc-a',world_id:'world-a',actor_id:'actor-a',source_resource_id:'definition',source_version:'1'};
 if(missingSource)delete source.source_version;
 let binding=null,created=0;
 const t=await context(async(url,init)=>{
  const path=new URL(url).pathname,method=init.method,body=init.body?JSON.parse(init.body):null;calls.push({path,method,body});
  if(path==='/api/fluctlights/b/detail')return Response.json({id:'b',relationships:[],memories:[]});
  if(path==='/api/fluctlights/a/npcs/npc-a')return Response.json(source);
  if(path==='/api/fluctlights/b/world/catalog'&&method==='POST'){created++;return Response.json({id:'world-b',fluctlight_id:'b'});}
  if(path==='/api/fluctlights/b/world/active'&&method==='PUT'){assert.equal(body.worldId,'world-b');if(persistSelection)binding={world_id:'world-b',revision:1};return Response.json({world_id:'world-b',revision:1});}
  if(path==='/api/fluctlights/a/world/catalog')return Response.json({active_binding:{world_id:'world-a',revision:1}});
  if(path==='/api/fluctlights/b/world/catalog')return Response.json({active_binding:binding});
  if(path==='/api/fluctlights/a/world')return Response.json({world:{id:'world-a',fluctlight_id:'a'}});
  if(path==='/api/fluctlights/b/world')return Response.json({world:binding?{id:'world-b',fluctlight_id:'b'}:null});
  if(path==='/api/studio/resources/definition')return Response.json({id:'definition',published_version:'2'});
  if(path==='/api/studio/resources/definition/apply-preview'){assert.equal(body.resourceVersion,'1');return Response.json({available:true,diff:[],instance_revision:0,runtime_revision:0});}
  if(path==='/api/studio/resources/definition/apply'){assert.equal(body.fluctlightId,'b');assert.equal(Object.hasOwn(body,'worldId'),false);assert.equal(body.resourceVersion,'1');assert.ok(binding);return Response.json({id:'npc-b',applied:true});}
  if(path==='/api/fluctlights/b/npcs/npc-b')return Response.json({...source,id:'npc-b',world_id:'world-b',actor_id:sharedActor?'actor-a':'actor-b'});
  if(path==='/api/fluctlights/b/npcs/npc-b/encounters')return Response.json({items:copiedEncounter?[{id:'primary-encounter'}]:[]});
  if(path==='/api/fluctlights/b/npcs/npc-a'||path==='/api/fluctlights/a/npcs/npc-b'){assert.ok(binding,'Scope reads must follow persisted world selection');return Response.json({code:'not_found'},{status:404});}
  throw new Error('Unexpected NPC isolation request '+method+' '+path);
 },{live:true});
 await t.remember('fixture:secondary',{id:'b'});
 return {t,calls,created:()=>created};
}
test('NPC isolation initializes the blank secondary world and verifies bindings before cross reads',async()=>{
 const {t,calls,created}=await npcIsolationContext();
 const evidence=await verifyNPCWorldIsolation(t,{fid:'a',worldId:'world-a',npcId:'npc-a'});
 assert.equal(evidence.secondary.worldId,'world-b');assert.equal(evidence.secondary.actorId,'actor-b');assert.equal(evidence.resourceVersion,'1');
 const selected=calls.findIndex(c=>c.method==='PUT'&&c.path.endsWith('/world/active')),cross=calls.findIndex(c=>c.path==='/api/fluctlights/b/npcs/npc-a');
 assert.ok(selected>=0&&cross>selected);assert.equal(t.modelOperations,0);
 assert.deepEqual(await verifyNPCWorldIsolation(t,{fid:'a',worldId:'world-a',npcId:'npc-a'}),evidence);assert.equal(created(),1,'Resume must reuse the saved secondary world');
});
test('NPC isolation cannot mistake an unpersisted selection for cross-world isolation',async()=>{
 const {t,calls}=await npcIsolationContext({persistSelection:false});
 await assert.rejects(verifyNPCWorldIsolation(t,{fid:'a',worldId:'world-a',npcId:'npc-a'}),/World selection must read back/);
 assert.ok(!calls.some(c=>c.path==='/api/fluctlights/b/npcs/npc-a'));
});
test('NPC isolation rejects shared Actor identity, copied encounters and missing definition provenance',async()=>{
 for(const options of [{sharedActor:true},{copiedEncounter:true},{missingSource:true}]){
  const {t,calls}=await npcIsolationContext(options);
  await assert.rejects(verifyNPCWorldIsolation(t,{fid:'a',worldId:'world-a',npcId:'npc-a'}),RegressionError);
  assert.ok(!calls.some(c=>c.path==='/api/fluctlights/b/npcs/npc-a'));
 }
});
const frame=(type,sequence,extra={})=>({type,sequence,turnId:'turn-original',payload:{},...extra});
const stream=bytes=>new Response(new ReadableStream({start(controller){for(const chunk of bytes)controller.enqueue(chunk);controller.close();}}),{headers:{'content-type':'application/x-ndjson'}});
test('preflight treats the actual anonymous 401 as login and retains its CSRF cookie',async()=>{
 let authenticated=false;const calls=[];
 const t=await context(async(url,init)=>{const path=new URL(url).pathname;calls.push(path);
  if(path==='/health/live')return Response.json({status:'ok'});
  if(path==='/health/ready')return Response.json({status:'ready'});
  if(path==='/auth/login'){assert.equal(init.headers.get('X-CSRF-Token'),'anonymous-csrf');authenticated=true;return Response.json({authenticated:true,actorId:'actual-owner'},{headers:{'Set-Cookie':'fluctlight_session=authenticated-session; Path=/'}});}
  if(path==='/auth/session')return authenticated?Response.json({authenticated:true,actorId:'actual-owner'}):Response.json({authenticated:false},{status:401,headers:{'Set-Cookie':'fluctlight_csrf=anonymous-csrf; Path=/'}});
  throw new Error('unexpected preflight request '+path);
 });
 assert.equal((await t.preflight()).authenticated,true);assert.equal(t.modelOperations,0);assert.equal(calls.filter(p=>p==='/auth/login').length,1);
});
test('preflight imports a supplied private cookie before its first authenticated check',async()=>{
 const folder=await mkdtemp(join(tmpdir(),'v3-cookie-preflight-')),path=join(folder,'cookies.txt');
 await writeFile(path,'127.0.0.1\tFALSE\t/\tFALSE\t0\tfluctlight_session\tknown-session\n127.0.0.1\tFALSE\t/\tFALSE\t0\tfluctlight_csrf\tknown-csrf\n',{mode:0o600});
 let authChecks=0;
 const t=await context(async(url,init)=>{const p=new URL(url).pathname;if(p==='/health/live')return Response.json({status:'ok'});if(p==='/health/ready')return Response.json({status:'ready'});assert.equal(p,'/auth/session');authChecks++;assert.match(init.headers.get('Cookie'),/fluctlight_session=known-session/);return Response.json({authenticated:true,actorId:'actual-owner'});},{FLUCTLIGHT_E2E_COOKIE_FILE:path,FLUCTLIGHT_E2E_PASSWORD:''});
 assert.equal((await t.preflight()).authenticated,true);assert.equal(authChecks,1);assert.equal(t.modelOperations,0);
});
test('multipart replay preserves real bytes, filename, CSRF and original identity without a JSON content type',async()=>{
 const calls=[];let fail=true;const t=await context(async(url,init)=>{calls.push({key:init.body.get('idempotencyKey'),name:init.body.get('file').name,bytes:new Uint8Array(await init.body.get('file').arrayBuffer()),headers:init.headers});if(fail)throw new Error('response lost');return Response.json({asset_id:'original'},{status:201});});
 t.cookies.set('fluctlight_csrf','actual-csrf');t.cookies.set('fluctlight_session','private-session');const bytes=new Uint8Array([137,80,78,71]);
 await assert.rejects(t.upload('original',{filename:'actual.png',mime:'image/png',bytes}));fail=false;await t.upload('original',{filename:'replacement.png',mime:'image/jpeg',bytes:new Uint8Array([1])});
 assert.equal(calls.length,2);assert.deepEqual(calls[0].bytes,bytes);assert.deepEqual(calls[1].bytes,bytes);assert.equal(calls[1].name,'actual.png');assert.equal(calls[0].key,calls[1].key);assert.equal(calls[0].headers.get('Content-Type'),null);assert.equal(calls[0].headers.get('X-CSRF-Token'),'actual-csrf');assert.match(calls[0].headers.get('Cookie'),/fluctlight_session=private-session/);
});
test('live execution is opt-in and runner never drops reordered flags',()=>{
 assert.equal(optionsFrom([],{}).live,false);assert.equal(optionsFrom(['--live'],{}).live,true);
 assert.deepEqual(runnerArguments(['--live','--all','--run-dir','private']),{id:null,forwarded:['--live','--run-dir','private']});
 assert.deepEqual(runnerArguments(['--case','l02','--live']),{id:'L02',forwarded:['--live']});
 assert.throws(()=>runnerArguments(['--case']),/requires/);assert.throws(()=>optionsFrom(['--timeout-ms','0'],{}),/timeoutMs/);
});
test('redaction drops credential fields and hashes the actual byte representation',()=>{
 const value=redact({password:'never-print',nested:{Cookie:'session',prompt:'raw private prompt'},safe:'postgresql://user:pass@host/db Bearer private-token'});
 assert.equal(value.password,'[redacted]');assert.equal(value.nested.Cookie,'[redacted]');assert.equal(value.nested.prompt,'[redacted]');assert.doesNotMatch(value.safe,/user:pass|private-token/);
 assert.equal(hash(new Uint8Array([97,98,99])),'ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad');
});
test('NDJSON handles split UTF-8 and enforces source, sequence and one terminal',async()=>{
 const bytes=new TextEncoder().encode(JSON.stringify(frame('token',1,{payload:{text:'你好'}}))+'\n'+JSON.stringify(frame('completed',2))+'\n');
 const parsed=await readStream(stream([bytes.slice(0,30),bytes.slice(30,65),bytes.slice(65)]));assert.equal(parsed.events[0].payload.text,'你好');assert.equal(parsed.terminal.type,'completed');
 for(const events of [[frame('token',1)],[frame('completed',1),frame('error',2)],[frame('token',2),frame('completed',1)],[frame('token',1),frame('completed',2,{turnId:'wrong-turn'})]])await assert.rejects(readStream(stream([new TextEncoder().encode(events.map(JSON.stringify).join('\n'))])),RegressionError);
});
test('unknown HTTP outcome keeps immutable original payload/key on retry',async()=>{
 const calls=[];let fail=true;const t=await context(async(url,init)=>{calls.push({url:String(url),body:JSON.parse(init.body)});if(fail)throw new Error('connection dropped');return Response.json({id:'persisted'});});
 await assert.rejects(t.command('create','POST','/api/studio/resources',()=>({name:'original',idempotencyKey:'same-key'})),/authoritative/);
 fail=false;const result=await t.command('create','POST','/api/studio/resources',()=>({name:'newer draft',idempotencyKey:'new-key'}));assert.equal(result.value.id,'persisted');assert.deepEqual(calls[0],calls[1]);
 const mode=(await stat(t.statePath)).mode&0o777;assert.equal(mode,0o600);
});
test('non-idempotent analysis cannot replay an unknown response or rerun a confirmed stage',async()=>{
 let calls=0,fail=true;const t=await context(async()=>{calls++;if(fail)throw new Error('lost analysis response');return Response.json({analysis_id:'actual-analysis'});});
 await assert.rejects(t.command('analysis','POST','/api/fluctlight-creations/analysis',{description:'original'},{replay:'read-result'}));
 fail=false;await assert.rejects(t.command('analysis','POST','/api/fluctlight-creations/analysis',{description:'new'},{replay:'read-result'}),e=>e.code==='NONREPLAYABLE_OUTCOME_UNKNOWN');assert.equal(calls,1);
 const first=await t.command('another-analysis','POST','/api/fluctlight-creations/analysis',{description:'explicit separate analysis'},{replay:'read-result'}),second=await t.command('another-analysis','POST','/api/fluctlight-creations/analysis',{description:'different draft'},{replay:'read-result'});assert.deepEqual(first,second);assert.equal(calls,2);
});
test('cyclic pagination and failed/timed-out jobs remain explicit errors',async()=>{
 const t=await context(async()=>Response.json({items:[{id:'actual'}],next_cursor:'repeated'}));await assert.rejects(t.pages('/api/studio/resources'),/progress/);
 await assert.rejects(t.until('failed product',async()=>({status:'failed',error_code:'provider_timeout'}),()=>false),e=>e.code==='provider_timeout');
 await assert.rejects(t.until('still queued',async()=>({status:'queued'}),()=>false,{timeoutMs:2}),e=>e.code==='TASK_TIMEOUT'&&e.details.latest.status==='queued');
});
test('isolated/model authorization gates run before any business or model request',async()=>{
 let calls=0;const t=await context(async()=>{calls++;return Response.json({});},{FLUCTLIGHT_E2E_ISOLATED:'0'});assert.throws(()=>t.model('test'),e=>e.code==='LIVE_EXECUTION_DISABLED');
 await assert.rejects(t.request('/api/studio/jobs',{method:'POST',body:{}}),e=>e.code==='ISOLATED_ENVIRONMENT_REQUIRED');assert.equal(calls,0);
});
test('missing deployment is blocked and does not become a successful empty scenario',async()=>{
 let calls=0;const t=await context(async()=>{calls++;throw new Error('connection refused');});
 await assert.rejects(t.preflight(),e=>e.status==='blocked'&&e.code==='ENVIRONMENT_UNAVAILABLE');assert.equal(calls,1);
});
test('report preserves failed expected/actual and redacts secrets from nested errors/log values',async()=>{
 const t=await context(async()=>Response.json({}));t.cookies.set('session','private-session-cookie');
 await t.step('intentional-failure','The actual value must be one',()=>{throw new RegressionError('ACTUAL_MISMATCH','private-test-password',{expected:1,actual:2,password:'raw',diagnostic:'private-session-cookie'});});
 assert.equal(await t.finish(),1);const text=await readFile(join(t.runDir,'L00.json'),'utf8'),report=JSON.parse(text);assert.equal(report.steps[0].error.details.actual,2);assert.equal(report.steps[0].error.details.expected,1);assert.doesNotMatch(text,/private-test-password|private-session-cookie|"password": "raw"/);
});
test('Go compile/no-tests/skip results can never certify executed integration coverage',()=>{
 assert.throws(()=>assertGoResults('ok package [no tests to run]'),e=>e.code==='TESTS_SKIPPED');
 assert.throws(()=>assertGoResults(JSON.stringify({Action:'skip',Test:'TestActualDatabase'})),e=>e.code==='TESTS_SKIPPED');
 assert.deepEqual(assertGoResults(JSON.stringify({Action:'pass',Test:'TestActualDatabase'})),{passed:['TestActualDatabase']});
 assert.throws(()=>assertGoResults(JSON.stringify({Action:'pass',Test:'TestActualDatabase'}),['TestMissing']),e=>e.code==='TEST_SELECTION_INCOMPLETE');
});
test('unexpected child exit or stale previous report cannot become success in the serial summary',()=>{
 const item={id:'L01',title:'case'},report={case:'L01',invocationId:'current',status:'passed',finishedAt:'now',steps:[]};
 assert.equal(caseOutcome(item,{code:134},report,'current').exitCode,1);assert.equal(caseOutcome(item,{code:4},report,'current').status,'harness_failed');
 assert.equal(caseOutcome(item,{code:0},report,'new-invocation').status,'harness_failed');assert.equal(caseOutcome(item,{code:0},report,'current').status,'passed');
 assert.equal(caseOutcome(item,{code:0},{...report,status:'preflight_passed',executionMode:'preflight'},'current').status,'preflight_passed');
});
test('cookie imports only matching root application cookies with domain/security/expiry validation',()=>{
 const now=Math.floor(Date.now()/1000),lines=[
  `api.example.test\tFALSE\t/\tTRUE\t${now+300}\tfluctlight_session\tcorrect-session`,
  `.example.test\tTRUE\t/\tTRUE\t0\tfluctlight_csrf\tcorrect-csrf`,
  `other.example.test\tFALSE\t/\tFALSE\t0\tfluctlight_session\twrong-host`,
  `api.example.test\tFALSE\t/auth\tFALSE\t0\tfluctlight_session\twrong-path`,
  `api.example.test\tFALSE\t/\tFALSE\t${now-1}\tfluctlight_session\texpired`,
  `api.example.test\tFALSE\t/\tFALSE\t0\tother_site_cookie\tnever-send`,
 ];
 assert.deepEqual([...importCookies(lines.join('\n'),new URL('https://api.example.test'))],[['fluctlight_session','correct-session'],['fluctlight_csrf','correct-csrf']]);
 assert.equal(importCookies(lines.join('\n'),new URL('http://api.example.test')).size,0);
});
test('external observer signal does not disable the configured request deadline',async()=>{
 const keepAlive=setTimeout(()=>{},1000);
 try{const t=await context(async(_,init)=>new Promise((_,reject)=>init.signal.addEventListener('abort',()=>reject(init.signal.reason),{once:true})));const controller=new AbortController();await assert.rejects(t.request('/api/studio/jobs',{signal:controller.signal,timeoutMs:5}),e=>e.code==='HTTP_TRANSPORT_ERROR');assert.equal(controller.signal.aborted,false);}
 finally{clearTimeout(keepAlive);}
});
test('a hung process family keeping inherited pipes is terminated within a bounded deadline',async()=>{
 const descendant='process.on("SIGTERM",()=>{});setInterval(()=>{},1000);';
 const program=`const {spawn}=require("node:child_process");process.on("SIGTERM",()=>{});spawn(process.execPath,["-e",${JSON.stringify(descendant)}],{stdio:["ignore",process.stdout,process.stderr]});console.log("parent-ready");setInterval(()=>{},1000);`;
 const start=Date.now(),result=await runBoundedProcess(process.execPath,['-e',program],{timeoutMs:200,graceMs:50});assert.equal(result.timedOut,true);assert.ok(Date.now()-start<2000);assert.match(result.output,/parent-ready/);
});
test('model family follows stored parent IDs and does not count unrelated prompt text',()=>{
 const root={id:'qwen',role:'cognitive_assessment',bindingRole:'generic_llm',correlationId:'turn:one',logicalRunId:'logical-one'};
 const reply={id:'cydonia',role:'conversation_reply',bindingRole:'conversation_reply',parentRunId:'logical-one',logicalRunId:'reply:child'};
 const unrelated={id:'other',role:'conversation_reply',prompt:'turn:one',parentRunId:'another-turn'};
 assert.deepEqual(modelRunFamily([root],[root,reply,unrelated],'turn:one').map(r=>r.id),['qwen','cydonia']);
});
test('an old canonical cannot stand in for the selected visual generation session',()=>{
 const queued={status:'active',canonical_asset_id:'old',session:{id:'new-session',status:'queued',current_attempt:2,attempts:[{attempt_number:1,candidate_asset_id:'old-candidate'},{attempt_number:2,candidate_asset_id:'new-candidate'}]}};
 assert.equal(visualGenerationComplete(queued,'new-session'),false);assert.equal(visualGenerationAsset(queued,'new-session'),null);
 const waiting={...queued,session:{...queued.session,status:'awaiting_review'}};assert.equal(visualGenerationAsset(waiting,'new-session'),'new-candidate');assert.equal(visualGenerationAsset(waiting,'wrong-session'),null);
});
test('a failed product cannot pass merely because a loose predicate accepts every terminal state',async()=>{
 const t=await context(async()=>Response.json({}));await assert.rejects(t.until('failed',async()=>({status:'failed',error_code:'npc_model_failed'}),()=>true),e=>e.code==='npc_model_failed');
});
test('all seventeen declared scripts resolve and --help performs no API/model operations',async()=>{
 assert.ok(cases.length>=17);assert.equal(new Set(cases.map(c=>c.id)).size,cases.length);
 const manual=await readFile(fileURLToPath(new URL('../../../docs/v3-llm-acceptance.md',import.meta.url)),'utf8');
 assert.deepEqual([...manual.matchAll(/^## (L\d+) /gm)].map(m=>m[1]),cases.map(c=>c.id),'Every documented model workflow requires an executable script');
 const files=await readdir(fileURLToPath(new URL('.',import.meta.url)));assert.equal(files.filter(name=>/^l\d\d-.*\.mjs$/.test(name)).length,cases.length);
 for(const c of cases){
  assert.ok(files.includes(c.file));const result=await new Promise(resolvePromise=>{const child=spawn(process.execPath,[fileURLToPath(new URL(c.file,import.meta.url)),'--help'],{env:{PATH:process.env.PATH},stdio:['ignore','pipe','pipe']});let out='';child.stdout.on('data',b=>{out+=b;});child.stderr.on('data',b=>{out+=b;});child.on('exit',code=>resolvePromise({code,out}));});
  assert.equal(result.code,0,`${c.id}: ${result.out}`);assert.match(result.out,/--live/);
 }
});
test('selected Go tests and component files actually exist in their owning packages',async()=>{
 const cache=new Map();
 for(const c of cases){const source=await readFile(new URL(c.file,import.meta.url),'utf8');
  for(const match of source.matchAll(/goArgs\(\[([^\]]+)\](?:,\s*['"]([^'"]+)['"])?\)/g)){
   const pkg=match[2]||'./internal/core',root=fileURLToPath(new URL(`../../../apps/core-go/${pkg.replace(/^\.\//,'')}/`,import.meta.url));
   if(!cache.has(root)){const files=(await readdir(root)).filter(name=>name.endsWith('_test.go'));cache.set(root,(await Promise.all(files.map(file=>readFile(join(root,file),'utf8')))).join('\n'));}
   for(const name of match[1].match(/Test[A-Za-z0-9_]+/g)||[])assert.match(cache.get(root),new RegExp(`func\\s+${name}\\s*\\(`),`${c.id} selects missing ${pkg}/${name}`);
  }
  for(const match of source.matchAll(/nodeArgs\(\[([^\]]+)\]\)/g))for(const filename of match[1].match(/['"]([^'"]+\.mjs)['"]/g)||[])await stat(fileURLToPath(new URL('../../../apps/web/test/'+filename.slice(1,-1),import.meta.url)));
 }
});
