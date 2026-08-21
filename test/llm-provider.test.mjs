import assert from 'node:assert/strict';
import test from 'node:test';

import {createMtplxProvider} from '../server/infrastructure/llm-provider.js';

test('MTPLX provider is inert until stream/models are invoked', async () => {
    let calls = 0;
    const provider = createMtplxProvider({
        settings: () => ({lmStudioUrl: 'http://127.0.0.1:8000/v1/', lmStudioApiKey: 'secret'}),
        fetchImpl: async (url, options) => {
            calls += 1;
            assert.equal(url, 'http://127.0.0.1:8000/v1/chat/completions');
            assert.equal(options.headers.Authorization, 'Bearer secret');
            return {ok: true, json: async () => ({ok: true})};
        }
    });
    assert.equal(calls, 0);
    const result = await provider.stream({model: 'fixture', messages: []});
    assert.deepEqual(await result.json(), {ok: true});
    assert.equal(calls, 1);
});

test('MTPLX provider bounds upstream errors', async () => {
    const provider = createMtplxProvider({
        settings: () => ({lmStudioUrl: 'http://fixture/v1', lmStudioApiKey: ''}),
        fetchImpl: async () => ({ok: false, status: 502, json: async () => ({error: {message: `${'x'.repeat(600)} token=secret`}})})
    });
    await assert.rejects(() => provider.models(), error => {
        assert.ok(error.message.length <= 240);
        assert.doesNotMatch(error.message, /secret/);
        return true;
    });
});
