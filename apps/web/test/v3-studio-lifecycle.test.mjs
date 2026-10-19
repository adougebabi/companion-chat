import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {persistentCommand} from '../src/v3/providers/persistent-command.mjs';
const tick=()=>new Promise(resolve=>setImmediate(resolve));
function storage(){const data=new Map();return {getItem:k=>data.get(k)??null,setItem:(k,v)=>data.set(k,v),removeItem:k=>data.delete(k)};}
function host(){
 const nodes=new Map(),elements={name:{value:'Independent copy'},source:{value:'published:1'},reason:{value:'Explicit Owner copy'}};
 return {isConnected:true,innerHTML:'',querySelector(selector){if(!nodes.has(selector))nodes.set(selector,{elements,events:new Map(),reportValidity:()=>true,querySelectorAll:()=>[],addEventListener(type,fn){this.events.set(type,fn);}});return nodes.get(selector);}};
}
function fixture(client){
 const saved=storage(),listeners=new Map();let actor='owner';
 const window={addEventListener(name,fn){if(!listeners.has(name))listeners.set(name,new Set());listeners.get(name).add(fn);},removeEventListener:(name,fn)=>listeners.get(name)?.delete(fn),dispatchEvent(event){for(const fn of listeners.get(event.type)||[])fn(event);}};
 const source=readFileSync(new URL('../src/v3/proto/components/resource-lifecycle.js',import.meta.url),'utf8').replace(/^import .+;\s*$/gm,'').replace('export function ','function ');
 const bindings={studioClient:client,esc:String,requireOwner:async()=>true,confirm:async()=>true,Field:()=>'',JsonDetails:JSON.stringify,mountCursorRecords:()=>({dispose(){}}),studioDraftStorage:()=>saved,persistentCommand,get:()=>({actorId:actor}),window,CustomEvent:class{constructor(type,options){this.type=type;this.detail=options.detail;}}};
 const mount=Function(...Object.keys(bindings),source+'\nreturn mountResourceLifecycle;')(...Object.values(bindings));
 const resource={id:'source',name:'Source',kind:'world',revision:2,published_version:'1',draft_version:'1',status:'published'};
 return {mount,saved,resource,setOwner:value=>{actor=value;}};
}
test('parent redraw during copy retains accepted result pointer and the new component reads the actual copy',async()=>{
 let finish;const result=new Promise(resolve=>{finish=resolve;});let copies=0,reads=0;
 const f=fixture({copy:async()=>{copies++;return result;},resource:async id=>{reads++;assert.equal(id,'copy-id');return {id,kind:'world',name:'Persisted copy'};}}),first=host();
 const a=f.mount(first,{actor:'owner',resource:f.resource,alive:()=>true,onChanged:()=>{throw new Error('copy cannot mutate original');}});
 first.querySelector('[data-copy]').onsubmit({preventDefault(){}});await tick();
 a.dispose();const second=host(),b=f.mount(second,{actor:'owner',resource:f.resource,alive:()=>true});
 finish({id:'copy-id'});await tick();await tick();
 assert.equal(copies,1);assert.equal(reads,1);assert.match(second.innerHTML,/#\/studio\/edit\/copy-id/);assert.match(second.innerHTML,/Persisted copy/);
 assert.equal(f.saved.getItem('fluctlight-studio:owner:source:lifecycle'),null);assert.equal(f.saved.getItem('fluctlight-studio:owner:source:lifecycle-receipt'),null);b.dispose();
});
test('archive response after parent redraw refreshes through server readback in the current component',async()=>{
 let finish;const result=new Promise(resolve=>{finish=resolve;});const changed=[];
 const f=fixture({archive:()=>result,resource:async()=>({...f.resource,status:'archived',revision:3})}),first=host();
 const a=f.mount(first,{actor:'owner',resource:f.resource,alive:()=>true,onChanged:r=>changed.push(r)});
 first.querySelector('[data-archive]').onclick();await tick();a.dispose();
 const b=f.mount(host(),{actor:'owner',resource:f.resource,alive:()=>true,onChanged:r=>changed.push(r)});
 finish({id:'source',status:'archived'});await tick();await tick();assert.equal(changed.length,1);assert.equal(changed[0].revision,3);assert.equal(changed[0].status,'archived');b.dispose();
});
test('Owner change during an accepted operation cannot recreate old result pointers',async()=>{
 let finish;const result=new Promise(resolve=>{finish=resolve;});const f=fixture({copy:()=>result}),el=host();
 const a=f.mount(el,{actor:'owner',resource:f.resource,alive:()=>true});el.querySelector('[data-copy]').onsubmit({preventDefault(){}});await tick();
 a.dispose();f.setOwner('other');f.saved.removeItem('fluctlight-studio:owner:source:lifecycle');finish({id:'old-copy'});await tick();await tick();
 assert.equal(f.saved.getItem('fluctlight-studio:owner:source:lifecycle-receipt'),null);assert.equal(f.saved.getItem('fluctlight-studio:owner:source:lifecycle'),null);
});
