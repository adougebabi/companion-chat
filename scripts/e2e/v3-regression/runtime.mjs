import {mkdir,readFile,writeFile,rename} from 'node:fs/promises';
import {resolve,join,dirname} from 'node:path';
import {fileURLToPath} from 'node:url';
import {createHash,randomUUID} from 'node:crypto';
import {spawn} from 'node:child_process';
import {isDeepStrictEqual} from 'node:util';
import {cases} from './cases.mjs';

export const repoRoot=resolve(dirname(fileURLToPath(import.meta.url)),'../../..');
export const hash=value=>createHash('sha256').update(typeof value==='string'?value:ArrayBuffer.isView(value)?Buffer.from(value.buffer,value.byteOffset,value.byteLength):JSON.stringify(value)).digest('hex');
const copy=value=>structuredClone(value);
const delay=ms=>new Promise(resolve=>setTimeout(resolve,ms));
const secretKey=/password|credential|authorization|cookie|csrf|secret|api.?key|access.?key|private.?key|settings.?key|service.?key|prompt|reasoning|source_text|structured_projection/i;
export function redact(value,depth=0){
 if(depth>12)return '[depth limited]';
 if(Array.isArray(value))return value.slice(0,100).map(v=>redact(v,depth+1));
 if(value&&typeof value==='object')return Object.fromEntries(Object.entries(value).map(([k,v])=>[k,secretKey.test(k)?'[redacted]':redact(v,depth+1)]));
 if(typeof value==='string')return value.replace(/(postgres(?:ql)?:\/\/)[^\s/@]+:[^\s/@]+@/gi,'$1[redacted]@').replace(/Bearer\s+[^\s"']+/gi,'Bearer [redacted]').slice(0,12000);
 return value;
}
export class RegressionError extends Error{
 constructor(code,message,details={},status='failed'){super(message);this.code=code;this.details=details;this.status=status;}
}
export function check(condition,message,expected=true,actual){if(arguments.length<4)actual=condition;if(!condition)throw new RegressionError('ASSERTION_FAILED',message,{expected,actual});}
export function equal(actual,expected,message){check(isDeepStrictEqual(actual,expected),message,expected,actual);}
export function assertGoResults(output,expectedNames=[]){
 const events=output.split('\n').flatMap(line=>{try{return [JSON.parse(line)];}catch{return [];}}),tests=events.filter(v=>v.Action==='pass'&&v.Test),skips=events.filter(v=>v.Action==='skip'&&v.Test);
 if(!tests.length||skips.length)throw new RegressionError('TESTS_SKIPPED','Selected Go tests were absent or skipped',{passed:tests.map(t=>t.Test),skipped:skips.map(t=>t.Test)},'blocked');
 const missing=expectedNames.filter(name=>!tests.some(t=>t.Test===name));if(missing.length)throw new RegressionError('TEST_SELECTION_INCOMPLETE','One or more explicitly selected tests did not run',{missing,passed:tests.map(t=>t.Test)},'blocked');
 return {passed:tests.map(t=>t.Test)};
}
const applicationCookies=new Set(['fluctlight_session','fluctlight_csrf']);
export function cookieApplies({domain,subdomains=false,path='/',secure=false,expires=0,name},target,now=Date.now()){
 const host=target.hostname.toLowerCase(),expected=domain.replace(/^\./,'').toLowerCase();
 return Number.isFinite(expires)&&expires>=0&&applicationCookies.has(name)&&(host===expected||subdomains&&host.endsWith('.'+expected))&&path==='/'&&(!secure||target.protocol==='https:')&&(!expires||expires*1000>now);
}
export function importCookies(text,target){
 const result=new Map();for(const raw of text.split('\n')){if(!raw||raw.startsWith('#')&&!raw.startsWith('#HttpOnly_'))continue;const fields=raw.replace(/^#HttpOnly_/,'').replace(/\r$/,'').split('\t');if(fields.length<7)continue;
  const [domain,includeSubdomains,path,secure,expiry,name,value]=fields;const expires=Number(expiry);if(!Number.isFinite(expires))continue;
  if(cookieApplies({domain,subdomains:includeSubdomains==='TRUE',path,secure:secure==='TRUE',expires,name},target))result.set(name,value);
 }return result;
}
export async function runBoundedProcess(executable,parameters,{cwd=repoRoot,env=process.env,timeoutMs,graceMs=1000}={}){
 return new Promise((resolvePromise,reject)=>{
  const grouped=process.platform!=='win32',child=spawn(executable,parameters,{cwd,env,detached:grouped,stdio:['ignore','pipe','pipe']});let output='',timedOut=false,settled=false;
  const kill=signal=>{try{if(grouped&&child.pid)process.kill(-child.pid,signal);else if(child.pid){const stop=spawn('taskkill',['/PID',String(child.pid),'/T','/F'],{stdio:'ignore'});stop.on('error',()=>child.kill(signal));}}catch(error){if(error.code!=='ESRCH')child.kill(signal);}};
  const settle=code=>{if(settled)return;settled=true;clearTimeout(timer);clearTimeout(hardTimer);clearTimeout(finalTimer);child.stdout.destroy();child.stderr.destroy();resolvePromise({code,output,timedOut});};
  child.stdout.on('data',data=>{output+=data;});child.stderr.on('data',data=>{output+=data;});
  const timer=setTimeout(()=>{timedOut=true;kill('SIGTERM');},timeoutMs),hardTimer=setTimeout(()=>kill('SIGKILL'),timeoutMs+graceMs),finalTimer=setTimeout(()=>{timedOut=true;kill('SIGKILL');settle(-1);},timeoutMs+graceMs*2);
  child.on('error',error=>{clearTimeout(timer);clearTimeout(hardTimer);clearTimeout(finalTimer);reject(error);});child.on('close',settle);
 });
}
export function optionsFrom(argv=process.argv.slice(2),env=process.env){
 const options={live:false,preflight:false,unit:true,runDir:env.FLUCTLIGHT_E2E_RUN_DIR||'',timeoutMs:Number(env.FLUCTLIGHT_E2E_TIMEOUT_MS||600000),requestTimeoutMs:Number(env.FLUCTLIGHT_E2E_REQUEST_TIMEOUT_MS||30000),pollMs:Number(env.FLUCTLIGHT_E2E_POLL_MS||3000)};
 for(let i=0;i<argv.length;i++){
  const arg=argv[i];if(arg==='--live')options.live=true;else if(arg==='--preflight')options.preflight=true;else if(arg==='--no-unit')options.unit=false;else if(arg==='--run-dir')options.runDir=argv[++i];else if(arg==='--timeout-ms')options.timeoutMs=Number(argv[++i]);else if(arg==='--help')options.help=true;else throw new RegressionError('ARGUMENT_INVALID',`Unknown argument ${arg}`);
 }
 for(const name of ['timeoutMs','requestTimeoutMs','pollMs'])if(!Number.isFinite(options[name])||options[name]<1)throw new RegressionError('ARGUMENT_INVALID',`Invalid ${name}`);
 return options;
}
async function atomicJSON(path,value){await mkdir(dirname(path),{recursive:true,mode:0o700});const temp=path+'.'+randomUUID()+'.tmp';await writeFile(temp,JSON.stringify(value,null,2)+'\n',{mode:0o600});await rename(temp,path);}
export async function readJSON(path,fallback){try{return JSON.parse(await readFile(path,'utf8'));}catch(error){if(error.code==='ENOENT')return fallback;throw new RegressionError('STATE_INVALID',`Cannot read ${path}`,{cause:error.message});}}

export class RegressionContext{
 constructor(id,title,options,env=process.env,fetcher=fetch){
  this.id=id;this.title=title;this.options=options;this.env=env;this.fetcher=fetcher;this.cookies=new Map();this.activeStep=null;this.modelOperations=0;
  this.base=new URL(env.FLUCTLIGHT_E2E_BASE_URL||'http://127.0.0.1:8081');this.origin=env.FLUCTLIGHT_E2E_ORIGIN||this.base.origin;
  this.runDir=resolve(options.runDir||join(repoRoot,'.v3-regression',new Date().toISOString().replace(/[:.]/g,'-')));
  this.report={schemaVersion:1,case:id,title,status:'running',invocationId:env.FLUCTLIGHT_E2E_INVOCATION_ID||randomUUID(),startedAt:new Date().toISOString(),executionLayer:'real-http-and-isolated-tests',baseURL:this.base.origin,steps:[],requests:[],reviews:[],coverageLimits:['HTTP and component tests do not certify real-browser rendering, IME or scrolling.','Controlled Provider fixtures do not certify a real model/renderer.']};
 }
 async init(){
  await mkdir(this.runDir,{recursive:true,mode:0o700});this.statePath=join(this.runDir,'private','state.json');
  this.state=await readJSON(this.statePath,{schemaVersion:1,runId:randomUUID(),base:this.base.origin,origin:this.origin,objects:{},commands:{}});
  check(this.state.base===this.base.origin&&this.state.origin===this.origin,'Do not resume a run against another environment',{base:this.state.base,origin:this.state.origin},{base:this.base.origin,origin:this.origin});
  this.report.runId=this.state.runId;await this.save();
 }
 async save(){await atomicJSON(this.statePath,this.state);}
 token(name){return `v3-regression:${this.state.runId}:${name}`;}
 object(name){return copy(this.state.objects[name]);}
 async remember(name,value){this.state.objects[name]=copy(value);await this.save();return value;}
 async step(id,expected,fn){
  const row={id,expected,status:'running',startedAt:new Date().toISOString()};this.report.steps.push(row);this.activeStep=id;
  process.stdout.write(`[${this.id}] ${id}: ${expected}\n`);
  try{const result=await fn();row.status='passed';if(result!==undefined)row.observed=redact(result);return result;}
  catch(error){row.status=error.status||'failed';row.error=this.sanitize({code:error.code||'UNEXPECTED_ERROR',message:error.message,details:error.details||{},stack:error.stack});process.stderr.write(`[${this.id}] ${id}: ${row.status} ${row.error.code}: ${row.error.message}\n`);return undefined;}
  finally{row.finishedAt=new Date().toISOString();this.activeStep=null;await this.writeReport();}
 }
 blocked(code,message,details={}){throw new RegressionError(code,message,details,'blocked');}
 model(label){
  if(!this.options.live)this.blocked('LIVE_EXECUTION_DISABLED',`${label} requires --live; no model or model-triggering mutation was sent`);
  const maximum=Number(this.env.FLUCTLIGHT_E2E_MAX_MODEL_OPERATIONS||32);
  if(++this.modelOperations>maximum)this.blocked('MODEL_OPERATION_LIMIT','Explicit model-triggering operation limit reached',{maximum,label});
 }
 review(id,description,evidence={}){this.report.reviews.push({id,status:'pending_user_review',description,evidence:redact(evidence)});}
 sanitizeText(value){
  let text=value.replace(/(postgres(?:ql)?:\/\/)[^\s/@]+:[^\s/@]+@/gi,'$1[redacted]@').replace(/Bearer\s+[^\s"']+/gi,'Bearer [redacted]');
  for(const [name,secret] of Object.entries(this.env))if(secretKey.test(name)&&typeof secret==='string'&&secret.length>=6)text=text.split(secret).join('[redacted]');
  for(const secret of this.cookies.values())if(secret?.length>=6)text=text.split(secret).join('[redacted]');
  return text;
 }
 sanitize(value){
  let text=JSON.stringify(redact(value));
  return JSON.parse(this.sanitizeText(text));
 }
 async writeReport(){await atomicJSON(join(this.runDir,`${this.id}.json`),this.sanitize(this.report));}
 async finish(){
  if(!this.options.preflight)for(const step of cases.find(c=>c.id===this.id)?.steps||[])if(!this.report.steps.some(s=>s.id===step))this.report.steps.push({id:step,status:'blocked',error:{code:'PREREQUISITE_NOT_MET',message:'An earlier required step did not produce its real prerequisite; this step was not executed.'}});
  const statuses=this.report.steps.map(s=>s.status);this.report.status=statuses.includes('failed')?'failed':statuses.includes('blocked')?'blocked':statuses.length?'passed':'blocked';
  this.report.automatedStatus=this.report.status;this.report.fullWorkflowAccepted=false;this.report.modelTriggerOperations=this.modelOperations;this.report.finishedAt=new Date().toISOString();
  this.report.executionMode=this.options.preflight?'preflight':'scenario';if(this.options.preflight&&this.report.status==='passed')this.report.status='preflight_passed';
  if(this.report.status==='passed'&&this.report.reviews.length)this.report.status='review_required';
  await this.writeReport();return this.report.status==='failed'?1:this.report.status==='blocked'?2:this.report.status==='review_required'?3:0;
 }
 rememberCookies(response){for(const value of response.headers.getSetCookie?.()||[]){const [first,...attributes]=value.split(';'),at=first.indexOf('=');if(at<1)continue;const name=first.slice(0,at),attrs=Object.fromEntries(attributes.map(raw=>{const [key,...rest]=raw.trim().split('=');return [key.toLowerCase(),rest.join('=')];}));if(cookieApplies({domain:attrs.domain||this.base.hostname,subdomains:!!attrs.domain,path:attrs.path||'/',secure:'secure'in attrs,expires:attrs.expires?Date.parse(attrs.expires)/1000:0,name},this.base))this.cookies.set(name,first.slice(at+1));}}
 async request(path,{method='GET',body,multipart,expected=[200],anonymous=false,signal,timeoutMs,stream=false}={}){
  check(path.startsWith('/api/')||path.startsWith('/auth/')||path.startsWith('/health/'),'Request must use an explicit public route','public API',path);
  if(method!=='GET'&&method!=='HEAD'&&!path.startsWith('/auth/')){
   if(this.env.FLUCTLIGHT_E2E_ISOLATED!=='1')this.blocked('ISOLATED_ENVIRONMENT_REQUIRED','Set FLUCTLIGHT_E2E_ISOLATED=1 only for a task-owned test deployment; no mutation was sent');
   check(['127.0.0.1','localhost','[::1]'].includes(this.base.hostname)||this.env.FLUCTLIGHT_E2E_ALLOW_REMOTE==='1','Remote mutations require an explicitly chosen test deployment');
  }
  const headers=new Headers({Origin:this.origin});if(!anonymous){if(this.cookies.size)headers.set('Cookie',[...this.cookies].map(([k,v])=>`${k}=${v}`).join('; '));if(this.cookies.has('fluctlight_csrf'))headers.set('X-CSRF-Token',this.cookies.get('fluctlight_csrf'));}
  check(!(body!==undefined&&multipart!==undefined),'Use either JSON or explicit multipart data');
  if(multipart!==undefined)check(multipart instanceof FormData,'Multipart requests require actual FormData');
  if(body!==undefined)headers.set('Content-Type','application/json');
  const started=Date.now(),record={step:this.activeStep,method,path,startedAt:new Date().toISOString()};this.report.requests.push(record);
  let response;
  const deadline=AbortSignal.timeout(timeoutMs||this.options.requestTimeoutMs),combined=signal?AbortSignal.any([signal,deadline]):deadline;
  try{response=await this.fetcher(new URL(path,this.base),{method,headers,body:multipart??(body===undefined?undefined:JSON.stringify(body)),signal:combined,redirect:'error'});}
  catch(error){record.elapsedMs=Date.now()-started;record.errorCode=error.name;throw new RegressionError('HTTP_TRANSPORT_ERROR',`${method} ${path} did not return an authoritative response`,{cause:error.message,method,path,elapsedMs:record.elapsedMs});}
  if(!anonymous)this.rememberCookies(response);record.status=response.status;record.elapsedMs=Date.now()-started;record.correlationId=response.headers.get('x-correlation-id')||response.headers.get('x-request-id')||undefined;
  if(stream&&expected.includes(response.status))return response;
  let text;try{text=await response.text();}catch(error){throw new RegressionError('HTTP_BODY_INTERRUPTED',`${method} ${path} body was interrupted`,{status:response.status,cause:error.message});}let value;
  try{value=text?JSON.parse(text):null;}catch{throw new RegressionError('HTTP_INVALID_JSON',`${method} ${path} returned non-JSON`,{status:response.status,responseDigest:hash(text)});}
  record.errorCode=value?.error?.code||value?.code;
  if(!expected.includes(response.status))throw new RegressionError(record.errorCode||'HTTP_UNEXPECTED_STATUS',`${method} ${path}: HTTP ${response.status}`,{expectedStatus:expected,actualStatus:response.status,path,response:redact(value)});
  return {status:response.status,value};
 }
 async get(path,options){return (await this.request(path,options)).value;}
 async upload(name,{filename,mime,bytes}){
  const key=`${this.id}:upload:${name}`;let record=this.state.commands[key];
  if(!record){check(bytes instanceof Uint8Array&&bytes.length>0&&bytes.length<=64*1024*1024,'Upload needs bounded actual fixture bytes');record={method:'POST',path:'/api/media',filename,mime,sha256:hash(bytes),bytes:Buffer.from(bytes).toString('base64'),idempotencyKey:this.token('upload:'+name)};this.state.commands[key]=record;await this.save();}
  check(record.method==='POST'&&record.path==='/api/media','Upload identity must retain its original route');
  const frozen=Buffer.from(record.bytes,'base64');equal(hash(frozen),record.sha256,'Frozen upload bytes must retain their checksum');
  const form=new FormData();form.set('file',new Blob([frozen],{type:record.mime}),record.filename);form.set('mime',record.mime);form.set('idempotencyKey',record.idempotencyKey);
  record.sentAt=new Date().toISOString();await this.save();return this.request('/api/media',{method:'POST',multipart:form,expected:[200,201]});
 }
 async command(name,method,path,makeBody,{expected=[200,201,202],replay='same-key',...options}={}){
  const key=`${this.id}:${name}`;let record=this.state.commands[key];
  if(!record){const body=typeof makeBody==='function'?await makeBody():copy(makeBody);record={method,path,body};this.state.commands[key]=record;await this.save();}
  check(record.path===path&&record.method===method,'Original mutation identity belongs to another route',{method:record.method,path:record.path},{method,path});
  if(replay==='read-result'&&record.receipt)return copy(record.receipt);
  if(replay==='read-result'&&record.sentAt)this.blocked('NONREPLAYABLE_OUTCOME_UNKNOWN','The original non-idempotent operation has no confirmed receipt. It was not silently repeated.',{command:key,method,path});
  // Re-run the frozen request, including its original expected revision and
  // timestamps. Unknown outcomes never get a fresh key or reconstructed body.
  record.sentAt=new Date().toISOString();await this.save();
  const receipt=await this.request(path,{...options,method,body:copy(record.body),expected});
  if(replay==='read-result'){record.receipt=receipt;await this.save();}
  return receipt;
 }
 async preflight(){
  if(!['127.0.0.1','localhost','[::1]'].includes(this.base.hostname)&&this.env.FLUCTLIGHT_E2E_ALLOW_REMOTE!=='1')this.blocked('REMOTE_ENVIRONMENT_NOT_SELECTED','Remote authentication requires an explicitly chosen test environment; no credentials were sent');
  let host,ready;try{host=await this.get('/health/live');ready=await this.get('/health/ready');}catch(error){this.blocked('ENVIRONMENT_UNAVAILABLE','The real API/database precondition is unavailable; no scenario or model call was started',{cause:error.message,code:error.code});}
  check(host?.status==='ok'&&ready?.status==='ready','API and database must be healthy',{live:'ok',ready:'ready'},{host,ready});
  if(this.env.FLUCTLIGHT_E2E_COOKIE_FILE){const raw=await readFile(this.env.FLUCTLIGHT_E2E_COOKIE_FILE,'utf8');for(const [name,value]of importCookies(raw,this.base))this.cookies.set(name,value);}
  // Import the supplied session before the first auth check. An anonymous
  // 401 is the real login contract and may still establish the CSRF cookie;
  // database/transport failures remain errors, never an anonymous fallback.
  let session=(await this.request('/auth/session',{expected:[200,401]})).value;
  if(!session?.authenticated){if(!this.env.FLUCTLIGHT_E2E_PASSWORD)this.blocked('TEST_LOGIN_REQUIRED','Supply FLUCTLIGHT_E2E_PASSWORD or a private FLUCTLIGHT_E2E_COOKIE_FILE; credentials are never written to reports');await this.request('/auth/login',{method:'POST',body:{password:this.env.FLUCTLIGHT_E2E_PASSWORD},expected:[200]});session=await this.get('/auth/session');}
  check(session?.authenticated&&typeof session.actorId==='string','An actual Owner session is required');
  if(this.state.owner)equal(session.actorId,this.state.owner,'Resume must use the original Owner');this.state.owner=session.actorId;this.owner=session.actorId;await this.save();
  return {authenticated:true,ownerActorId:this.owner,baseURL:this.base.origin,liveEnabled:this.options.live};
 }
 async until(label,read,predicate,{terminalFailure=value=>['failed','dead_letter','terminated'].includes(value?.status),timeoutMs=this.options.timeoutMs}={}){
  const start=Date.now();let latest;
  while(Date.now()-start<timeoutMs){latest=await read();if(terminalFailure(latest))throw new RegressionError(latest.error_code||latest.last_error_code||'TASK_FAILED',`${label} reached a failure state`,{latest});if(predicate(latest))return latest;await delay(this.options.pollMs);}
  throw new RegressionError('TASK_TIMEOUT',`${label} did not produce the required result before the deadline`,{timeoutMs,latest});
 }
 async pages(path,{items='items',cursor='next_cursor',maxPages=100}={}){
  const rows=[],seen=new Set();let next='';for(let page=0;page<maxPages;page++){
   const url=new URL(path,this.base);if(next)url.searchParams.set('cursor',next);const value=await this.get(url.pathname+url.search);check(Array.isArray(value?.[items]),'Paged response must contain an actual list',items,value);
   rows.push(...value[items]);next=value[cursor];if(!next)return rows;check(!seen.has(next),'Pagination cursor must progress');seen.add(next);
  }throw new RegressionError('PAGINATION_LIMIT','Pagination remains incomplete',{path,maxPages,loaded:rows.length});
 }
 async suite(name,args,{database=false,layer='controlled-tests'}={}){
  if(!this.options.unit)this.blocked('SUITE_NOT_EXECUTED','This branch requires the isolated test suite; remove --no-unit',{name});
  if(database&&!this.env.GO_CORE_TEST_DATABASE_URL)this.blocked('TEST_DATABASE_REQUIRED','GO_CORE_TEST_DATABASE_URL must point to a disposable test database endpoint',{name});
  if(database&&this.env.FLUCTLIGHT_E2E_ISOLATED!=='1')this.blocked('ISOLATED_ENVIRONMENT_REQUIRED','Database regression requires an explicitly isolated deployment',{name});
  const executable=args[0],parameters=args.slice(1);const result=await runBoundedProcess(executable,parameters,{env:{...this.env,FLUCTLIGHT_LIVE_PROVIDER_TEST:'0',GOCACHE:this.env.GOCACHE||'/tmp/fluctlight-go-cache'},timeoutMs:this.options.timeoutMs});
  const log=join(this.runDir,`${this.id}-${name}.log`);await writeFile(log,this.sanitizeText(result.output),{mode:0o600});
  if(result.timedOut)throw new RegressionError('SUITE_TIMEOUT',`${name} exceeded its deadline`,{log});
  check(result.code===0,`${name} failed`,0,{exitCode:result.code,log});
  if(executable==='go'){
   const selected=parameters[parameters.indexOf('-run')+1],names=selected?.startsWith('^(')&&selected.endsWith(')$')?selected.slice(2,-2).split('|'):[];
   try{assertGoResults(result.output,names);}catch(error){error.details.log=log;throw error;}
  }
  if(executable===process.execPath||executable==='node')if(parameters.includes('--test')){
   const count=Number(result.output.match(/(?:#|ℹ) tests (\d+)/)?.[1]||0),skipped=Number(result.output.match(/(?:#|ℹ) skipped (\d+)/)?.[1]||0),todo=Number(result.output.match(/(?:#|ℹ) todo (\d+)/)?.[1]||0);
   if(!count||skipped||todo)this.blocked('TESTS_SKIPPED',`${name} did not execute all component assertions`,{tests:count,skipped,todo,log});
  }
  return {layer,exitCode:result.code,log};
 }
}

export async function runScenario(id,title,run){
 let ctx;
 try{
  const options=optionsFrom();if(options.help){console.log(`${id}: ${title}\nUsage: node <script> --live --run-dir <private run folder> [--preflight] [--no-unit]\nCredentials: FLUCTLIGHT_E2E_PASSWORD or FLUCTLIGHT_E2E_COOKIE_FILE. Mutations require FLUCTLIGHT_E2E_ISOLATED=1.`);return;}
  ctx=new RegressionContext(id,title,options);await ctx.init();const ready=await ctx.step('preflight','Healthy real API, database and original Owner session',()=>ctx.preflight());
  if(ready&&!options.preflight){if(!options.live)await ctx.step('live-execution','User explicitly enables model-related business regression',async()=>ctx.blocked('LIVE_EXECUTION_DISABLED','Use --live only after the full delivery is ready; no scenario/model-triggering mutation was sent'));else await run(ctx);}
  if(options.preflight)ctx.report.coverageLimits.push('Preflight only: no scenario, model or business operation was executed.');
  process.exitCode=await ctx.finish();console.log(`${id}: ${ctx.report.status}; report ${join(ctx.runDir,id+'.json')}`);
 }catch(error){if(ctx){await ctx.step('harness','Regression harness must finish with diagnostic evidence',async()=>{throw error;});process.exitCode=await ctx.finish();}else{console.error(JSON.stringify(redact({code:error.code||'HARNESS_ERROR',message:error.message,details:error.details})));process.exitCode=1;}}
}
