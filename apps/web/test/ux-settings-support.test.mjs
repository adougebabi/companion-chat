import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {URL as NodeURL} from 'node:url';
import {configurationTestSummary,beijingFilterInput,diagnosticTimeFilter} from '../src/v3/configuration-feedback.mjs';

const source=name=>readFileSync(new NodeURL(`../src/v3/proto/components/${name}.js`,import.meta.url),'utf8').replace(/^import .*;\s*$/gm,'').replace(/^export /gm,'');
const settle=()=>new Promise(resolve=>setImmediate(resolve));
function box(){const nodes=new Map();return {isConnected:true,innerHTML:'',insertAdjacentHTML(_where,html){this.innerHTML+=html;},querySelector(selector){if(!nodes.has(selector))nodes.set(selector,{attributes:{},setAttribute(key,value){this.attributes[key]=value;},remove(){},onclick:null});return nodes.get(selector);}};}

test('a failed call or a nested recent error takes precedence over a healthy saved configuration',()=>{
 assert.equal(configurationTestSummary({call_status:'unavailable',error_code:'request_failed'}).state,'failed');
 assert.equal(configurationTestSummary({status:'available',last_call:{last_error:'request_timeout'}}).state,'failed');
 assert.equal(configurationTestSummary({call_status:'ok',error_code:''}).state,'success');
 assert.equal(configurationTestSummary({status:'completed'}).state,'unknown');
});
test('vision requires actual image verification while media reachability does not claim generation',()=>{
 assert.equal(configurationTestSummary({state:'ready',image_input_verified:false}).state,'unknown');
 assert.equal(configurationTestSummary({state:'ready',image_input_verified:true,error_code:''}).state,'success');
 assert.equal(configurationTestSummary({state:'reachable',generation_tested:false}).state,'success');
 assert.equal(configurationTestSummary({state:'unconfigured',generation_tested:false}).state,'failed');
});
test('diagnostic filters convert ordinary Beijing time and reject rollover or malformed dates',()=>{
 assert.equal(diagnosticTimeFilter('2026-10-22 10:00'),'2026-10-22T10:00:00+08:00');
 assert.equal(beijingFilterInput('2026-10-22T02:00:00Z'),'2026-10-22 10:00');
 assert.equal(diagnosticTimeFilter(''), '');
 for(const value of ['2026-02-30 10:00','2026-10-22 25:00','2026-10-22','bad','2026-10-22 10:00:60'])assert.throws(()=>diagnosticTimeFilter(value),/请|无效/);
});
test('initial cursor failure removes loading and can retry, later failure retains the last successful page',async()=>{
 let fail=true;const host=box(),message=()=> '读取失败，请重试';
 const mount=Function('get','subscribe','CursorPager','bindCursorPager','esc','errorMessage',source('cursor-records')+'\nreturn mountCursorRecords;')(()=>({actorId:'owner'}),()=>{},()=>'',()=>{},String,message);
 const panel=mount(host,{read:async()=>{if(fail)throw Error('request_failed');return {items:['成功记录']};},render:String});
 await settle();assert.doesNotMatch(host.innerHTML,/正在读取/);assert.match(host.innerHTML,/读取失败/);
 fail=false;host.querySelector('[data-panel-retry]').onclick();await settle();assert.match(host.innerHTML,/成功记录/);
 fail=true;await panel.reload();assert.match(host.innerHTML,/成功记录/);assert.match(host.innerHTML,/读取失败/);panel.dispose();
});
test('async page error offers a business return without changing old read/paint arguments',async()=>{
 const host=box();const mount=Function('esc','errorMessage','JsonDetails',source('async-page')+'\nreturn asyncPage;')(String,()=> '暂时无法读取',()=>'<details>技术详情</details>');
 mount(host,async()=>{throw Error('unavailable');},()=>{throw Error('must not paint');},{backHref:'#/inbox',backLabel:'返回收件箱'});
 await settle();assert.doesNotMatch(host.innerHTML,/正在读取/);assert.match(host.innerHTML,/href="#\/inbox"/);assert.match(host.innerHTML,/返回收件箱/);
});
test('unsupported file picker uses bounded Blob download only after complete terminal receipt',async()=>{
 const oldWindow=globalThis.window,oldDocument=globalThis.document,oldURL=globalThis.URL;let clicked=0,appended=0,removed=0;
 globalThis.window={showSaveFilePicker:async()=>{throw Object.assign(Error('blocked'),{name:'SecurityError'});}};
 globalThis.document={body:{append(){appended++;}},createElement:()=>({click(){clicked++;},remove(){removed++;}})};
 globalThis.URL={createObjectURL:()=> 'blob:test',revokeObjectURL(){}};
 try {
  const text=JSON.stringify({type:'end',complete:true,counts:{events:1}})+'\n';
  const download=Function('diagnosticsClient','setTimeout',source('diagnostic-download')+'\nreturn downloadDiagnostics;')({export:async()=>({body:new ReadableStream({start(controller){controller.enqueue(new TextEncoder().encode(text));controller.close();}})})},()=>{});
  const receipt=await download({},{});assert.equal(receipt.complete,true);assert.equal(clicked,1);assert.equal(appended,1);assert.equal(removed,1);
  const incomplete=Function('diagnosticsClient','setTimeout',source('diagnostic-download')+'\nreturn downloadDiagnostics;')({export:async()=>({body:new ReadableStream({start(controller){controller.enqueue(new TextEncoder().encode('{}\n'));controller.close();}})})},()=>{});
  await assert.rejects(incomplete({}),/没有确认导出完整/);assert.equal(clicked,1);
 } finally {globalThis.window=oldWindow;globalThis.document=oldDocument;globalThis.URL=oldURL;}
});
test('numeric edits compare values and an untouched editor cannot submit an unchanged form',()=>{
 const records=new Map(),nodes=new Map(),raw={type:'number',value:'10',dataset:{field:'limit'}};
 const node=selector=>{if(!nodes.has(selector))nodes.set(selector,{value:'',checked:false,textContent:'',addEventListener(){}});return nodes.get(selector);};
 const host={isConnected:true,innerHTML:'',querySelector:node,querySelectorAll:selector=>selector==='[data-field]'?[raw]:[]};
 const mount=Function('get','studioDraftStorage','configurationCommand','Field','esc','JsonDetails',source('configuration-editor')+'\nreturn mountConfigurationEditor;')(()=>({actorId:'owner'}),()=>({getItem:key=>records.get(key),setItem:(key,value)=>records.set(key,value)}),()=>({pending:()=>null}),()=>'',String,()=> '');
 mount(host,{slotKey:'numeric',fields:[{key:'limit',type:'number'}],initial:{limit:10}});
 assert.equal(node('[data-save]').disabled,true);
 raw.oninput();assert.equal(node('[data-save]').disabled,true);
 raw.value='11';raw.oninput();assert.equal(node('[data-save]').disabled,false);
 raw.value='10';raw.oninput();assert.equal(node('[data-save]').disabled,true);
});
test('an original-only recovery drawer without a pending identity cannot create an empty command',async()=>{
 let admitted=0;const host=box(),mount=Function('get','persistentCommand','studioDraftStorage','drawer','esc','subscribe',source('durable-command-drawer')+'\nreturn durableCommandDrawer;')(()=>({actorId:'owner'}),()=>({pending:()=>null,submit:async()=>{admitted++;}}),()=>({getItem:()=>null}),()=>({el:host,close(){}}),String,()=>()=>{});
 mount({slot:'original',title:'原提交',body:'',read:()=>({}),execute(){},originalOnly:true});
 assert.equal(host.querySelector('[data-command-send]').disabled,true);
 assert.equal(host.querySelector('[data-command-send]').hidden,true);
 assert.equal(host.querySelector('[data-command-retry]').hidden,true);
 assert.equal(host.querySelector('[data-command-error]').attributes['data-feedback-kind'],'information');
 await host.querySelector('[data-command-send]').onclick();assert.equal(admitted,0);
});
