import assert from 'node:assert/strict';
import test from 'node:test';
import { readFile, readdir } from 'node:fs/promises';
import { createMockSeed } from '../src/v3/proto/mock/seed.js';
import { Button, Field, Tabs, TextComposer } from '../src/v3/proto/components/forms.js';
import { ImageCard } from '../src/v3/proto/components/cards.js';
import { ConflictPanel } from '../src/v3/proto/components/revisions.js';
const saved = new Map();
globalThis.localStorage = { getItem: key => saved.get(key) || null, setItem: (key, value) => saved.set(key, value) };
globalThis.window = {};
globalThis.document = { addEventListener() {} };
const store = await import('../src/v3/proto/mock/store.js');
const { mockClient: client } = await import('../src/v3/proto/mock/client.js');
const { mockWorld: worlds } = await import('../src/v3/proto/mock/world.js');
const { mockGovernance: governance } = await import('../src/v3/proto/mock/governance.js');
const { mockOperations: operations } = await import('../src/v3/proto/mock/operations.js');
test.beforeEach(() => store.resetDemo());

test('restored Mock seed matches the original v3 fixture and uses its own namespace', async () => {
  const { seed } = await import('./fixtures/proto-seed.mjs');
  const expected = JSON.parse(JSON.stringify(seed()).replaceAll('assets/', '/v3/assets/'));
  assert.deepEqual(createMockSeed(), expected);
  assert.ok(saved.has('fluctlight.v3.acceptance.mock.v1'));
  assert.equal(saved.size, 1);
});
test('about-you save has CAS, role/Actor isolation, history and persisted readback', async () => {
  const old = await client.aboutYou('c1', 'u1');
  const value = await client.saveAboutYou('c1', 'u1', { fields: { ...old.fields, 城市: 'Mock 城市' }, expectedVersion: old.version });
  assert.equal(value.version, 1); assert.equal(value.history.length, 1);
  await assert.rejects(client.saveAboutYou('c1', 'u1', { fields: {}, expectedVersion: 0 }), { status: 409 });
  assert.equal((await client.aboutYou('c1', 'u1')).fields.城市, 'Mock 城市');
  assert.deepEqual((await client.aboutYou('c2', 'u1')).fields, {});
  assert.deepEqual((await client.aboutYou('c1', 'u_guest')).fields, {});
  await assert.rejects(client.aboutYou('c1', 'unknown'), /未找到交流参与者/);
  const fresh = await import('../src/v3/proto/mock/store.js?persisted-read');
  assert.equal(fresh.get('user').background.城市, 'Mock 城市');
});
test('NPC definition create/publish/apply never creates a relationship or moves the role', async () => {
  const location = store.get('characters', 'c1').locationId, relations = store.list('relations').length;
  const definition = await client.createNPCDraft('c1', { name: 'Mock 人物', role: '店员' });
  assert.equal(definition.published, null); assert.equal(store.list('npcs').length, 3);
  await client.publishNPCDraft(definition.id);
  const a = await client.applyNPC('c1', definition.id, 'p_cafe'), b = await client.applyNPC('c2', definition.id, '');
  assert.notEqual(a.id, b.id); assert.equal(a.appliedVersion, 'v1');
  assert.equal((await client.npcs('c1')).some(npc => npc.id === b.id), false);
  await assert.rejects(client.npc('c2', a.id), /未找到该 NPC/);
  assert.equal(store.list('relations').length, relations); assert.equal(store.get('characters', 'c1').locationId, location);
});
test('NPC editing validates scope and time, protects references, and preserves local overrides on reapply', async () => {
  const referenced = await client.npc('c1', 'n_alan');
  assert.ok(referenced.references.length); await assert.rejects(client.removeNPC('c1', 'n_alan'), /引用/);
  await assert.rejects(client.saveNPC('c1', 'n_alan', { ...referenced, expectedVersion: referenced.version, routine: [{ day: '每天', start: '99:00', end: '10:00', placeId: 'p_cafe', activity: '活动' }] }), /完整填写/);
  const definition = await client.createNPCDraft('c1', { name: '原名称', role: '原身份', bio: '' }); await client.publishNPCDraft(definition.id);
  const npc = await client.applyNPC('c1', definition.id, 'p_cafe');
  const edited = await client.saveNPC('c1', npc.id, { ...npc, expectedVersion: npc.version, name: '本地称呼', routine: [{ day: '每天', start: '09:00', end: '18:00', placeId: 'p_cafe', activity: '值班' }] });
  await client.saveNPCDraft(definition.id, { name: '新定义名称', role: '新身份', bio: '新背景' }); await client.publishNPCDraft(definition.id);
  const updated = await client.updateNPCDefinition('c1', npc.id, { expectedVersion: edited.version });
  assert.equal(updated.name, '本地称呼'); assert.equal(updated.role, '新身份'); assert.equal(updated.appliedVersion, 'v2'); assert.equal(updated.routine.length, 1);
  await assert.rejects(client.updateNPCDefinition('c1', npc.id, { expectedVersion: edited.version }), { status: 409 });
});
test('world/place management enforces revisions and reference deletion without moving on browse/save', () => {
  const location = store.get('characters', 'c1').locationId;
  assert.throws(() => worlds.removePlace('c1', 'w1', location), /引用/);
  const id = worlds.create('c1', 'Mock 世界'), world = worlds.world('c1', id);
  assert.throws(() => worlds.world('c2', id), /未找到世界/);
  assert.notEqual(store.get('worldSettings', id), store.get('worldSettings', 'w1'));
  assert.equal(store.get('worldSettings', id).harmLocked, true);
  assert.deepEqual(store.get('worldSettings', id).intercepts, []);
  worlds.save('c1', id, { name: '改名' }, world.version);
  assert.throws(() => worlds.save('c1', id, { name: '冲突' }, world.version), { status: 409 });
  const placeId = worlds.savePlace('c1', id, null, { name: 'Mock 地点', scene: '/v3/assets/scenes/cafe.svg' }, 0);
  assert.equal(worlds.place('c1', id, placeId).name, 'Mock 地点');
  assert.equal(store.get('characters', 'c1').locationId, location);
  worlds.removePlace('c1', id, placeId);
  worlds.select('c1', id);
  assert.equal(worlds.world('c1', 'w1').id, 'w1', 'the original world stays reachable after a switch');
  worlds.select('c1', 'w1');
  worlds.removeWorld('c1', id);
  assert.equal(store.get('worlds', id), undefined);
});
test('Mock goal planning actually finishes and produces candidates without automatic adoption', async () => {
  const old = governance.read('c1'), count = store.list('goals').length;
  for (const claim of old.developing.claims) for (const reference of claim.evidence) assert.ok(store.find(reference), reference);
  const next = await governance.plan('c1', old.version);
  assert.equal(next.requests[0].state, '完成'); assert.equal(next.candidates.length, 1);
  assert.equal(store.list('goals').length, count); assert.equal(governance.read('c2').candidates.length, 0);
  assert.throws(() => governance.change('c1', old.version, 'stale', () => {}), { status: 409 });
});
test('goal edits retain history and reject another role or stale revision', () => {
  const goal = governance.goal('c1', 'g_post');
  governance.commandGoal('c1', goal.id, '暂停', goal.version);
  assert.equal(governance.goal('c1', goal.id).state, '已暂停');
  assert.equal(governance.goal('c1', goal.id).revisions.length, 1);
  assert.throws(() => governance.goal('c2', goal.id), /未找到目标/);
  assert.throws(() => governance.commandGoal('c1', goal.id, '恢复', goal.version), { status: 409 });
});
test('settings validate JSON and CAS and never persist credential input', () => {
  const config = operations.configuration('models');
  operations.saveConfiguration('models', '', { ...config.values, secret: 'fictional-secret-for-mock' }, config.version);
  assert.equal(JSON.stringify(store.get()).includes('fictional-secret-for-mock'), false);
  assert.throws(() => operations.saveConfiguration('models', '', config.values, config.version), { status: 409 });
  const workflow = operations.configuration('media-workflows');
  assert.throws(() => operations.saveConfiguration('media-workflows', '', { ...workflow.values, workflow: '{bad' }, workflow.version), /JSON/);
});
test('diagnostic pagination and versioned workflow commands keep ordered records', () => {
  const first = operations.runs('workflows'), next = operations.runs('workflows', '', first.nextOffset);
  assert.equal(first.items.length, 8); assert.equal(next.items.length, 8);
  assert.equal(new Set([...first.items, ...next.items].map(item => item.id)).size, 16);
  const row = first.items[0]; operations.commandRun('workflows', row.id, '暂停', row.version);
  assert.equal(operations.runs('workflows', row.id).items[0].status, '已暂停');
  assert.throws(() => operations.commandRun('workflows', row.id, '恢复', row.version), { status: 409 });
});
test('common control components preserve attributes and avoid accidental form submissions', () => {
  assert.match(Tabs({ value: 'a', items: [{ value: 'a', label: 'A' }] }), /type="button"/);
  assert.match(Tabs({ value: 'a', items: [{ value: 'a', label: 'A', href: '#/a', attrs: { 'data-test': 'tab-a' } }] }), /data-test="tab-a"/);
  assert.match(TextComposer({ attrs: { 'aria-label': '聊天' } }), /aria-label="聊天"/);
  assert.match(ConflictPanel({ baseVersion: 1, currentVersion: 2 }), /type="button"/);
  assert.throws(() => ImageCard({ as: 'a', img: 'x', imageHref: '#/bad' }), /another link/);
  assert.match(Field({ label: '温度', name: 'temperature', value: 0.7, type: 'number' }), /step="any"/);
  assert.match(Field({ label: '标题', value: '<script>', name: 'title' }), /&lt;script&gt;/);
  assert.throws(() => Button({ attrs: { onclick: 'x' } }), /Unsafe/);
});
test('production entry selects real providers while fixtures are explicit build-time imports', async () => {
  const facade = await readFile(new URL('../src/v3/proto/store.js',import.meta.url),'utf8');
  const app = await readFile(new URL('../src/approved-ui/workspace.mjs',import.meta.url),'utf8');
  assert.match(facade,/VITE_PRODUCT_MODE === 'mock'/);
  assert.match(facade,/fixture = isMock \? await import/);
  assert.match(facade,/throw new Error\('此操作尚未连接真实命令/);
  assert.match(app,/registerProviders\(\)/);
  assert.match(app,/refreshProduct\(\{ initialOnly: true \}\)/);
  assert.match(app,/WorkspaceKey/);
  assert.doesNotMatch(app,/mountCoreSurface|component-host/);
});

test('failed Mock persistence rolls back memory and does not report a successful write', () => {
  const previous = store.get('user').background.城市, write = localStorage.setItem;
  localStorage.setItem = () => { throw new Error('quota'); };
  try { assert.throws(() => store.update(data => { data.user.background.城市 = 'unsaved'; }, { identity: '所有者修改', summary: 'failure test' }), /Mock 保存失败/); }
  finally { localStorage.setItem = write; }
  assert.equal(store.get('user').background.城市, previous);
});
test('a Mock NPC relationship is created only from an explicit sourced encounter', async () => {
  const before = store.list('relations').length, location = store.get('characters', 'c1').locationId;
  const definition = await client.createNPCDraft('c1', { name: '接触样本' }); await client.publishNPCDraft(definition.id);
  const npc = await client.applyNPC('c1', definition.id, 'p_cafe');
  await client.recordEncounter('c1', npc.id, { placeId: 'p_cafe', summary: '仅接触', establishRelation: false });
  assert.equal(store.list('relations').length, before);
  const result = await client.recordEncounter('c1', npc.id, { placeId: 'p_cafe', summary: '明确结识', establishRelation: true });
  const relation = store.get('relations', result.relationId);
  assert.equal(relation.revisions[0].source, result.eventId);
  assert.equal(store.get('events', result.eventId).npcId, npc.id);
  assert.equal(store.get('characters', 'c1').locationId, location);
});
test('isolated demo uses the same single route inventory without another app',async()=>{
  const {approvedRoute}=await import('../src/approved-ui/routing.mjs');
  const {featureRouteInventory}=await import('../src/approved-ui/feature-routes.mjs');
  const paths=featureRouteInventory.map(route=>route.path);
  assert.equal(new Set(paths).size,paths.length);
  for(const path of paths){const actual=path.replace(/:(\w+)/g,(_m,key)=>key==='charId'?'fixture-role':key==='section'?'turns':'sample');assert.ok(approvedRoute('#'+actual).feature,path);}
  const app=await readFile(new URL('../src/App.vue',import.meta.url),'utf8');
  assert.match(app,/VITE_PRODUCT_MODE/);assert.match(app,/reference-fixture/);assert.doesNotMatch(app,/proto\/app|LegacySurface/);
});

test('retired roles remain read-only in all new Mock domain services', async () => {
  await assert.rejects(client.saveAboutYou('c4', 'u1', { fields: {}, expectedVersion: 0 }), /已退休/);
  assert.throws(() => worlds.create('c4', 'must not create'), /已退休/);
  assert.throws(() => governance.saveGoal('c4', null, { title: 'must not save' }, 0), /已退休/);
  assert.throws(() => governance.change('c4', 1, 'must not mutate', () => {}), /已退休/);
});
