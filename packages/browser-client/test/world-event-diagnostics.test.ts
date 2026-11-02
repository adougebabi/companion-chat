import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { BrowserClient } from '../src/index.ts';

test('D12 world diagnostic filters and opaque cursors reach the browser API unchanged', async () => {
  const urls: URL[] = [];
  const client = new BrowserClient('http://local.test', async input => {
    urls.push(new URL(String(input)));
    return Response.json({ items: [], next_cursor: '' });
  });
  const filters = { worldId: 'world/一', intentType: 'world.events.generate', cursor: 'opaque/+?cursor', limit: 10 };
  await client.ownerDiagnosticRecords('workflow-intents', filters);
  assert.equal(urls[0].pathname, '/api/diagnostic-records/workflow-intents');
  assert.deepEqual(Object.fromEntries(urls[0].searchParams), { ...filters, limit: '10' });
  await client.ownerDiagnosticRecords('world-event-generations', { worldId: filters.worldId });
  assert.equal(urls[1].searchParams.get('worldId'), filters.worldId);
  assert.equal(urls[1].searchParams.has('fluctlightId'), false);
  const api = JSON.parse(await readFile(new URL('../openapi.json', import.meta.url), 'utf8'));
  for (const path of ['/api/diagnostic-records/{kind}', '/api/diagnostic-export']) {
    for (const filter of ['worldId', 'intentType']) assert.ok(api.paths[path].get.parameters.some((p: {name: string; in: string}) => p.name === filter && p.in === 'query'));
  }
});
