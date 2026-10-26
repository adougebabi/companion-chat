import {readFileSync} from 'node:fs';
import * as Vue from 'vue';
import {parse,compileTemplate} from 'vue/compiler-sfc';
import {renderToString} from 'vue/server-renderer';

// Execute the actual SFC setup and compile its actual template. Only external
// services/lifecycle are injected; no second implementation of page behavior.
export function setupSFC(url,bindings,names){
 const raw=readFileSync(url,'utf8'),{descriptor,errors}=parse(raw);
 if(errors.length)throw errors[0];
 const mounted=[],unmounted=[];
 const runtime={ref:Vue.ref,computed:Vue.computed,reactive:Vue.reactive,watch:Vue.watch,nextTick:Vue.nextTick,onMounted:fn=>mounted.push(fn),onUnmounted:fn=>unmounted.push(fn),onBeforeUnmount:fn=>unmounted.push(fn),...bindings};
 const source=descriptor.scriptSetup.content.replace(/^import\s[\s\S]*?from\s*['"][^'"]+['"];?\s*$/gm,'');
 const state=Function(...Object.keys(runtime),source+'\nreturn {'+names.join(',')+'};')(...Object.values(runtime));
 return {state,async mount(){for(const fn of mounted)await fn();},unmount(){for(const fn of unmounted)fn();},async html(components={}){
  const compiled=compileTemplate({source:descriptor.template.content,filename:String(url),id:'r10-page-harness'});
  if(compiled.errors.length)throw compiled.errors[0];
  const code=compiled.code.replace(/import\s*\{([\s\S]*?)\}\s*from\s*["']vue["'];?/g,(_m,imports)=>'const {'+imports.replace(/\bas\b/g,':')+'}=Vue;').replace('export function render','function render');
  const render=Function('Vue',code+';return render;')(Vue);
  return renderToString(Vue.createSSRApp({components,setup:()=>state,render}));
 }};
}
export const memoryStorage=()=>{const values=new Map();return {getItem:k=>values.get(k)??null,setItem:(k,v)=>values.set(k,String(v)),removeItem:k=>values.delete(k),values};};
export const tick=()=>new Promise(resolve=>setImmediate(resolve));
