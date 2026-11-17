import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {act, installDOM, mountReact} from './helpers/react-dom.mjs';
import {mediaFixture} from './fixtures/media-fixture.mjs';

const env = installDOM();
const vite = await createServer({
  root: fileURLToPath(new URL('../', import.meta.url)),
  envFile: false, appType: 'custom', logLevel: 'silent', server: {middlewareMode: true},
});
test.after(async () => {await vite.close(); env.restore();});
const button = (root, text) => [...root.querySelectorAll('button')].find(node => node.textContent.trim() === text);
const dialog = () => document.querySelector('[role=dialog]');
const emptyText = /还没有保存过视觉定义/;
const forbidden = /private-|sql:|Internal Server Error|fluctlight_id|schema_version/;

async function openHistory({scoped = false, read} = {}) {
  const {createWorkspace} = await vite.ssrLoadModule('/src/state/workspace.mjs');
  const store = await vite.ssrLoadModule('/src/v3/proto/store.js');
  const {default: Media} = await vite.ssrLoadModule('/src/react-app/pages/MediaPage.tsx');
  const api = mediaFixture({scenario: 'empty'});
  const w = createWorkspace();
  store.replaceSnapshot({...store.get(), session: {actorId: 'media-owner'}, characters: {A: {id: 'A', name: '林青', status: 'active'}}, ui: {charId: 'A'}});
  const historyCalls = [];
  w.client.fetcher = async (input, init = {}) => {
    assert.equal(init.method || 'GET', 'GET', '查看与重试只能读取，不触发保存或生成');
    const url = new URL(input, 'http://media.test');
    if (url.pathname.endsWith('/visual-brief/history')) {
      historyCalls.push(url.searchParams.get('cursor'));
      return Response.json(await read(url, historyCalls.length));
    }
    let response;
    const handled = await api.handler({url, path: url.pathname, method: 'GET', body: {}, json: (_res, row, status = 200) => {response = Response.json(row, {status});}});
    assert.ok(handled, url.pathname);
    return response;
  };
  const view = await mountReact(vite, Media, {workspace: w, props: {route: {path: scoped ? '/c/A/media/identity' : '/media', fid: scoped ? 'A' : undefined, query: new URLSearchParams()}, initialTab: scoped ? 'identity' : undefined}});
  if (!scoped) {
    await act(async () => [...view.host.querySelectorAll('[role=tab]')].find(node => node.textContent.startsWith('视觉身份')).dispatchEvent(new window.MouseEvent('mousedown', {bubbles: true, button: 0})));
    await view.flush();
  }
  assert.equal(historyCalls.length, 0, '弹窗打开前不读取修订');
  await view.click(button(view.host, '定义修订'));
  return {view, historyCalls, async dispose() {await view.dispose(); w.dispose();}};
}

for (const scoped of [false, true]) test(`${scoped ? '角色媒体' : '全局媒体'}：空修订显示共享空状态和保存说明`, async () => {
  const context = await openHistory({scoped, read: async () => ({items: [], next_cursor: ''})});
  try {
    const panel = dialog();
    assert.match(panel.textContent, /视觉定义修订/);
    assert.match(panel.querySelector('[data-empty-variant]').textContent, emptyText);
    assert.match(panel.textContent, /保存视觉定义后，每次实际保存的参考说明会显示在这里/);
    assert.doesNotMatch(panel.textContent, /正在读取|暂未读全/);
    assert.doesNotMatch(panel.textContent, forbidden);
    assert.deepEqual(context.historyCalls, ['']);
  } finally {await context.dispose();}
});

test('读取期间显示加载态，完成后才显示空状态', async () => {
  let finish;
  const pending = new Promise(resolve => {finish = resolve;});
  const context = await openHistory({read: () => pending});
  try {
    assert.match(dialog().textContent, /正在读取定义修订/);
    assert.doesNotMatch(dialog().textContent, emptyText);
    await act(async () => finish({items: [], next_cursor: ''}));
    await context.view.flush();
    assert.match(dialog().textContent, emptyText);
    assert.doesNotMatch(dialog().textContent, /正在读取/);
  } finally {await context.dispose();}
});

test('读取失败显示中文错误且隐藏原始诊断，重新读取后恢复', async () => {
  const context = await openHistory({read: async (_url, count) => {
    if (count === 1) throw Error('Internal Server Error sql: private-id');
    return {items: [], next_cursor: ''};
  }});
  try {
    assert.match(dialog().querySelector('[role=alert]').textContent, /暂时无法读取视觉定义修订，请重新读取/);
    assert.doesNotMatch(dialog().textContent, emptyText);
    assert.doesNotMatch(dialog().textContent, forbidden);
    assert.equal(dialog().querySelector('pre'), null);
    await context.view.click(button(dialog(), '重新读取定义修订'));
    assert.deepEqual(context.historyCalls, ['', '']);
    assert.match(dialog().textContent, emptyText);
    assert.equal(dialog().querySelector('[role=alert]'), null);
  } finally {await context.dispose();}
});

test('有修订时显示版本列表，翻页失败保留列表并重试原游标', async () => {
  const context = await openHistory({read: async (url, count) => {
    if (count === 2) throw Error('Internal Server Error private-cursor');
    return url.searchParams.get('cursor')
      ? {items: [{id: 'private-older', revision: 1, reason: '首次保存参考说明'}], next_cursor: ''}
      : {items: [{id: 'private-newer', revision: 2, reason: '更新发型参考'}], next_cursor: 'private-next'};
  }});
  try {
    assert.match(dialog().textContent, /视觉定义 v2/);
    assert.match(dialog().textContent, /更新发型参考/);
    assert.doesNotMatch(dialog().textContent, emptyText);
    await context.view.click(button(dialog(), '加载更多定义修订'));
    assert.match(dialog().textContent, /视觉定义 v2/);
    assert.match(dialog().textContent, /暂时无法读取/);
    assert.doesNotMatch(dialog().textContent, forbidden);
    await context.view.click(button(dialog(), '重新读取定义修订'));
    assert.match(dialog().textContent, /视觉定义 v2/);
    assert.match(dialog().textContent, /视觉定义 v1/);
    assert.deepEqual(context.historyCalls, ['', 'private-next', 'private-next']);
    assert.doesNotMatch(dialog().textContent, forbidden);
    assert.doesNotMatch(dialog().textContent, emptyText);
  } finally {await context.dispose();}
});
