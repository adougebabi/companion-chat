import test from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'vite';
import { fileURLToPath } from 'node:url';
import { act, installDOM, mountReact } from './helpers/react-dom.mjs';
import { creationFixture } from './support/creation-fixture.mjs';
import { worldFixture } from './fixtures/world-fixture.mjs';

const env = installDOM();
const vite = await createServer({ root: fileURLToPath(new URL('../', import.meta.url)), appType: 'custom', logLevel: 'silent', server: { middlewareMode: true } });
const { default: Wizard } = await vite.ssrLoadModule('/src/react-app/views/CharacterWizard.jsx');
const { default: World } = await vite.ssrLoadModule('/src/react-app/pages/WorldPage.jsx');
const { createWorkspace } = await vite.ssrLoadModule('/src/state/workspace.mjs');
const store = await vite.ssrLoadModule('/src/v3/proto/store.js');
test.after(async () => { await vite.close(); env.restore(); });
const button = (root, text) => [...root.querySelectorAll('button')].find(node => node.textContent.trim() === text);

function wizardModel(c, extra = {}) {
  return { c, draft: c.draft, step: 'world', currentStep: 0, busy: false, pending: null,
    previewFields: [], error: '', fieldErrors: {}, groups: [], existingWorlds: [], worlds: [], templates: [],
    worldsConfirmed: true, publishedWorldsConfirmed: true, canContinue: false, goStep() {}, configHref: () => '#/settings', ...extra };
}

test('existing worlds invalidate a retained daily-world preview until a real existing source is selected', async () => {
  const f = creationFixture();
  // This saved choice was valid when the directory was empty. It must not
  // masquerade as a current selection after another world becomes available.
  f.c.draft.worldName = '过期的日常世界名称';
  const existing = { id: 'existing', name: '溪边小镇', fluctlightId: 'other', characterName: '雨禾', revision: 1 };
  const m = wizardModel(f.c, { existingWorlds: [existing] });
  const v = await mountReact(vite, Wizard, { props: { model: m } });
  try {
    const preview = () => v.host.querySelector('.creation-aside');
    assert.equal(preview().querySelector('h3').textContent, '待选择');
    assert.match(preview().textContent, /生活世界待选择/);
    assert.doesNotMatch(v.host.textContent, /过期的日常世界名称|将为 TA 准备一个日常生活世界|已明确选择/);
    assert.equal(v.host.querySelectorAll('[role=radio][aria-checked=true]').length, 0);
    assert.equal(button(v.host, '继续，认识角色').disabled, true);
    assert.equal(f.c.draft.useDefaultWorld, true, 'presentation must preserve the retained recovery draft');
    f.c.selectExistingWorld(existing);
    await v.update({ model: { ...m, canContinue: true } });
    assert.equal(preview().querySelector('h3').textContent, '溪边小镇');
    assert.match(v.host.textContent, /将加入「溪边小镇」的同一世界/);
    assert.equal(v.host.querySelectorAll('[role=radio][aria-checked=true]').length, 1);
    assert.equal(button(v.host, '继续，认识角色').disabled, false);
  } finally { await v.dispose(); f.c.dispose(); }
});

test('wizard step changes reset the actual workspace scroll owner and focus the new heading', async () => {
  const f = creationFixture(), m = wizardModel(f.c);
  const v = await mountReact(vite, Wizard, { props: { model: m } });
  try {
    v.host.classList.add('workspace-page');
    v.host.scrollTop = 560;
    const scrolls = [];
    v.host.scrollTo = options => { scrolls.push(options); v.host.scrollTop = options.top; };
    await v.update({ model: { ...m, currentStep: 1, step: 'person' } });
    assert.equal(scrolls.at(-1).top, 0);
    assert.equal(v.host.scrollTop, 0);
    assert.equal(document.activeElement, v.host.querySelector('h1'));
    assert.equal(v.host.querySelector('h1').textContent, '你想怎样认识这个人？');
    assert.equal(v.host.querySelectorAll('nav[aria-label="创建步骤"] button').length, 4);
  } finally { await v.dispose(); f.c.dispose(); }
});

