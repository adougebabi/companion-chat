import assert from 'node:assert/strict';
import test from 'node:test';
import { readProduct } from '../src/v3/read-product.mjs';
import { viewFromBundle } from '../src/v3/contract-map.mjs';

test('actual Go detail fields retain owner scope and stable media/message identities', () => {
  const view = viewFromBundle({ session: { authenticated: true, actorId: 'human-real' },
    fluctlights: [{ id: 'fl-real', status: 'active', identity: { name: '测试角色' } }],
    details: { 'fl-real': { context: { location: '旧文本地点', activity: '阅读', source: 'event' },
      schedule: { items: [{ id: 'schedule-real', activity: '阅读', status: 'accepted', start_at: '2026-10-15T10:00:00Z', end_at: '2026-10-15T11:00:00Z' }] },
      actor_user: { background: { name: '当前用户' } },
      memories: [{ id: 'memory-real', owner_fluctlight_id: 'fl-real', content: '真实来源', status: 'active', type: 'semantic', revision: 7 }],
      relationships: [{ id: 'rel-real', target_actor_id: 'human-real', target_actor_type: 'human', summary: '有来源的关系', revision: 4, metrics: {} }],
    } },
    wardrobes: { 'fl-real': { items: [], worn_items: [], used_items: [], revision: 3 } },
    media: { 'fl-real': { items: [{ id: 'asset-one', url: '/api/media/asset-one' }, { id: 'asset-two', url: '/api/media/asset-two' }] } },
    conversations: { 'fl-real': { conversation: { id: 'conversation-real', createdByActorId: 'human-real' }, messages: [{ id: 'msg-real', authorActorId: 'human-real', kind: 'user', text: '你好' }] } },
  });
  assert.equal(view.characters['fl-real'].status, '正常');
  assert.equal(view.characters['fl-real'].locationText, '旧文本地点');
  assert.equal(view.schedule['schedule-real'].charId, 'fl-real');
  assert.equal(view.memories['memory-real'].version, 7);
  assert.equal(view.relations['rel-real'].charId, 'fl-real');
  assert.equal(view.relations['rel-real'].npcId, 'human-real');
  assert.deepEqual(Object.keys(view.media), ['asset-one', 'asset-two']);
  assert.equal(view.messages['msg-real'].from, 'user');
  assert.deepEqual(view.outfit['fl-real'].current, []);
  assert.deepEqual(view.user.background, {});
  assert.equal(view.characters['fl-real'].mood, undefined);
});

test('an optional 502 preserves successful modules and differs from unavailable and empty', async () => {
  const client = new Proxy({
    session: async () => ({ authenticated: true, actorId: 'human-real' }),
    listFluctlights: async () => [{ id: 'fl-real', status: 'active', identity: { name: '测试角色' } }],
    listActorGroups: async () => [], moments: async () => [],
    detail: async () => ({ id: 'fl-real', memories: [] }),
    listCharacterMedia: async () => { throw Object.assign(new Error('媒体服务不可用'), { status: 502, code: 'media_unavailable' }); },
    world: async () => ({ available: false, status: 'not_implemented' }),
    listDiary: async () => ({ items: [], next_cursor: null }),
  }, { get: (target, key) => target[key] || (async () => ({})) });
  const bundle = await readProduct(client);
  assert.equal(bundle.details['fl-real'].id, 'fl-real');
  assert.equal(bundle.surfaces['fl-real:media'].httpStatus, 502);
  assert.equal(bundle.surfaces['fl-real:worlds'].status, 'unavailable');
  assert.equal(bundle.surfaces['fl-real:diaries'].status, 'ready');
  assert.deepEqual(bundle.diaries['fl-real'].items, []);
  assert.equal(bundle.media['fl-real'], undefined);
});

test('an authentication transport failure is not represented as logged out', async () => {
  await assert.rejects(readProduct({ session: async () => { throw new Error('Core unavailable'); } }), /Core unavailable/);
});
