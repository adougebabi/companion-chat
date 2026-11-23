import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {act,installDOM,mountReact} from './helpers/react-dom.mjs';

const base=process.env.FLUCTLIGHT_M4_HTTP_BASE;
assert.equal(new URL(base).hostname,'127.0.0.1');
const environment=installDOM();
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),envFile:false,appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
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
  async function readDownload(root){
    const caption=button(root,'原始记忆记录（诊断）');
    assert.ok(caption);
    await view.click(caption);
    const create=URL.createObjectURL,revoke=URL.revokeObjectURL;
    const anchor=window.HTMLAnchorElement.prototype,click=anchor.click;
    let blob;
    try {
      URL.createObjectURL=value=>{blob=value;return 'blob:memory-http-fixture';};
      URL.revokeObjectURL=()=>{};
      anchor.click=function(){assert.equal(this.download,'fluctlight-record.json');};
      await view.click(button(root,'下载原始记录'));
      assert.ok(blob);
      return JSON.parse(await blob.text());
    } finally {
      URL.createObjectURL=create;
      URL.revokeObjectURL=revoke;
      anchor.click=click;
    }
  }
  const historyRow=(label,marker)=>[...dialog().querySelectorAll('.lp-history-item')]
    .find(node=>node.querySelector('h4').textContent===label&&node.textContent.includes(marker));
  async function assertCurrentSource(){
    const validSource=()=>[...dialog().querySelectorAll('.lp-source-card')]
      .find(node=>node.textContent.includes('来源有效'));
    await wait(()=>dialog().textContent.includes('当前记忆的来源')&&validSource());
    const row=await readDownload(validSource());
    assert.equal(row.memory_revision,1);
    assert.equal(row.live,true);
  }
  view=await mount();
  await wait(()=>view.host.querySelector('[aria-label^="查看记忆 M4_MEMORY_ORIGINAL"]'));
  await view.click(view.host.querySelector('[aria-label^="查看记忆 M4_MEMORY_ORIGINAL"]'));
  await wait(()=>button(dialog(),'记错了，纠正一下'));
  await view.click(button(dialog(),'记错了，纠正一下'));
  await view.input(dialog().querySelector('[aria-label="记忆修正正文"]'),'M4_MEMORY_CORRECTED 用户喜欢在窗边听雨。');
  await view.input(dialog().querySelector('[aria-label="记忆操作说明"]'),'补充原来遗漏的情境');
  await view.click(button(dialog(),'保存纠正'));
  await wait(()=>historyRow('当前记录','M4_MEMORY_CORRECTED'));
  const corrected=await readDownload(historyRow('当前记录','M4_MEMORY_CORRECTED'));
  assert.equal(corrected.revision,1);
  assert.match(corrected.content,/M4_MEMORY_CORRECTED/);
  await tab('原始来源');
  await assertCurrentSource();
  await tab('修订记录');
  await wait(()=>historyRow('历史修订','M4_MEMORY_ORIGINAL'));
  const old=historyRow('历史修订','M4_MEMORY_ORIGINAL');
  const original=await readDownload(old);
  assert.equal(original.revision,0);
  assert.match(original.content,/M4_MEMORY_ORIGINAL/);
  await view.click(button(old,'查看此版来源'));
  await wait(()=>dialog().textContent.includes('这次历史修订的来源')&&dialog().querySelector('.lp-source-card'));
  assert.equal((await readDownload(dialog().querySelector('.lp-source-card'))).memory_revision,0);
  await view.dispose();
  view=await mount();
  await wait(()=>view.host.querySelector('[aria-label^="查看记忆 M4_MEMORY_CORRECTED"]'));
  await view.click(view.host.querySelector('[aria-label^="查看记忆 M4_MEMORY_CORRECTED"]'));
  await tab('原始来源');
  await assertCurrentSource();
  console.log('M4_MEMORY_UI corrected_revision=1 reloaded=true current_and_historical_sources_read=true');
}finally{
  if(view)await view.dispose();
  await vite.close();environment.restore();
}
