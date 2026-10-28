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
   try{assert.equal(calls[0].fluctlightId,filter||undefined);assert.equal(calls[0].scope,'all');await view.click([...view.host.querySelectorAll('button')].find(b=>b.textContent.trim()==='来源、角色与库中展示'));assert.equal(view.host.querySelector('[role=combobox][aria-label="媒体角色范围"]').textContent.trim(),filter?'已指定角色范围（名称待读取）':'我的全部素材');assert.match(view.host.textContent,/添加素材/);}finally{await view.dispose();}
  }
 }finally{await vite.close();environment.restore();}
});
