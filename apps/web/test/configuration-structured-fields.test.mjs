import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';

function fixture(){
 const source=readFileSync(new URL('../src/v3/proto/components/configuration-editor.js',import.meta.url),'utf8').replace(/^import .+;\s*$/gm,'').replace('export function ','function ');
 const records=new Map(),nodes=new Map();
 const node=selector=>{if(!nodes.has(selector))nodes.set(selector,{value:'',dataset:{},textContent:'',addEventListener(){}});return nodes.get(selector);};
 const raw=node('[data-field="quiet_hours"]');raw.dataset.field='quiet_hours';
 const start=node('start');start.dataset={quietField:'quiet_hours',quietKey:'start'};start.value='23:00';
 const host={isConnected:true,innerHTML:'',querySelector:node,querySelectorAll(selector){return selector==='[data-field]'?[raw]:selector==='[data-quiet-field]'?[start]:[];}};
 const bindings={get:()=>({actorId:'owner'}),configurationClient:{},Field:()=>'',esc:String,confirm:async()=>true,JsonDetails:()=>'',configurationChoiceLabel:String,autonomyActionChoices:[],configurationCommand:()=>({pending:()=>null}),studioDraftStorage:()=>({getItem:key=>records.get(key),setItem:(key,value)=>records.set(key,value)})};
 const mount=Function(...Object.keys(bindings),source+'\nreturn mountConfigurationEditor;')(...Object.values(bindings));
 mount(host,{slotKey:'autonomy',fields:[{key:'quiet_hours',label:'安静时段',type:'quiet-hours'}],initial:{quiet_hours:{start:'22:00',end:'07:00',timezone:'Asia/Shanghai',future:{policy:'retain'}}}});
 const draft=()=>JSON.parse(records.get('fluctlight-studio:owner:configuration:autonomy:draft')).fields.quiet_hours;
 return {host,raw,start,draft};
}
test('structured quiet-hour edits preserve unknown values and malformed advanced drafts stay recoverable',()=>{
 const f=fixture();f.start.oninput();
 assert.deepEqual(JSON.parse(f.draft()),{start:'23:00',end:'07:00',timezone:'Asia/Shanghai',future:{policy:'retain'}});
 for(const value of ['{','null','[]']){
  f.raw.value=value;f.raw.oninput();
  assert.doesNotThrow(()=>f.start.oninput());assert.equal(f.draft(),value);
  assert.match(f.host.querySelector('[data-error]').textContent,/原始草稿已保留/);
 }
});
