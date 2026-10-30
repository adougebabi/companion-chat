import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {act,mountReact,installDOM} from './helpers/react-dom.mjs';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {callDurationLabel} from '../src/approved-ui/task-model-call-presentation.mjs';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const load=path=>vite.ssrLoadModule('/src/'+path),button=(host,text)=>[...host.querySelectorAll('button')].find(n=>n.textContent.trim()===text);
const call=(ref,data)=>({ref,kind:'model-runs',data});
const payload={prompt:{messages:[{role:'system',content:'完整运行规则'},{role:'assistant',content:null,tool_calls:[{id:'physical-tool',function:{name:'inventory_lookup',arguments:'{"count":0}'}}]}],extra:{enabled:false,zero:0,nothing:null,list:[]}},response:{choices:[{message:{content:null,tool_calls:[{id:'response-tool',function:{name:'wear',arguments:'{"confirm":false}'}}]}}],extension:{kept:true}},status:'error',error_code:'provider_exact_code',error:{message:'精确调用错误',nested:{retry:false}},model_id:'actual-model'};
// R16: stored Request/Response are owned by the model-call detail, never task detail.
const selected=data=>({id:'physical',label:'思考与通用分析',model:'actual-model',_record:call('physical',data)});
test('model detail keeps complete stored Request, Response, tool calls and precise errors',async()=>{
 const {default:Details}=await load('react-app/shared/ModelCallDetails.tsx');let recoveries=0;
 const original=JSON.stringify(payload),v=await mountReact(vite,Details,{props:{record:selected(payload),onRecover:()=>recoveries++}});
 try{const values=[...v.host.querySelectorAll('pre')].map(n=>n.textContent);assert.ok(values.includes(JSON.stringify(payload.prompt,null,2)));assert.ok(values.includes(JSON.stringify(payload.response,null,2)));assert.match(v.host.textContent,/provider_exact_code/);assert.match(v.host.textContent,/精确调用错误/);await v.click(button(v.host,'查看原任务与恢复'));assert.equal(recoveries,1);await v.click(button(v.host,'技术详情与关联标识'));assert.equal([...v.host.querySelectorAll('pre')].at(-1).textContent,JSON.stringify(call('physical',payload),null,2));assert.equal(JSON.stringify(payload),original);}finally{await v.dispose();}
});
test('stored missing, null, false, zero, empty strings, raw strings and structures remain distinct',async()=>{
 const {default:Details}=await load('react-app/shared/ModelCallDetails.tsx');
 const v=await mountReact(vite,Details,{props:{record:selected({})}});
 try{assert.match(v.host.textContent,/未记录 Request/);assert.match(v.host.textContent,/未记录 Response/);assert.equal(v.host.querySelector('pre'),null);
 for(const [prompt,response] of [[null,[]],[false,0],['',' {"raw":  false} '],[{},['tool',null,false,0]]]){await v.update({record:selected({prompt,response,error:null,error_code:null})});assert.deepEqual([...v.host.querySelectorAll('pre')].map(n=>n.textContent),[prompt,response].map(value=>typeof value==='string'?value:JSON.stringify(value,null,2)));assert.doesNotMatch(v.host.textContent,/调用错误/);if(prompt===null||prompt==='')assert.match(v.host.textContent,/存储值为/);}
 }finally{await v.dispose();}
});
test('physical call metadata uses only recorded sequence, per-run count, stage and timing',async()=>{
 const {default:Details}=await load('react-app/shared/ModelCallDetails.tsx');
 const v=await mountReact(vite,Details,{props:{record:selected({...payload,sequence:2,round_count:4,stage:'tool_request',started_at:'2026-10-30T01:00:00Z',completed_at:'2026-10-30T01:00:01.250Z'})}});
 try{const summary=()=>Object.fromEntries([...v.host.querySelectorAll('dl>div')].map(n=>[n.querySelector('dt').textContent,n.querySelector('dd').textContent]));assert.deepEqual(summary(),{'模型':'actual-model','用途':'思考与通用分析','耗时':'1.25 秒','调用序号':'2','所属运行调用数':'4','调用阶段':'工具请求'});
 await v.update({record:selected({metrics:{sequence:0,round_count:0,stage:'future-stage',model_id:'metric-model',latency_ms:0},prompt:false,response:0})});assert.deepEqual(summary(),{'模型':'metric-model','用途':'思考与通用分析','耗时':'0 毫秒','调用序号':'0','所属运行调用数':'0','调用阶段':'future-stage'});
 await v.update({record:selected({queued_at:'2026-10-30T00:00:00Z',completed_at:'2026-10-30T01:00:01Z'})});assert.equal(summary()['耗时'],'耗时未记录');assert.equal(summary()['调用序号'],undefined);assert.doesNotMatch(v.host.textContent,/第 .* 轮/);
 }finally{await v.dispose();}
});
test('media stored provider prompt and extensions stay directly reachable',async()=>{
 const {default:Details}=await load('react-app/shared/ModelCallDetails.tsx'),data={prompt:null,response:false,provider_prompt:{tools:[{extension:0}]},negative_prompt:'',parameters:{seed:0},result:{asset_id:'actual-asset'}};let opened;
 const v=await mountReact(vite,Details,{props:{record:{...selected(data),_record:{kind:'media-prompts',ref:'media',data}},onOpenMedia:id=>opened=id}});
 try{assert.match(v.host.textContent,/extension/);assert.match(v.host.textContent,/seed/);await v.click(button(v.host,'查看已记录素材'));assert.equal(opened,'actual-asset');}finally{await v.dispose();}
});
test('duration never uses queued/created time, a browser clock, missing/invalid/reversed instants or coerced latency',()=>{
 const instant='2026-10-30T01:00:00Z';
 for(const record of [
  {},{queued_at:instant,completed_at:instant},{created_at:instant,completed_at:instant},
  {started_at:instant},{completed_at:instant},{started_at:'invalid',completed_at:instant},
  {started_at:instant,completed_at:'invalid'},{started_at:'2026-10-30T01:00:01Z',completed_at:instant},
  {started_at:0,completed_at:0},{latency_ms:null},{latency_ms:false},{latency_ms:''},{latency_ms:'123'},{latency_ms:-1},{latency_ms:Infinity},
 ])assert.equal(callDurationLabel(record),'耗时未记录',JSON.stringify(record));
 assert.equal(callDurationLabel({started_at:instant,completed_at:instant}),'0 毫秒（起止时间）');
 assert.equal(callDurationLabel({metrics:{started_at:instant,completed_at:'2026-10-30T01:00:02Z'}}),'2 秒（起止时间）');
 assert.equal(callDurationLabel({latency_ms:123.5}),'123.5 毫秒（记录耗时）');
});