test('an unavailable retained published-world choice also stays pending when existing worlds require a new choice', async () => {
  const f = creationFixture();
  Object.assign(f.c.draft, { useDefaultWorld: false, worldResourceId: 'old-resource', worldResourceVersion: 'v2', worldName: '旧的世界底稿' });
  const m = wizardModel(f.c, { existingWorlds: [{ id: 'existing', name: '溪边小镇', fluctlightId: 'other', characterName: '雨禾', revision: 1 }] });
  const v = await mountReact(vite, Wizard, { props: { model: m } });
  try {
    assert.equal(v.host.querySelector('.creation-aside h3').textContent, '待选择');
    assert.doesNotMatch(v.host.textContent, /旧的世界底稿|本次固定使用/);
    assert.equal(v.host.querySelectorAll('[role=radio][aria-checked=true]').length, 0);
    assert.equal(button(v.host, '继续，认识角色').disabled, true);
    assert.equal(f.c.draft.worldResourceId, 'old-resource');
    assert.equal(f.c.draft.worldResourceVersion, 'v2');
  } finally { await v.dispose(); f.c.dispose(); }
});

async function worldUI() {
  sessionStorage.clear();
  const api = worldFixture({ owner: 'r34c-world-owner', projectionReads: true });
  const w = createWorkspace();
  store.replaceSnapshot({ ...store.get(), session: { actorId: 'r34c-world-owner' }, characters: { A: { id: 'A', name: '林青', status: 'active' } }, ui: { charId: 'A' } });
  w.client.fetcher = async (input, init = {}) => {
    const url = new URL(input, 'http://world.test');
    let result;
    const handled = await api.handler({ url, path: url.pathname, method: init.method || 'GET', body: init.body ? JSON.parse(init.body) : {},
      json: (_res, value, status = 200) => (result = new Response(JSON.stringify(value), { status, headers: { 'content-type': 'application/json' } })) });
    assert.ok(handled, 'unexpected fixture request: ' + url.pathname);
    return result;
  };
  const v = await mountReact(vite, World, { workspace: w });
  return { v, w, api, async dispose() { await v.dispose(); w.dispose(); } };
}
async function tab(v, name) {
  await act(async () => [...v.host.querySelectorAll('[role=tab]')].find(node => node.textContent.trim().startsWith(name))
    .dispatchEvent(new window.MouseEvent('mousedown', { bubbles: true, button: 0, ctrlKey: false })));
  await v.flush();
}

test('world directory keeps one creation entry, including after an empty search', async () => {
  const f = await worldUI();
  try {
    assert.ok(button(f.v.host, '新建世界'));
    assert.equal(button(f.v.host, '世界底稿'), undefined);
    assert.equal(f.v.host.querySelector('.r-create-world'), null);
    await f.v.input(f.v.host.querySelector('[aria-label="搜索世界名称或类型"]'), '不存在的世界');
    assert.match(f.v.host.textContent, /没有找到匹配的世界/);
    assert.equal(f.v.host.querySelectorAll('[data-testid^=world-card-]').length, 0);
    assert.equal(f.v.host.querySelector('.r-create-world'), null);
    assert.ok(button(f.v.host, '新建世界'));
    assert.equal(f.api.calls.filter(call => call.method !== 'GET').length, 0);
  } finally { await f.dispose(); }
});

test('world detail keeps its directory name and offers people and event preparation in an unbound world', async () => {
  const f = await worldUI(), moves = [];
  f.w.navigatePath = path => moves.push(path);
  try {
    const card = f.v.host.querySelector('[aria-label="进入世界 远山森林"]');
    assert.ok(card);
    await f.v.click(card);
    assert.equal(f.v.host.querySelector('h1').textContent, '远山森林');
    assert.equal(f.v.host.querySelector('.r-world-hero h2').textContent, '远山森林');
    assert.doesNotMatch(f.v.host.textContent, /一座属于日常的世界/);
    await tab(f.v, '世界设定');
    assert.match(f.v.host.textContent, /这个世界还没有已读人物/);
    await f.v.click(button(f.v.host, '创建人物底稿'));
    assert.equal(moves.at(-1), '/studio?kind=npc&new=1');
    await tab(f.v, '世界事件');
    assert.match(f.v.host.textContent, /事件列表随当前运行世界读取/);
    await f.v.click(button(f.v.host, '创建事件底稿'));
    assert.equal(moves.at(-1), '/studio?kind=event_template&new=1');
    assert.equal(f.api.state('A').binding.world_id, 'A-world-one');
    assert.equal(f.api.calls.filter(call => call.method !== 'GET').length, 0);
    assert.equal(f.api.calls.some(call => call.path.endsWith('/world/event-templates')), false);
  } finally { await f.dispose(); }
});
