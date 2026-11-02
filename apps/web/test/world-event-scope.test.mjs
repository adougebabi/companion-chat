import test from 'node:test';
import assert from 'node:assert/strict';
import { worldEventInScope } from '../src/approved-ui/world-event-scope.mjs';

test('shared world executions require the explicit ambient projection and exact world, retaining private-character fences', () => {
  const row = { id: 'rain', fluctlight_id: null, world_id: 'world-A', record_kind: 'world_event_execution', context_scope: 'ambient', source: 'world_execution', status: 'applied', life_write: false };
  assert.equal(worldEventInScope(row, 'A', 'world-A'), true);
  assert.equal(worldEventInScope({ id: 'private', fluctlight_id: 'A', world_id: 'world-A' }, 'A', 'world-A'), true);
  assert.equal(worldEventInScope({ id: 'legacy-private', fluctlight_id: 'A', world_id: null }, 'A', 'world-A'), true);
  for (const change of [{ world_id: 'world-B' }, { world_id: null }, { fluctlight_id: 'B' }, { fluctlight_id: undefined }, { record_kind: 'life_event' }, { context_scope: 'private' }, { source: 'studio' }, { status: 'queued' }, { life_write: true }, { id: '' }]) {
    assert.equal(worldEventInScope({ ...row, ...change }, 'A', 'world-A'), false, JSON.stringify(change));
  }
});
