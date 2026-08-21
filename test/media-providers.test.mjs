import assert from 'node:assert/strict';
import test from 'node:test';

import {cleanUrl} from '../server/infrastructure/provider-ports.js';
import {createComfyUiProvider, createH3Provider, createMediaProviders} from '../server/infrastructure/media-providers.js';

function response({body = {}, contentType = 'application/json', bytes = Buffer.from('asset'), ok = true, status = 200} = {}) {
    return {
        ok,
        status,
        headers: {get(name) { return name.toLowerCase() === 'content-type' ? contentType : null; }},
        async json() { return body; },
        async arrayBuffer() { return bytes; }
    };
}

function dependencies(overrides = {}) {
    const calls = [];
    const stats = new Map();
    const options = {
        fetch: async (url, init) => {
            calls.push({url, init});
            if (url.endsWith('/prompt')) return response({body: {prompt_id: 'prompt_test'}});
            if (url.includes('/history/')) return response({body: {prompt_test: {outputs: {node: {images: [{filename: 'result.png'}]}}}}});
            return response({contentType: 'image/png', bytes: Buffer.from('png')});
        },
        cleanUrl,
        fs: {
            mkdirSync(path, options) { calls.push({mkdir: path, options}); },
            statSync(path) { return stats.get(path) || {isFile: () => true, size: 1}; },
            readFileSync(path) { calls.push({readFile: path}); return Buffer.from('video'); }
        },
        spawn() { calls.push({spawn: true}); },
        h3Args(payload, config, outputPath) { calls.push({h3Args: {payload, config, outputPath}}); return ['-o', outputPath]; },
        h3OutputFile() { return '/allowed/output/video.mp4'; },
        async runH3(executable, args, timeoutMs, hooks) {
            calls.push({runH3: {executable, args, timeoutMs, hasSpawn: typeof hooks.spawn === 'function'}});
            hooks.onOutput?.('stdout', '50% generating');
        },
        safeH3Path(path, root) { return path.startsWith(root); },
        validComfyPromptId(value) { return value === 'prompt_test'; },
        comfyOutputFiles(history, promptId) { return history[promptId]?.outputs?.node?.images || []; },
        id(prefix) { return `${prefix}_test`; },
        settings() { return {h3AllowedRoot: '/allowed', h3OutputDir: '/allowed/output'}; },
        ...overrides
    };
    return {options, calls, stats};
}

test('media provider factory constructs both adapters without invoking injected effects', () => {
    const fixture = dependencies();
    const providers = createMediaProviders(fixture.options);

    assert.deepEqual(Object.keys(providers), ['comfyui', 'h3']);
    assert.deepEqual(providers.comfyui.capabilities, ['image', 'video']);
    assert.deepEqual(providers.h3.capabilities, ['video']);
    assert.deepEqual(fixture.calls, []);
});

test('ComfyUI adapter clones workflow prompt input, normalizes URL, and polls output files', async () => {
    const fixture = dependencies();
    const provider = createComfyUiProvider(fixture.options);
    const workflow = JSON.stringify({node: {inputs: {prompt: 'prefix {{prompt}}'}}});
    const config = {comfyUrl: 'http://comfy.test///', imageWorkflow: workflow};
    const result = await provider.submit({kind: 'image', prompt: 'a lake', settings: config});
    const submitted = JSON.parse(fixture.calls[0].init.body);

    assert.deepEqual(result, {externalId: 'prompt_test', pending: true});
    assert.equal(fixture.calls[0].url, 'http://comfy.test/prompt');
    assert.equal(submitted.prompt.node.inputs.prompt, 'prefix a lake');
    assert.equal(workflow.includes('{{prompt}}'), true);
    assert.deepEqual(await provider.poll({externalId: 'prompt_test', settings: config}), {
        status: 'complete',
        files: [{filename: 'result.png'}]
    });
    assert.equal(fixture.calls[1].url, 'http://comfy.test/history/prompt_test');
});

test('h3 adapter preserves progress, injected fs helpers, lease checks, and bounded result shape', async () => {
    const fixture = dependencies();
    const provider = createH3Provider(fixture.options);
    const events = [];
    const progress = {
        stage(name) { events.push(`stage:${name}`); return {changed: true}; },
        output(stream, text) { events.push(`output:${stream}:${text}`); },
        flush() { events.push('flush'); }
    };
    const result = await provider.submit({
        prompt: 'a quiet scene',
        payload: {h3: {width: 720}},
        settings: {h3OutputDir: '/allowed/output', h3AllowedRoot: '/allowed', h3TimeoutMs: 1234},
        progress
    });

    assert.deepEqual(result, {
        externalId: 'h3_result_test',
        pending: false,
        files: [{filename: '/allowed/output/video.mp4', type: 'h3', format: 'video', path: '/allowed/output/video.mp4'}]
    });
    assert.equal(fixture.calls.some(call => call.runH3?.timeoutMs === 1234 && call.runH3.hasSpawn), true);
    assert.equal(fixture.calls.some(call => call.mkdir === '/allowed/output'), true);
    assert.deepEqual(events, ['stage:preparing', 'stage:generating', 'output:stdout:50% generating', 'flush', 'stage:validating_output']);
    assert.deepEqual(await provider.poll({externalId: '/allowed/output/video.mp4'}), {
        status: 'complete',
        files: [{filename: '/allowed/output/video.mp4', type: 'h3', format: 'video', path: '/allowed/output/video.mp4'}]
    });
});

test('h3 asset reads use current injected settings and reject paths outside the allowed root', async () => {
    const fixture = dependencies();
    const provider = createH3Provider(fixture.options);
    const sent = [];
    await provider.readAsset({asset: {filename: '/allowed/output/video.mp4'}, res: {sendFile(path) { sent.push(path); }}});
    assert.deepEqual(sent, ['/allowed/output/video.mp4']);
    await assert.rejects(
        provider.readCandidate({file: {path: '/private/video.mp4'}, settings: {h3AllowedRoot: '/allowed', h3OutputDir: '/allowed/output'}}),
        /路径无效/
    );
});
