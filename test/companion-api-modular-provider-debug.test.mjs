import assert from 'node:assert/strict';
import {EventEmitter} from 'node:events';
import {mkdtempSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test, {after, before} from 'node:test';

import {createCompanionRuntime} from '../server/index.js';
import {createMediaJobService} from '../server/application/media-job-service.js';
import {createMediaPromptMaster, createSkippedMediaAcceptance} from '../server/infrastructure/media-prompt-master.js';
import {consumeMtplxStream} from '../server/infrastructure/llm-provider.js';
import {createProviderRegistry} from '../server/infrastructure/provider-ports.js';

const dataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-modular-provider-debug-'));
const h3Root = join(dataDir, 'h3');
const h3ModelDir = join(h3Root, 'models');
const h3OutputDir = join(h3Root, 'outputs');
const now = '2026-08-21T08:00:00.000Z';

let idCounter = 0;
let fetchMode = 'normal';
let spawnMode = 'complete';
const fetchCalls = [];
const spawnCalls = [];
const fsCalls = [];

function id(prefix) {
    idCounter += 1;
    return `${prefix}_provider_debug_${idCounter}`;
}

function response({body = {}, ok = true, status = 200, contentType = 'application/json'} = {}) {
    return {
        ok,
        status,
        headers: {get(name) { return name.toLowerCase() === 'content-type' ? contentType : null; }},
        async json() { return body; },
        async arrayBuffer() { return Buffer.from('fixture-asset'); }
    };
}

function streamResponse(lines) {
    const encoder = new TextEncoder();
    const body = new ReadableStream({
        start(controller) {
            for (const line of lines) controller.enqueue(encoder.encode(line));
            controller.close();
        }
    });
    return {ok: true, status: 200, headers: {get: () => 'text/event-stream'}, body};
}

async function fakeFetch(url, options = {}) {
    fetchCalls.push({url, options});
    if (fetchMode === 'error' && (url.endsWith('/models') || url.endsWith('/chat/completions'))) {
        return response({ok: false, status: 502, body: {error: {message: `provider secret=${'x'.repeat(600)}`}}});
    }
    if (url.endsWith('/models')) return response({body: {data: [{id: 'fixture-model'}]}});
    if (url.endsWith('/prompt')) return response({body: {prompt_id: 'prompt_provider_debug'}});
    if (url.includes('/history/')) {
        return response({body: {prompt_provider_debug: {outputs: {node: {images: [{filename: 'fixture.png'}]}}}}});
    }
    if (url.includes('/view?')) return response({contentType: 'image/png'});
    if (url.endsWith('/chat/completions')) {
        const body = JSON.parse(options.body || '{}');
        const operation = body.trace?.operation || '';
        if (operation === 'prompt_error') {
            return response({ok: false, status: 502, body: {error: {message: '模型服务拒绝请求'}}});
        }
        const system = body.messages?.find(message => message.role === 'system')?.content || '';
        const content = system.includes('media prompt master')
            ? JSON.stringify({finalPrompt: '摄影者在画外，画面内只有闺蜜；服装和小狗保持为非人物道具。'})
            : '好的。';
        return response({body: {choices: [{message: {content}}]}});
    }
    throw new Error(`Unexpected fake fetch URL: ${url}`);
}

const fakeFs = {
    mkdirSync(path, options) { fsCalls.push({kind: 'mkdir', path, options}); },
    statSync(path) {
        fsCalls.push({kind: 'stat', path});
        return {isFile: () => true, isDirectory: () => true, size: 32};
    },
    readFileSync(path) { fsCalls.push({kind: 'read', path}); return Buffer.from('fixture-video'); }
};

function fakeSpawn(executable, args) {
    spawnCalls.push({executable, args});
    const child = new EventEmitter();
    child.stdout = new EventEmitter();
    child.stderr = new EventEmitter();
    child.kill = signal => {
        spawnCalls.push({killed: signal});
    };
    if (spawnMode === 'complete') {
        queueMicrotask(() => {
            child.stdout.emit('data', Buffer.from('first\nsecond\nthird\nfourth\nfifth\n'));
            child.stdout.emit('end');
            child.emit('close', 0, null);
        });
    }
    return child;
}

function routeFor(runtime, path, method) {
    const layer = (runtime.app.router?.stack || []).find(item =>
        item.route?.path === path && item.route.methods?.[method.toLowerCase()]
    );
    assert.ok(layer, `${method} ${path} is registered`);
    return layer.route.stack.at(-1).handle;
}

async function invoke(runtime, path, method, {params = {}, body, query = {}} = {}) {
    const responseValue = {
        statusCode: 200,
        body: undefined,
        headersSent: false,
        writableEnded: false,
        headers: {},
        status(code) { this.statusCode = code; return this; },
        set(headers) { Object.assign(this.headers, headers); return this; },
        setHeader(name, value) { this.headers[name] = value; },
        flushHeaders() { this.headersSent = true; },
        json(value) { this.body = value; this.headersSent = true; return this; },
        end(value) { if (value !== undefined) this.body = value; this.headersSent = true; this.writableEnded = true; return this; }
    };
    const result = routeFor(runtime, path, method)({params, body, query}, responseValue);
    if (result && typeof result.then === 'function') await result;
    return responseValue;
}

function mediaConcept(kind = 'image') {
    return {
        schemaVersion: 1,
        mediaKind: kind,
        scene: '公园花墙前',
        action: '为闺蜜拍摄自然肖像',
        mood: '开心',
        narrative: '摄影者在画外记录朋友。',
        humanSubjects: [{label: '闺蜜', role: '被摄主体', inFrame: true}],
        nonHumanObjects: [{label: '一只小狗', kind: 'animal', inFrame: true}],
        capture: {mode: 'operator_pov', operator: '画外摄影者', deviceVisibility: 'out_of_frame', framingIntent: '自然中景'},
        compositionIntent: '保持主体与环境关系。'
    };
}

function mediaJobFixture(verdict) {
    const job = {
        id: 'job_acceptance_provider_debug',
        job_type: 'chat_image',
        status: 'leased',
        lease_owner: 'acceptance-worker',
        lease_expires_at: '2026-08-21T08:01:00.000Z',
        attempt_count: 1,
        max_attempts: 3,
        persona_id: 'persona_acceptance_provider_debug',
        message_id: 'message_acceptance_provider_debug',
        payload_json: JSON.stringify({
            kind: 'image',
            provider: 'fixture',
            envelope: {schemaVersion: 1, mediaKind: 'image'},
            personaMediaConcept: mediaConcept(),
            maxQualityRetries: 1
        }),
        result_json: '{}'
    };
    const jobs = new Map([[job.id, job]]);
    const targets = [];
    const repo = {
        findLeased({id: requestedId, leaseOwner}) {
            const found = jobs.get(requestedId);
            return found?.status === 'leased' && found.lease_owner === leaseOwner ? found : null;
        },
        find({id: requestedId}) { return jobs.get(requestedId) || null; },
        patchResult(found, {patch}) {
            found.result_json = JSON.stringify({...JSON.parse(found.result_json || '{}'), ...patch});
            return {changed: true, job: found, result: JSON.parse(found.result_json)};
        },
        enqueue(input) {
            const child = {...input, id: input.id || `job_child_${jobs.size}`, status: 'queued', payload_json: JSON.stringify(input.payload || {})};
            jobs.set(child.id, child);
            return child;
        },
        settle(input) {
            const found = jobs.get(input.id);
            if (!found || found.status !== 'leased' || found.lease_owner !== input.leaseOwner) return {changed: false, status: null, job: null};
            found.status = input.status;
            found.result_json = JSON.stringify(input.result || JSON.parse(found.result_json || '{}'));
            found.error = input.error || null;
            return {changed: true, status: input.status, job: found};
        },
        retry(input) {
            const found = jobs.get(input.id);
            if (!found || found.status !== 'leased' || found.lease_owner !== input.leaseOwner) return {changed: false, status: null, job: null};
            found.status = 'queued';
            found.run_after = input.runAfter;
            found.error = input.error || null;
            return {changed: true, status: 'retry', job: found};
        }
    };
    const provider = {
        id: 'fixture',
        capabilities: ['image'],
        async submit() { return {externalId: 'fixture_acceptance', pending: false, files: [{filename: 'fixture.png', type: 'output'}]}; },
        async poll() { return {status: 'complete', files: [{filename: 'fixture.png', type: 'output'}]}; }
    };
    const service = createMediaJobService({
        providers: createProviderRegistry({providers: [provider]}),
        repositories: {
            job: repo,
            mediaFlow: {
                updateTarget(input) { targets.push(input); return {changed: true}; },
                persistAssets({files}) { return files.map((file, index) => ({id: `asset_${index}`, file})); },
                enqueueQualityRetry({payload}) { return repo.enqueue({jobType: 'chat_image', personaId: job.persona_id, payload}); }
            }
        },
        promptMaster: {fill: () => ({finalPrompt: 'bounded provider prompt'})},
        acceptance: {accept: () => verdict},
        clock: () => now
    });
    return {job, jobs, repo, service, targets};
}

let runtime;

before(async () => {
    runtime = createCompanionRuntime({
        dataDir,
        environment: {DATA_DIR: dataDir, COMPANION_DEBUG_INSPECTOR: '1'},
        clock: () => now,
        idGenerator: id,
        fetchImpl: fakeFetch,
        spawnImpl: fakeSpawn,
        providerFs: fakeFs,
        h3Preflight: async config => ({
            ok: true,
            stage: 'process',
            config: {configured: Boolean(config?.h3Executable)},
            process: {started: true, output: [{stream: 'stdout', text: 'ready'}]}
        }),
        worker: false
    });
    await runtime.start({listen: false, worker: false});
});

after(async () => {
    await runtime.stop();
    rmSync(dataDir, {recursive: true, force: true});
});

test('default production provider registry exposes MTPLX, ComfyUI, and h3 with injectable effects', async () => {
    assert.deepEqual(runtime.providers.summaries({detailed: true}).map(item => item.id), ['mtplx', 'comfyui', 'h3']);
    const models = await runtime.providers.get('mtplx', {portType: 'llm-streaming'}).models();
    assert.deepEqual(models.data, [{id: 'fixture-model'}]);
    assert.equal(fetchCalls.some(call => call.url.endsWith('/models')), true);

    const comfy = runtime.providers.get('comfyui', {portType: 'media', capability: 'image'});
    const workflow = JSON.stringify({node: {inputs: {prompt: 'prefix {{prompt}}'}}});
    const submitted = await comfy.submit({kind: 'image', prompt: 'a quiet lake', settings: {comfyUrl: 'http://comfy.test///', imageWorkflow: workflow}});
    assert.deepEqual(submitted, {externalId: 'prompt_provider_debug', pending: true});
    const promptRequest = fetchCalls.find(call => call.url.endsWith('/prompt'));
    assert.equal(promptRequest.url, 'http://comfy.test/prompt');
    assert.equal(JSON.parse(promptRequest.options.body).prompt.node.inputs.prompt, 'prefix a quiet lake');
    assert.equal(workflow.includes('{{prompt}}'), true);
    assert.deepEqual(await comfy.poll({externalId: submitted.externalId, settings: {comfyUrl: 'http://comfy.test'}}), {
        status: 'complete', files: [{filename: 'fixture.png'}]
    });

    const h3 = runtime.providers.get('h3', {portType: 'media', capability: 'video'});
    const progress = {stage() { return {changed: true}; }, output() {}, flush() {}};
    const h3Result = await h3.submit({
        prompt: 'a short clip',
        payload: {h3: {width: 720}},
        settings: {h3Executable: '/fake/h3.c', h3ModelDir, h3OutputDir, h3AllowedRoot: h3Root, h3TimeoutMs: 1234},
        progress
    });
    assert.equal(h3Result.pending, false);
    assert.match(h3Result.files[0].filename, /\.mp4$/);
    assert.equal(spawnCalls.some(call => call.args?.includes('--width') && call.args?.includes('720')), true);
    assert.equal(fsCalls.some(call => call.kind === 'mkdir'), true);

    spawnMode = 'hang';
    await assert.rejects(() => h3.submit({
        prompt: 'timeout', payload: {},
        settings: {h3Executable: '/fake/h3.c', h3ModelDir, h3OutputDir, h3AllowedRoot: h3Root, h3TimeoutMs: 5},
        progress
    }), /timed out/);
    spawnMode = 'complete';
});

test('MTPLX errors are bounded and completion parsing redacts reasoning while preserving native calls', async () => {
    const provider = runtime.providers.get('mtplx');
    fetchMode = 'error';
    try {
        await assert.rejects(() => provider.models(), error => {
            assert.ok(error.message.length <= 240);
            assert.doesNotMatch(error.message, /x{10,}/);
            return true;
        });
    } finally {
        fetchMode = 'normal';
    }

    const tokens = [];
    const completion = await consumeMtplxStream(streamResponse([
        'data: {"choices":[{"delta":{"reasoning_content":"private reasoning"}}]}\n',
        'data: {"choices":[{"delta":{"content":"visible ","tool_calls":[{"index":0,"id":"call_1","function":{"name":"media_event","arguments":"{\\"kind\\":\\"image\\","}}]}}]}\n',
        'data: {"choices":[{"delta":{"content":"text","tool_calls":[{"index":0,"function":{"arguments":"\\"count\\":1}"}}]}}]}\n',
        'data: {"choices":[{"finish_reason":"tool_calls","delta":{}}]}\n',
        'data: [DONE]\n\n'
    ]), {onText: token => tokens.push(token), personaId: 'persona_1', causationId: 'message_1'});
    assert.deepEqual(tokens, ['visible ', 'text']);
    assert.equal(completion.text, 'visible text');
    assert.equal(completion.toolCalls[0].name, 'media_event');
    assert.deepEqual(completion.toolCalls[0].arguments, {kind: 'image', count: 1});
    assert.equal(completion.doneSeen, true);
    assert.doesNotMatch(completion.text, /reasoning/);

    const incomplete = await consumeMtplxStream(streamResponse([
        'data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"media_event","arguments":"{"}}]}}]}\n'
    ]));
    assert.equal(incomplete.doneSeen, false);
    assert.deepEqual(incomplete.incompleteToolIndexes, [0]);
    assert.equal(incomplete.parseErrors.length > 0, true);
});

test('prompt master and prompt-run debug DTO preserve frozen media semantics and redact credentials', async () => {
    const persona = runtime.application.services.persona.create({name: '提示词调试人格', role: '摄影者', foundation: '只用于 provider debug。'});
    const provider = runtime.providers.get('mtplx');
    const promptMaster = createMediaPromptMaster({provider, settings: runtime.repositories.settings});
    const filled = await promptMaster.fill({
        envelope: {schemaVersion: 1, mediaKind: 'image', personaId: persona.id},
        personaMediaConcept: mediaConcept()
    });
    assert.match(filled.finalPrompt, /画面内只有闺蜜/);
    assert.match(filled.finalPrompt, /小狗保持为非人物/);

    const traceResponse = await provider.stream({
        model: 'fixture-trace-model',
        stream: false,
        messages: [{role: 'user', content: `Bearer super-secret data:image/png;base64,${'A'.repeat(1600)}`}],
        trace: {operation: 'prompt_trace', personaId: persona.id}
    });
    await traceResponse.json();
    const debugRuns = await invoke(runtime, '/api/companion/prompt-runs', 'GET', {query: {personaId: persona.id, limit: 20}});
    assert.equal(debugRuns.statusCode, 200);
    const run = debugRuns.body.find(item => item.operation === 'prompt_trace');
    assert.ok(run);
    assert.equal(run.status, 'submitted');
    assert.equal(run.model, 'fixture-trace-model');
    assert.doesNotMatch(run.request.messages[0].content, /super-secret/);
    assert.match(run.request.messages[0].content, /binary omitted/);

    const acceptance = createSkippedMediaAcceptance({clock: () => now});
    assert.deepEqual(await acceptance.accept({}), {verdict: 'skipped', diagnostic: 'media acceptance is not configured', checkedAt: now});
});

test('media acceptance gates pass, one retry, reject, and infrastructure skip', async () => {
    for (const [verdict, expectedStatus, expectedTarget] of [
        ['pass', 'complete', 'ready'],
        ['retry', 'complete', 'queued'],
        ['reject', 'failed', 'failed'],
        ['skipped', 'complete', 'ready']
    ]) {
        const fixture = mediaJobFixture({verdict, ...(verdict === 'retry' ? {retryGuidance: '保持冻结镜头。'} : {})});
        const result = await fixture.service.submit(fixture.job, {leaseOwner: 'acceptance-worker', now});
        assert.equal(result.status, expectedStatus, verdict);
        assert.equal(fixture.job.status, expectedStatus, verdict);
        assert.equal(fixture.targets.at(-1).status, expectedTarget, verdict);
        if (verdict === 'retry') assert.equal(fixture.jobs.size, 2);
    }
});

test('settings validation, public DTOs, debug redaction, and explicit debug route gating stay bounded', async () => {
    const persona = runtime.application.services.persona.create({name: '调试人格', role: '测试者', foundation: '用于调试 DTO。'});
    runtime.application.services.conversations.appendMessage({
        personaId: persona.id,
        role: 'user',
        text: `Bearer secret-token ${'x'.repeat(2_600)}`
    });
    const context = await invoke(runtime, '/api/companion/personas/:personaId/debug-context', 'GET', {params: {personaId: persona.id}});
    assert.equal(context.statusCode, 200);
    assert.equal(JSON.stringify(context.body).includes('secret-token'), false);
    assert.equal(JSON.stringify(context.body).length <= 12_000, true);

    const settings = await invoke(runtime, '/api/companion/settings', 'PUT', {body: {model: 'public-model', lmStudioApiKey: 'public-secret'}});
    assert.equal(settings.statusCode, 200);
    assert.equal(settings.body.hasLmStudioApiKey, true);
    assert.equal(Object.hasOwn(settings.body, 'lmStudioApiKey'), false);
    assert.equal(JSON.stringify(settings.body).includes('public-secret'), false);
    assert.equal(runtime.repositories.settings.read().lmStudioApiKey, 'public-secret');

    const invalidProvider = await invoke(runtime, '/api/companion/settings', 'PUT', {body: {imageProvider: 'h3'}});
    assert.equal(invalidProvider.statusCode, 400);
    const invalidTimeout = await invoke(runtime, '/api/companion/settings', 'PUT', {body: {h3TimeoutMs: 999}});
    assert.equal(invalidTimeout.statusCode, 400);

    const preflight = await invoke(runtime, '/api/companion/h3-preflight', 'POST', {body: {h3Executable: '/fake/h3.c'}});
    assert.equal(preflight.statusCode, 200);
    assert.equal(preflight.body.ok, true);
    assert.equal(JSON.stringify(preflight.body).includes(dataDir), false);

    const disabledDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-modular-debug-disabled-'));
    const disabled = createCompanionRuntime({
        dataDir: disabledDir,
        environment: {DATA_DIR: disabledDir, COMPANION_DEBUG_INSPECTOR: '0'},
        fetchImpl: fakeFetch,
        spawnImpl: fakeSpawn,
        providerFs: fakeFs,
        worker: false
    });
    try {
        const paths = (disabled.app.router?.stack || []).filter(item => item.route).map(item => item.route.path);
        assert.equal(paths.some(path => String(path).includes('debug-context')), false);
        assert.equal(paths.some(path => String(path).includes('prompt-runs')), false);
        assert.equal(paths.some(path => String(path).includes('h3-preflight')), false);
    } finally {
        await disabled.stop();
        rmSync(disabledDir, {recursive: true, force: true});
    }
});
