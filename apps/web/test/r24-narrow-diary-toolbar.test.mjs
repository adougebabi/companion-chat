import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {createServer} from './support/module-server.mjs';
import {fileURLToPath} from 'node:url';
import {act, installDOM, mountReact} from './helpers/react-dom.mjs';
import {diaryToolbarWorkspace} from './fixtures/diary-toolbar-fixture.mjs';

const env = installDOM();
const vite = await createServer({root:fileURLToPath(new URL('../', import.meta.url)), envFile:false, appType:'custom', logLevel:'silent', server:{middlewareMode:true}});
const {default:Diary} = await vite.ssrLoadModule('/src/react-app/pages/DiaryPage.jsx');
test.after(async () => {await vite.close(); env.restore();});

test('日记工具栏使用独立搜索框、带文字的收藏开关及独立刷新按钮', async () => {
  const workspace = diaryToolbarWorkspace();
  const view = await mountReact(vite, Diary, {workspace});
  try {
    const toolbar = view.host.querySelector('.lp-toolbar');
    const search = toolbar.querySelector('.replica-diary-search');
    const input = search.querySelector('input');
    const toggle = toolbar.querySelector('[role=switch][aria-label="只看收藏"]');
    const refresh = toolbar.querySelector('[aria-label="刷新日记"]');
    assert.equal(input.placeholder, '在日记中寻找…');
    assert.equal(search.querySelector('button'), null, '输入框内没有收藏或刷新按钮');
    assert.equal(toggle.closest('label').textContent.trim(), '只看收藏');
    assert.equal(toggle.closest('.replica-diary-controls'), search.parentElement);
    assert.equal(refresh.parentElement, search.parentElement);
    assert.equal(toolbar.querySelector('[aria-label="日记筛选"]'), null);
    await view.input(input, '今天的回忆');
    await view.click(toggle);
    assert.equal(toggle.getAttribute('aria-checked'), 'true');
    assert.equal(input.value, '今天的回忆');
    assert.deepEqual(workspace.calls.at(-1), {kind:'diary', favorite:true});
    const count = workspace.calls.length;
    await view.click(refresh);
    assert.equal(workspace.calls.length, count + 1);
    assert.deepEqual(workspace.calls.at(-1), {kind:'diary', favorite:true});
    await act(async () => [...toolbar.querySelectorAll('[role=tab]')].find(n => n.textContent === '共同经历').dispatchEvent(new window.MouseEvent('mousedown', {bubbles:true, button:0})));
    await view.flush();
    assert.equal(toolbar.querySelector('.replica-diary-controls'), null);
    await view.click(toolbar.querySelector('[aria-label="刷新日记"]'));
    assert.deepEqual(workspace.calls.at(-1), {kind:'shared'});
  } finally {await view.dispose();}
});

test('窄屏限定为单列 Tabs、整行搜索和下一行收藏/刷新，桌面保留原布局规则', async () => {
  const css = await readFile(new URL('../src/styles/replica-life.css', import.meta.url), 'utf8');
  const mobile = css.slice(css.indexOf('@media(max-width:600px){\n  .replica-diary .lp-toolbar'), css.indexOf('\n\n.replica-diary .lp-paper h2'));
  assert.match(mobile, /\.lp-toolbar\{[^}]*grid-template-columns:minmax\(0,1fr\)/);
  assert.match(mobile, /\.replica-diary-search\{[^}]*grid-column:1 \/ -1;grid-row:1;[^}]*width:100%/);
  assert.match(mobile, /\.replica-diary-search input\{padding-right:12px\}/);
  assert.match(mobile, /\.replica-diary-options\{grid-column:1;grid-row:2;display:block\}/);
  assert.match(mobile, /\.replica-diary-refresh\{grid-column:2;grid-row:2;position:static;transform:none/);
  assert.match(css, /\.replica-life \.lp-toolbar\{display:flex;justify-content:space-between/);
  assert.match(css, /\.replica-diary-controls \.replica-diary-search\{width:320px\}/);
  assert.doesNotMatch(css, /replica-diary-options\{display:none/);
});
