import test from 'node:test';
import assert from 'node:assert/strict';
import { studioFixture } from './fixtures/studio-fixture.mjs';
import { worldFixture } from './fixtures/world-fixture.mjs';
function setup(scenario = 'runtime') {
  const world = worldFixture(), studio = studioFixture({ scenario, externalWorld: true, worldRuntime: world.studioWorld });
  const request = async (path, method = 'GET', body = {}) => {
    let result; const input = { path, url: new URL(path, 'http://fixture.test'), method, body,
      json: (_res, value, status = 200) => { result = { value, status }; } };
    assert.ok(await studio.handler(input) || await world.handler(input), path);
    return result;
  };
  return { world, studio, request };
}
test('world application shares catalogue truth, immutable source, keep/override, framework identity and role scope', async () => {
  const { world, studio, request } = setup();
  const source = studio.publications.get('studio-world:draft:1');
  const originalBinding = structuredClone(world.state('A').binding), other = structuredClone(world.state('B').worlds);
  const body = { fluctlightId: 'A', resourceVersion: 'draft:1' };
  const preview = (await request('/api/studio/resources/studio-world/apply-preview', 'POST', body)).value;
  assert.equal(preview.operation, 'create'); assert.equal(preview.runtime_revision, 0); assert.deepEqual(preview.diff, []);
  const command = { ...body, expectedInstanceRevision: preview.instance_revision, expectedRuntimeRevision: 0, choices: [], idempotencyKey: 'original' };
  const invalid = await request('/api/studio/resources/studio-world/apply', 'POST', { ...command, choices: [{ field: 'background', decision: 'invalid' }], idempotencyKey: 'invalid' });
  assert.equal(invalid.status, 400); assert.equal(world.state('A').worlds.length, 2);
  const stale = await request('/api/studio/resources/studio-world/apply', 'POST', { ...command, expectedInstanceRevision: 0, idempotencyKey: 'stale' });
  assert.equal(stale.status, 409); assert.equal(world.state('A').worlds.length, 2);
  const receipt = (await request('/api/studio/resources/studio-world/apply', 'POST', command)).value;
  const list = (await request('/api/fluctlights/A/world/catalog')).value;
  const row = list.items.find(w => w.id === receipt.id);
  assert.equal(row.background, source.body.background); assert.equal(row.source_version, source.version);
  assert.equal(row.source_resource_id, source.id); assert.deepEqual(list.active_binding, originalBinding); assert.equal(row.active, false);
  assert.equal(studio.resources.get(source.id).referenced_by[0].runtime_id, row.id);
  const snapshot = (await request('/api/fluctlights/A/world/catalog/' + row.id)).value;
  assert.equal(snapshot.places.length, 1); const placeId = snapshot.places[0].id;
  // A later unpublished draft must never become the applied source.
  studio.resources.get(source.id).body.background = 'UNPUBLISHED';
  world.state('A').worlds.find(w => w.id === row.id).background = 'Owner local background';
  const update = (await request('/api/studio/resources/studio-world/apply-preview', 'POST', body)).value;
  assert.equal(update.operation, 'update'); assert.equal(update.runtime_id, row.id);
  assert.equal(update.diff.find(d => d.field === 'background').to, source.body.background);
  const choices = update.diff.map(d => ({ field: d.field, decision: d.field === 'background' ? 'keep' : 'override' }));
  await request('/api/studio/resources/studio-world/apply', 'POST', { ...command, expectedRuntimeRevision: update.runtime_revision, choices, idempotencyKey: 'keep' });
  assert.equal(world.state('A').worlds.find(w => w.id === row.id).background, 'Owner local background');
  const next = (await request('/api/studio/resources/studio-world/apply-preview', 'POST', body)).value;
  await request('/api/studio/resources/studio-world/apply', 'POST', { ...command, expectedRuntimeRevision: next.runtime_revision, choices: next.diff.map(d => ({ field: d.field, decision: 'override' })), idempotencyKey: 'override' });
  assert.equal(world.state('A').worlds.find(w => w.id === row.id).background, source.body.background);
  assert.equal(world.state('A').places.filter(p => p.world_id === row.id).length, 1);
  assert.equal(world.state('A').places.find(p => p.id === placeId).name, '书房');
  assert.deepEqual(world.state('B').worlds, other); assert.deepEqual(world.state('A').binding, originalBinding);
});
test('rejection leaves runtime unchanged; lost acknowledgement commits once and original replay wins over stale CAS', async () => {
  for (const scenario of ['runtime-rejected', 'runtime-unknown']) {
    const { world, request } = setup(scenario), before = world.state('A').worlds.length;
    const body = { fluctlightId: 'A', resourceVersion: 'draft:1', expectedInstanceRevision: 1, expectedRuntimeRevision: 0, choices: [], idempotencyKey: 'same-original' };
    const first = await request('/api/studio/resources/studio-world/apply', 'POST', body);
    assert.equal(first.status, scenario === 'runtime-rejected' ? 422 : 503);
    assert.equal(world.state('A').worlds.length, before + (scenario === 'runtime-unknown' ? 1 : 0));
    if (scenario === 'runtime-unknown') {
      const replay = await request('/api/studio/resources/studio-world/apply', 'POST', body);
      assert.equal(replay.status, 200); assert.equal(replay.value.applied, true);
      assert.equal(world.state('A').worlds.length, before + 1); assert.equal(world.state('A').worlds.at(-1).revision, 1);
    }
  }
});

test('delayed transport exposes no synthetic pending success and duplicate in-flight key cannot apply twice', async () => {
  const { world, request } = setup('runtime-pending');
  const before = world.state('A').worlds.length;
  const body = { fluctlightId: 'A', resourceVersion: 'draft:1', expectedInstanceRevision: 1, expectedRuntimeRevision: 0, choices: [], idempotencyKey: 'delayed-original' };
  let settled = false;
  const pending = request('/api/studio/resources/studio-world/apply', 'POST', body).then(value => { settled = true; return value; });
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(settled, false);
  const duplicate = await request('/api/studio/resources/studio-world/apply', 'POST', body);
  assert.equal(duplicate.value.status, 'applied');
  assert.equal(world.state('A').worlds.length, before + 1);
  assert.deepEqual((await pending).value, duplicate.value);
  assert.equal(world.state('A').worlds.at(-1).revision, 1);
});

test('runtime original key cannot return a foreign receipt after role, source, version or content changes', async () => {
  const { world, request } = setup();
  const path = '/api/studio/resources/studio-world/apply';
  const body = { fluctlightId: 'A', resourceVersion: 'draft:1', expectedInstanceRevision: 1, expectedRuntimeRevision: 0, choices: [], idempotencyKey: 'scoped-original' };
  const accepted = await request(path, 'POST', body);
  assert.equal(accepted.status, 200);
  for (const [target, changed] of [
    [path, { ...body, fluctlightId: 'B' }],
    [path, { ...body, resourceVersion: 'draft:2' }],
    [path, { ...body, choices: [{ field: 'background', decision: 'keep' }] }],
    ['/api/studio/resources/studio-item/apply', body],
  ]) {
    const result = await request(target, 'POST', changed);
    assert.equal(result.status, 409); assert.equal(result.value.code, 'idempotency_conflict');
  }
  assert.equal(world.state('B').worlds.length, 2);
  assert.equal(world.state('A').worlds.length, 3);
  // Object key ordering does not change the original JSON command identity.
  const replay = await request(path, 'POST', Object.fromEntries(Object.entries(body).reverse()));
  assert.equal(replay.status, 200); assert.deepEqual(replay.value, accepted.value);
});
