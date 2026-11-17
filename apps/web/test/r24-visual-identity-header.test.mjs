import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
import {createServer} from 'vite';
import {installDOM, mountReact} from './helpers/react-dom.mjs';
import {mediaFixture} from './fixtures/media-fixture.mjs';

const env = installDOM();
const vite = await createServer({
  root: fileURLToPath(new URL('../', import.meta.url)),
  envFile: false, appType: 'custom', logLevel: 'silent', server: {middlewareMode: true},
});
test.after(async () => {await vite.close(); env.restore();});

test('视觉身份页头：标题说明与三个操作分离，重新读取只发起读取', async () => {
  const {createWorkspace} = await vite.ssrLoadModule('/src/state/workspace.mjs');
  const store = await vite.ssrLoadModule('/src/v3/proto/store.js');
  const {default: Media} = await vite.ssrLoadModule('/src/react-app/pages/MediaPage.tsx');
  const api = mediaFixture({scenario: 'empty'}), w = createWorkspace();
  store.replaceSnapshot({...store.get(), session: {actorId: 'media-owner'}, characters: {A: {id: 'A', name: '林青', status: 'active'}}, ui: {charId: 'A'}});
  w.client.fetcher = async (input, init = {}) => {
    assert.equal(init.method || 'GET', 'GET', '页头展示与刷新不得触发生成或保存');
    const url = new URL(input, 'http://media.test');
    let response;
    assert.ok(await api.handler({url, path: url.pathname, method: 'GET', body: {}, json: (_res, row, status = 200) => {response = Response.json(row, {status});}}));
    return response;
  };
  const v = await mountReact(vite, Media, {workspace: w, props: {route: {path: '/c/A/media/identity', fid: 'A', query: new URLSearchParams()}, initialTab: 'identity'}});
  try {
    const header = v.host.querySelector('.r-toolbar.r-identity-header');
    const copy = header.querySelector(':scope > .r-grow');
    assert.equal(copy.querySelector('h2').textContent, '同一个人，越来越清晰');
    assert.equal(copy.querySelector('p').textContent, '规范参考、候选与评审，各自留下真实记录。');
    assert.equal(copy.querySelector('button'), null);
    const buttons = [...header.querySelectorAll(':scope > button')];
    assert.deepEqual(buttons.map(node => node.textContent.trim()), ['编辑视觉定义', '生成新候选', '重新读取']);
    assert.ok(buttons.every(node => node.dataset.slot === 'button'));
    assert.doesNotMatch(header.textContent, /media-owner|Internal Server Error|fluctlight_id/);
    const before = api.calls.length;
    await v.click(buttons[2]);
    assert.ok(api.calls.length > before);
  } finally {await v.dispose(); w.dispose();}
});

test('视觉身份页头仅在窄屏让标题区独占整行，桌面沿用原工具栏布局', async () => {
  const style = document.createElement('style');
  style.textContent = await readFile(new URL('../src/styles/replica-media.css', import.meta.url), 'utf8');
  document.head.append(style);
  try {
    const rules = [...style.sheet.cssRules];
    const mobile = rules.find(rule => rule.conditionText === '(max-width: 700px)');
    assert.ok(mobile);
    const toolbar = [...mobile.cssRules].find(rule => rule.selectorText === '.replica-media .r-toolbar');
    assert.equal(toolbar.style.getPropertyValue('flex-wrap'), 'wrap');
    const title = [...mobile.cssRules].find(rule => rule.selectorText === '.replica-media .r-identity-header > .r-grow');
    assert.equal(title.style.getPropertyValue('flex'), '0 0 100%');
    assert.equal(title.style.getPropertyValue('min-width'), '0px');
    assert.equal(rules.some(rule => rule.selectorText?.includes('r-identity-header')), false);
  } finally {style.remove();}
});
