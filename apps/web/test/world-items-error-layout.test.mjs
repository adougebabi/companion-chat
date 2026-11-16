import test from 'node:test';
import assert from 'node:assert/strict';
import { installDOM } from './helpers/react-dom.mjs';

const env = installDOM();
const store = await import('../src/v3/proto/store.js');
const { configureDomainService } = await import('../src/v3/proto/services.js');
const { renderWorldItems } = await import('../src/v3-domain/world-items.js');
const { renderRealWorldEvents } = await import('../src/v3-domain/world-events.js');
const tick = () => new Promise(resolve => setImmediate(resolve));
const ctx = { charId: 'fl-suqing', query: {} };
const deferred = () => { let resolve, reject; const promise = new Promise((a, b) => { resolve = a; reject = b; }); return { promise, resolve, reject }; };
const item = { id: 'private-item', name: '旅行背包', definition: { description: '轻便的背包', item_kind: 'ordinary' }, available: true };
function fixture(read, worldId = 'private-world') {
  store.replaceSnapshot({ ...store.get(), session: { actorId: 'owner' }, characters: { 'fl-suqing': { id: 'fl-suqing', worldId } } });
  const calls = [];
  configureDomainService('world', { items: (fid, options) => { calls.push({ fid, ...options }); return read(options); } });
  const el = document.createElement('div'); document.body.append(el);
  const dispose = renderWorldItems(el, ctx);
  return { el, calls, dispose() { dispose(); el.remove(); } };
}
function controlsDisabled(el, expected) {
  for (const control of el.querySelectorAll('[data-create], [data-search] input, [data-search] button')) assert.equal(control.disabled, expected);
}
test.after(() => env.restore());

test('no running world reuses the events empty state and makes no directory request', () => {
  const f = fixture(() => { throw Error('must not read'); }, null);
  const events = document.createElement('div'); document.body.append(events);
  const dispose = renderRealWorldEvents(events, ctx);
  try {
    assert.equal(f.el.innerHTML, events.innerHTML);
    assert.match(f.el.textContent, /尚未选择运行世界/);
    assert.equal([...f.el.querySelectorAll('a')].find(a => a.textContent === '创建或选择世界').getAttribute('href'), '#/c/fl-suqing/worlds');
    assert.equal(f.el.querySelector('[data-search], [data-create]'), null);
    assert.equal(f.calls.length, 0);
  } finally { f.dispose(); dispose(); events.remove(); }
});

test('failed read keeps Chinese feedback and retry inside the card, blocks search and creation, then recovers', async () => {
  const first = deferred(), retry = deferred();
  let reads = 0;
  const f = fixture(() => ++reads === 1 ? first.promise : retry.promise);
  try {
    controlsDisabled(f.el, true);
    first.reject(Object.assign(Error('Internal Server Error private-world'), { code: 'private_key' })); await tick();
    const failure = f.el.querySelector('[data-panel-failure]');
    assert.equal(failure.closest('[data-domain-ui="card"]'), f.el.firstElementChild);
    assert.equal(failure.querySelector('[data-panel-retry]').closest('[role="alert"]'), failure);
    assert.match(failure.textContent, /暂时无法读取世界物品目录，请稍后再试。/);
    assert.doesNotMatch(f.el.textContent, /private|Internal Server Error|技术详情/);
    controlsDisabled(f.el, true);
    const form = f.el.querySelector('form');
    form.elements.q.value = '不应提交'; form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    const originalHash = location.hash; f.el.querySelector('[data-create]').click();
    assert.equal(location.hash, originalHash); assert.equal(f.calls.length, 1);
    assert.ok(f.el.querySelector('a[href="#/c/fl-suqing/world"]'));
    failure.querySelector('button').click(); controlsDisabled(f.el, true);
    retry.resolve({ items: [], next_cursor: '' }); await tick();
    controlsDisabled(f.el, false); assert.equal(f.el.querySelector('[data-panel-failure]'), null);
    assert.match(f.el.textContent, /当前世界尚未注册物品/);
  } finally { f.dispose(); }
});

test('search and pagination retain their query and cursor; a later failure hides stale item actions', async () => {
  let fail = false;
  const f = fixture(async () => { if (fail) throw Error('private_error'); return { items: [item], next_cursor: 'opaque-cursor' }; });
  try {
    await tick(); controlsDisabled(f.el, false);
    const form = f.el.querySelector('form'); form.elements.q.value = ' 背包 ';
    form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    controlsDisabled(f.el, true); await tick();
    assert.equal(f.calls.at(-1).q, '背包');
    fail = true; f.el.querySelector('[data-page-next]').click(); await tick();
    assert.equal(f.calls.at(-1).cursor, 'opaque-cursor');
    assert.equal(f.calls.at(-1).q, '背包'); controlsDisabled(f.el, true);
    assert.equal(f.el.querySelector('[data-items] a'), null);
    fail = false; f.el.querySelector('[data-panel-retry]').click(); await tick();
    assert.equal(f.calls.at(-1).cursor, 'opaque-cursor'); controlsDisabled(f.el, false);
    assert.match(f.el.textContent, /旅行背包/);
    assert.equal(f.el.querySelector('[data-panel-failure]'), null);
    assert.ok(f.el.querySelector('[data-page-previous]'));
  } finally { f.dispose(); }
});

test('late reads after leaving the page cannot re-enable controls or show errors', async () => {
  for (const reject of [false, true]) {
    const pending = deferred(), f = fixture(() => pending.promise);
    f.dispose();
    if (reject) pending.reject(Error('private_error')); else pending.resolve({ items: [], next_cursor: '' });
    await tick(); controlsDisabled(f.el, true);
    assert.equal(f.el.querySelector('[data-panel-failure]'), null);
  }
});
