import assert from 'node:assert/strict';
import {createServer} from './support/module-server.mjs';
import {fileURLToPath} from 'node:url';
import {proxy, snapshot, subscribe} from 'valtio/vanilla';
import {act, installDOM, mountReact} from './helpers/react-dom.mjs';

const base = process.env.FLUCTLIGHT_M4_HTTP_BASE;
assert.equal(new URL(base).hostname, '127.0.0.1');
const phase = process.argv[2];
assert.ok(['enable', 'disable'].includes(phase));
const environment = installDOM();
const vite = await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
let view;
try {
  const {BrowserClient} = await vite.ssrLoadModule('/@fs/' + fileURLToPath(new URL('../../../packages/browser-client/src/index.ts',import.meta.url)));
  const {createSelfProvider} = await vite.ssrLoadModule('/src/v3/providers/self.ts');
  const {default:Panel} = await vite.ssrLoadModule('/src/react-app/pages/ExpressionLearningPanel.tsx');
  const client = new BrowserClient(base, (url,options={}) => {
    const headers = new Headers(options.headers);
    headers.set('cookie','fluctlight_session=session; fluctlight_csrf=csrf');
    headers.set('origin','http://fluctlight.local');
    headers.set('x-csrf-token','csrf');
    return fetch(url,{...options,headers});
  });
  const state = proxy({session:{actorId:'human-1'},characters:[{id:'m4-reflection',status:'active'}]});
  const workspace = {state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),services:{selfClient:createSelfProvider(client)}};
  const button = text => [...document.querySelectorAll('button')].find(node=>node.textContent.trim()===text);
  const field = label => {
    const node = [...document.querySelectorAll('label')].find(node => node.textContent.trim() === label);
    assert.ok(node, 'Missing labeled field '+label);
    return document.getElementById(node.htmlFor);
  };
  const wait = async predicate => {
    for (let attempt=0;attempt<100;attempt++) {
      let ready;
      await act(async()=>{ready=await predicate();});
      if (ready) return;
      await view.flush();
    }
    throw Error('Expression page did not reach expected state: '+document.body.textContent);
  };
  const confirm = async () => {
    await wait(()=>button('确认提交')&&!button('确认提交').disabled);
    await view.click(button('确认提交'));
    await wait(()=>!document.querySelector('[role=dialog]'));
  };
  view = await mountReact(vite,Panel,{workspace,props:{fid:'m4-reflection'}});
  if (phase==='enable') {
    await wait(()=>button('学习：已关闭'));
    await view.click(button('学习：已关闭'));await confirm();
    await wait(()=>button('参考：已关闭'));
    await view.click(button('参考：已关闭'));await confirm();
    await view.click([...view.host.querySelectorAll('summary')].find(node=>node.textContent==='选择真实消息提取候选'));
    await wait(()=>button('从这条提取'));
    await view.click(button('从这条提取'));await confirm();
    await wait(()=>button('审核并启用'));
    await view.click(button('审核并启用'));
    await view.input(field('适用触发词'),'进度');
    await view.input(field('审核理由'),'Synthetic approved progress expression');
    await confirm();
  } else {
    await wait(()=>button('停用'));
    await view.click(button('停用'));
    await view.input(field('审核理由'),'Synthetic withdrawal of expression reference');
    await confirm();
  }
  const expected = phase==='enable'?'使用中':'已停用';
  await wait(()=>view.host.textContent.includes(expected));
  await view.dispose();
  view = await mountReact(vite,Panel,{workspace,props:{fid:'m4-reflection'}});
  await wait(()=>view.host.textContent.includes(expected));
  assert.match(view.host.textContent,/慢慢来呀/);
  console.log('M4_EXPRESSION_UI',phase,'RELOADED_STATUS',expected);
} finally {
  if (view) await view.dispose();
  await vite.close();environment.restore();
}
