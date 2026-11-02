import test from 'node:test';
import assert from 'node:assert/strict';
import { BrowserClient } from '../src/index.ts';

test('directory GET retains selected avatar and explicit unavailable state for the browser', async () => {
  const expected = [
    { id: 'qinglan', identity: { name: 'FT-青岚' }, status: 'active', avatar: { asset_id: 'portrait', url: '/api/media/portrait' }, avatar_state: { status: 'ready', mode: 'canonical' } },
    { id: 'qingwu', identity: { name: 'FT-清雾' }, status: 'active', avatar: null, avatar_state: { status: 'source_unavailable', mode: 'asset' } },
  ];
  const calls: Array<{ url: string; method: string; credentials?: RequestCredentials }> = [];
  const client = new BrowserClient('https://core.example.test', async (input, init) => {
    calls.push({ url: String(input), method: init?.method || 'GET', credentials: init?.credentials });
    return Response.json(expected);
  });
  assert.deepEqual(await client.listFluctlights(), expected);
  assert.deepEqual(calls, [{ url: 'https://core.example.test/api/fluctlights', method: 'GET', credentials: 'include' }]);
});
