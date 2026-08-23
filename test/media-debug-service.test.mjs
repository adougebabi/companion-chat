import assert from 'node:assert/strict';
import test from 'node:test';

import {createMediaDebugService} from '../server/application/media-debug-service.js';
import {createProviderRegistry} from '../server/infrastructure/provider-ports.js';

test('media asset reads pass the HTTP response through the provider registry input', async () => {
    let received;
    const response = {
        headers: {},
        set(name, value) { this.headers[name] = value; },
        end(value) { this.body = value; this.writableEnded = true; }
    };
    const provider = {
        id: 'fixture',
        label: 'Fixture media',
        portType: 'media',
        capabilities: ['image'],
        async readAsset(input) {
            received = input;
            input.res.set('Content-Type', 'image/png');
            input.res.end(Buffer.from('png'));
        }
    };
    const service = createMediaDebugService({
        enabled: true,
        providers: createProviderRegistry({providers: [provider]}),
        settings: () => ({}),
        repositories: {
            mediaAsset: {find: () => ({id: 'asset_1', provider: 'fixture', media_kind: 'image', filename: 'fixture.png', locator_json: '{}'})}
        }
    });

    await service.getMedia({mediaId: 'asset_1'}, {response});
    assert.equal(received.res, response);
    assert.equal(response.headers['Content-Type'], 'image/png');
    assert.deepEqual(response.body, Buffer.from('png'));
});
