import assert from 'node:assert/strict';
import test from 'node:test';

import {
    PROVIDER_PORT_TYPES,
    ProviderPortError,
    assertProviderPort,
    boundedProviderError,
    cleanUrl,
    createProviderRegistry
} from '../server/infrastructure/provider-ports.js';

function dryRunMedia(overrides = {}) {
    return {
        id: 'dry-media',
        label: 'Dry media',
        portType: PROVIDER_PORT_TYPES.MEDIA,
        capabilities: ['image', 'video'],
        async submit(input, receiver) {
            receiver?.receive?.({type: 'submitted', input});
            return {externalId: 'dry_external', pending: false};
        },
        async poll() {
            return {status: 'complete', files: []};
        },
        ...overrides
    };
}

test('provider registry registers injected adapters and exposes bounded summaries', () => {
    const llm = {
        id: 'dry-llm',
        portType: 'llm-streaming',
        capabilities: ['stream'],
        stream() {
            return {text: 'dry'};
        }
    };
    const media = dryRunMedia();
    const assets = {
        id: 'dry-assets',
        portType: 'asset-reader',
        capabilities: ['asset'],
        readAsset() {
            return {bytes: Buffer.from('dry'), mimeType: 'text/plain'};
        }
    };
    const registry = createProviderRegistry({dryRunAdapters: [llm, media, assets]});

    assert.strictEqual(registry.get('dry-llm'), llm);
    assert.strictEqual(registry.get('dry-media', 'media'), media);
    assert.strictEqual(registry.get('dry-media', {kind: 'image'}), media);
    assert.equal(registry.has('dry-assets', 'asset-reader'), true);
    assert.deepEqual(registry.summaries(), [
        {id: 'dry-llm', label: 'dry-llm', capabilities: ['stream']},
        {id: 'dry-media', label: 'Dry media', capabilities: ['image', 'video']},
        {id: 'dry-assets', label: 'dry-assets', capabilities: ['asset']}
    ]);
    assert.deepEqual(registry.summaries({detailed: true})[1], {
        id: 'dry-media',
        label: 'Dry media',
        portType: 'media',
        capabilities: ['image', 'video'],
        operations: ['submit', 'poll']
    });
});

test('unknown providers and unavailable capabilities fail with bounded provider errors', () => {
    const registry = createProviderRegistry({providers: [dryRunMedia()]});

    assert.throws(
        () => registry.get('missing-provider'),
        error => error instanceof ProviderPortError
            && error.code === 'PROVIDER_NOT_FOUND'
            && error.message.length <= 240
            && /not registered/.test(error.message)
    );
    assert.throws(
        () => registry.get('dry-media', 'llm-streaming'),
        error => error instanceof ProviderPortError
            && error.code === 'PROVIDER_CAPABILITY_UNAVAILABLE'
            && error.message.length <= 240
    );
});

test('cleanUrl trims configured URL suffixes without touching protocol-only values', () => {
    assert.equal(cleanUrl('  http://127.0.0.1:8188///  '), 'http://127.0.0.1:8188');
    assert.equal(cleanUrl('https://provider.example/api/'), 'https://provider.example/api');
    assert.equal(cleanUrl('https://'), 'https://');
    assert.equal(cleanUrl(undefined), '');
});

test('provider calls preserve adapter receiver and pass the optional receiver through', async () => {
    const receiver = {events: [], receive(event) { this.events.push(event); }};
    const provider = dryRunMedia({
        calls: 0,
        async submit(input, sink) {
            this.calls += 1;
            sink.receive({type: 'submitted', input});
            return {externalId: `dry_${this.calls}`, pending: false};
        }
    });
    const registry = createProviderRegistry({providers: [provider]});
    const result = await registry.invoke('dry-media', 'submit', {kind: 'image'}, receiver);

    assert.deepEqual(result, {externalId: 'dry_1', pending: false});
    assert.equal(provider.calls, 1);
    assert.deepEqual(receiver.events, [{type: 'submitted', input: {kind: 'image'}}]);
});

test('provider failures are wrapped and redact credentials without exposing an unbounded body', async () => {
    const provider = dryRunMedia({
        async submit() {
            throw new Error(`Bearer super-secret ${'x'.repeat(1_000)}?api_key=leak`);
        }
    });
    const registry = createProviderRegistry({providers: [provider]});

    await assert.rejects(
        registry.submit('dry-media', {kind: 'image'}),
        error => error instanceof ProviderPortError
            && error.code === 'PROVIDER_OPERATION_FAILED'
            && error.message.length <= 240
            && !error.message.includes('super-secret')
            && !error.message.includes('leak')
    );
    assert.equal(boundedProviderError('x'.repeat(1_000)).length, 240);
});

test('assertProviderPort validates the declared capability boundary and retains identity', () => {
    const provider = dryRunMedia();
    assert.strictEqual(assertProviderPort(provider, 'media'), provider);
    assert.throws(() => assertProviderPort(provider, 'llm-streaming'), /not llm-streaming/);
    assert.throws(() => assertProviderPort({id: 'invalid', portType: 'media', capabilities: ['image']}), /provide/);
    assert.throws(() => assertProviderPort({id: 'invalid', portType: 'media', submit() {}}), /declare capabilities/);
});
