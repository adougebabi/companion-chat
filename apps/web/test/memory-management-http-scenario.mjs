import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {act,installDOM,mountReact} from './helpers/react-dom.mjs';

const base=process.env.FLUCTLIGHT_M4_HTTP_BASE;
assert.equal(new URL(base).hostname,'127.0.0.1');
const environment=installDOM();
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
let view;
try {
  const {BrowserClient}=await vite.ssrLoadModule('/@fs/'+fileURLToPath(new URL('../../../packages/browser-client/src/index.ts',import.meta.url)));
  const {default:Memory}=await vite.ssrLoadModule('/src/react-app/pages/MemoryPage.jsx');
  const client=new BrowserClient(base,(url,options={})=>{
    const headers=new Headers(options.headers);
    headers.set('cookie','fluctlight_session=session; fluctlight_csrf=csrf');
    headers.set('origin','http://fluctlight.local');
    headers.set('x-csrf-token','csrf');
    return fetch(url,{...options,headers});
  });
  const state=proxy({session:{actorId:'human-1'},selectedId:'m4-reflection',characters:[{id:'m4-reflection',status:'active'}]});
  const workspace={state,client,current:{get value(){return state.characters[0];}},subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),isCurrent:(owner,fid)=>owner==='human-1'&&fid==='m4-reflection',navigatePath(){},services:{}};
  const mount=()=>mountReact(vite,Memory,{workspace,props:{route:{query:new URLSearchParams()}}});
  const button=(root,text)=>[...root.querySelectorAll('button')].find(node=>node.textContent.trim()===text);
  const dialog=()=>document.querySelector('[role=dialog]');
  const wait=async predicate=>{
    for(let attempt=0;attempt<100;attempt++){
      let ready;await act(async()=>{ready=await predicate();});
      if(ready)return;
      await view.flush();
    }
    throw Error('Memory page did not reach expected state: '+document.body.textContent);
  };
  const tab=async text=>{
    const target=[...dialog().querySelectorAll('[role=tab]')].find(node=>node.textContent.trim().startsWith(text));
    assert.ok(target,'missing tab '+text);
    await act(async()=>target.dispatchEvent(new window.MouseEvent('mousedown',{bubbles:true,button:0})));
    await view.flush();
  };
  view=await mount();
  await wait(()=>view.host.querySelector('[aria-label^="查看记忆 M4_MEMORY_ORIGINAL"]'));
  await view.click(view.host.querySelector('[aria-label^="查看记忆 M4_MEMORY_ORIGINAL"]'));
  await wait(()=>button(dialog(),'记错了，纠正一下'));
  await view.click(button(dialog(),'记错了，纠正一下'));
  await view.input(dialog().querySelector('[aria-label="记忆修正正文"]'),'M4_MEMORY_CORRECTED 用户喜欢在窗边听雨。');
  await view.input(dialog().querySelector('[aria-label="记忆操作说明"]'),'补充原来遗漏的情境');
  await view.click(button(dialog(),'保存纠正'));
  await wait(()=>dialog().textContent.includes('第 1 版')&&dialog().textContent.includes('M4_MEMORY_CORRECTED'));
  await tab('原始来源');
  await wait(()=>dialog().textContent.includes('第 1 版的来源')&&dialog().textContent.includes('来源有效'));
  await tab('修订记录');
  await wait(()=>[...dialog().querySelectorAll('.lp-history-item')].some(node=>node.textContent.includes('第 0 版')));
  const old=[...dialog().querySelectorAll('.lp-history-item')].find(node=>node.textContent.includes('第 0 版'));
  await view.click(button(old,'查看此版来源'));
  await wait(()=>dialog().textContent.includes('第 0 版的来源'));
  await view.dispose();
  view=await mount();
  await wait(()=>view.host.querySelector('[aria-label^="查看记忆 M4_MEMORY_CORRECTED"]'));
  await view.click(view.host.querySelector('[aria-label^="查看记忆 M4_MEMORY_CORRECTED"]'));
  await tab('原始来源');
  await wait(()=>dialog().textContent.includes('第 1 版的来源')&&dialog().textContent.includes('来源有效'));
  console.log('M4_MEMORY_UI corrected_revision=1 reloaded=true current_and_historical_sources_read=true');
}finally{
  if(view)await view.dispose();
  await vite.close();environment.restore();
}
