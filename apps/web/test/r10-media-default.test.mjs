import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {installDOM,mountReact} from './helpers/react-dom.mjs';
test('React media defaults to every character even from a bookmark and preserves explicit filtering',async()=>{
 const environment=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
 try{const {default:Media}=await vite.ssrLoadModule('/src/react-app/pages/MediaPage.tsx');
  for(const filter of ['', 'other-role']){
   const calls=[],state=proxy({session:{actorId:'owner'},characters:[]});
   const w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),services:{mediaClient:{list:async options=>{calls.push(options);return {items:[],next_cursor:''};}}}};
   const view=await mountReact(vite,Media,{workspace:w,props:{route:{fid:'selected-role',query:new URLSearchParams(filter?{fid:filter}:{})}}});
   try{assert.equal(calls[0].fluctlightId,filter||undefined);assert.equal(calls[0].scope,'all');assert.match(view.host.textContent,/全部角色/);assert.match(view.host.textContent,/上传素材/);}finally{await view.dispose();}
  }
 }finally{await vite.close();environment.restore();}
});
