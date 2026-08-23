import assert from 'node:assert/strict';
import {mkdtempSync, rmSync} from 'node:fs';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import test from 'node:test';
import Database from 'better-sqlite3';

import {createInterviewAnalyzer} from '../server/application/interview-analyzer.js';
import {createInterviewService} from '../server/application/interview-service.js';
import {createCompanionRouteHandlers} from '../server/application/companion-route-handlers.js';
import {createMtplxJsonCompletionPort} from '../server/infrastructure/llm-provider.js';
import {createCompanionRuntime} from '../server/runtime/runtime.js';

function modelContent(content) {
    return {content: typeof content === 'string' ? content : JSON.stringify(content)};
}

function validResult(overrides = {}) {
    return {
        answers: {name: '林晚', role: '设计学生', foundation: '细腻慢热，喜欢摄影。', ...overrides},
        inferredFields: ['role']
    };
}

function httpResponse() {
    return {
        statusCode: 200,
        body: undefined,
        status(code) { this.statusCode = code; return this; },
        json(value) { this.body = value; return this; },
        end() { return this; },
        set() { return this; }
    };
}

test('interview analyzer accepts fenced JSON and builds an llm blueprint', async () => {
    const analyzer = createInterviewAnalyzer({jsonCompletion: {complete: async () => modelContent('```json\n' + JSON.stringify(validResult()) + '\n```')}});
    const result = await analyzer.analyze({description: '她叫林晚，喜欢摄影。'});

    assert.equal(result.source, 'llm');
    assert.equal(result.status, 'ready');
    assert.equal(result.preview.name, '林晚');
    assert.equal(result.preview.blueprint.identity.role, '设计学生');
    assert.equal(result.fieldSources.role, 'inferred');
});

test('interview analyzer normalizes structured persona lists to short strings', async () => {
    const analyzer = createInterviewAnalyzer({jsonCompletion: {complete: async () => modelContent({
        ...validResult(),
        blueprint: {
            interests: [{name: '摄影', category: '视觉创作'}],
            routine: [{from: 8, to: 10, label: '在工作室画画', scene: '工作室'}],
            supportingCast: [{name: '顾老师', role: '导师'}]
        }
    })}});

    const result = await analyzer.analyze({description: '她喜欢摄影，早晨会在工作室画画。'});

    assert.deepEqual(result.answers.interests, ['摄影']);
    assert.deepEqual(result.answers.routine, ['在工作室画画']);
    assert.deepEqual(result.answers.supportingCast, ['顾老师']);
    assert.deepEqual(result.blueprint.routine, ['在工作室画画']);
});

test('interview analyzer preserves valid string persona lists', async () => {
    const analyzer = createInterviewAnalyzer({jsonCompletion: {complete: async () => modelContent(validResult({
        interests: ['摄影', '散步'],
        routine: ['上午在工作室创作'],
        supportingCast: ['顾老师']
    }))}});

    const result = await analyzer.analyze({description: '她的生活节奏很稳定。'});

    assert.deepEqual(result.answers.interests, ['摄影', '散步']);
    assert.deepEqual(result.answers.routine, ['上午在工作室创作']);
    assert.deepEqual(result.answers.supportingCast, ['顾老师']);
});

test('interview analyzer accepts interaction boundaries as an explicit string list', async () => {
    const analyzer = createInterviewAnalyzer({jsonCompletion: {complete: async () => modelContent({
        ...validResult({interactionBoundaries: ['不要替我确认未经证实的事实', '需要独处时请先询问']}),
        blueprint: {interactionBoundaries: ['不接受强迫式追问']}
    })}});

    const result = await analyzer.analyze({description: '她有明确的互动边界。'});

    assert.deepEqual(result.answers.interactionBoundaries, ['不要替我确认未经证实的事实', '需要独处时请先询问']);
    assert.deepEqual(result.blueprint.interactionBoundaries, ['不要替我确认未经证实的事实', '需要独处时请先询问']);
});

test('interview analyzer rejects unknown or oversized structured list items', async () => {
    for (const payload of [
        {...validResult(), blueprint: {routine: [{label: '工作', unexpected: '不允许'}]}},
        {...validResult(), blueprint: {routine: [{label: '过长'.repeat(81)}]}},
        {...validResult(), blueprint: {routine: [{from: 8, to: 10}]}}
    ]) {
        const analyzer = createInterviewAnalyzer({jsonCompletion: {complete: async () => modelContent(payload)}});
        await assert.rejects(() => analyzer.analyze({description: '描述'}), error => error.status === 502);
    }
});

test('interview analyzer rejects missing required fields and unknown fields', async () => {
    for (const payload of [
        {answers: {name: '林晚', role: '学生'}, inferredFields: []},
        {...validResult(), unexpected: true}
    ]) {
        const analyzer = createInterviewAnalyzer({jsonCompletion: {complete: async () => modelContent(payload)}});
        await assert.rejects(() => analyzer.analyze({description: '描述'}), error => error.status === 502);
    }
});

test('json completion port forces non-streaming transport and bounds timeout', async () => {
    let request;
    const port = createMtplxJsonCompletionPort({
        settings: () => ({model: 'fixture'}),
        timeoutMs: 100,
        provider: {
            async stream(input) {
                request = input;
                return {ok: true, json: async () => ({choices: [{message: {content: JSON.stringify(validResult())}}]})};
            }
        }
    });
    const result = await port.complete({messages: []});
    assert.equal(request.stream, false);
    assert.equal(request.model, 'fixture');
    assert.ok(request.signal);
    assert.match(result.content, /answers/);
});

test('json completion port has no automatic timeout by default', async () => {
    const started = Date.now();
    const port = createMtplxJsonCompletionPort({
        settings: () => ({model: 'fixture'}),
        provider: {
            async stream() {
                await new Promise(resolve => setTimeout(resolve, 35));
                return {ok: true, json: async () => ({choices: [{message: {content: JSON.stringify(validResult())}}]})};
            }
        }
    });
    const result = await port.complete({messages: []});
    assert.match(result.content, /answers/);
    assert.ok(Date.now() - started >= 30);
});

test('persona JSON completion disables prompt tracing for raw descriptions', async () => {
    let request;
    const port = createMtplxJsonCompletionPort({
        settings: () => ({model: 'fixture'}),
        provider: {
            async stream(input) {
                request = input;
                return {ok: true, json: async () => ({choices: [{message: {content: JSON.stringify(validResult())}}]})};
            }
        }
    });
    await port.complete({messages: [{role: 'user', content: '原始描述'}]});
    assert.equal(request.trace, false);
});

test('json completion port rejects a provider that ignores abort', async () => {
    const port = createMtplxJsonCompletionPort({
        timeoutMs: 10,
        provider: {stream: async () => new Promise(() => {})}
    });
    await assert.rejects(() => port.complete({messages: []}), error => error.status === 502 && error.code === 'MODEL_JSON_COMPLETION_TIMEOUT');
});

test('json completion port bounds a response body that never resolves', async () => {
    const port = createMtplxJsonCompletionPort({
        timeoutMs: 10,
        provider: {
            async stream() {
                return {ok: true, json: async () => new Promise(() => {})};
            }
        }
    });
    await assert.rejects(() => port.complete({messages: []}), error => error.status === 502 && error.code === 'MODEL_JSON_COMPLETION_TIMEOUT');
});

test('interview service persists only structured answers in a ready session', async () => {
    const saved = [];
    const repository = {
        createReadyInterview(input) {
            saved.push(input);
            return {id: 'interview_1', status: 'ready', answers: input.answers, source: input.source, inferredFields: input.inferredFields};
        }
    };
    const service = createInterviewService({repository, analyzer: {analyze: async () => ({...validResult(), source: 'llm', blueprint: {identity: {name: '林晚', role: '设计学生'}}})}});
    const result = await service.analyze({description: '原始描述不应入库'});

    assert.equal(result.interviewId, 'interview_1');
    assert.equal(result.source, 'llm');
    assert.equal(result.status, 'ready');
    assert.equal(result.preview.name, '林晚');
    assert.equal(Object.hasOwn(saved[0].answers, 'description'), false);
    assert.equal(Object.hasOwn(result, 'description'), false);
});

test('interview analyze route returns the ready LLM session contract', async () => {
    const handlers = createCompanionRouteHandlers({
        services: {
            interview: {
                analyze(command) {
                    assert.deepEqual(command, {description: '一段长描述'});
                    return {status: 'ready', source: 'llm', interviewId: 'interview_route_1', answers: validResult().answers, preview: {name: '林晚'}};
                }
            }
        }
    });
    const res = httpResponse();
    await handlers.interviewAnalyze({body: {description: '一段长描述'}}, res);
    assert.equal(res.statusCode, 201);
    assert.equal(res.body.interviewId, 'interview_route_1');
    assert.equal(res.body.source, 'llm');
});

test('interview service does not call the legacy repository analyzer', async () => {
    let legacyCalled = false;
    const repository = {
        analyze() { legacyCalled = true; throw new Error('legacy analyzer called'); },
        createReadyInterview() { return {id: 'interview_1'}; }
    };
    const service = createInterviewService({repository});
    assert.throws(() => service.analyze({description: '描述'}), error => error.status === 501);
    assert.equal(legacyCalled, false);
});

test('repository no longer exposes natural-language rule analysis', async () => {
    const source = await import('node:fs/promises').then(module => module.readFile(new URL('../server/infrastructure/interview-repository.js', import.meta.url), 'utf8'));
    assert.doesNotMatch(source, /value\.match\(\/\(\?:叫\|名为\|名字是\)/);
});

test('default companion runtime analyzes through MTPLX and creates a ready session', async () => {
    const dataDir = mkdtempSync(join(tmpdir(), 'persona-analyzer-'));
    let calls = 0;
    const runtime = createCompanionRuntime({
        Database,
        dataDir,
        workerRuntime: false,
        environment: {DATA_DIR: dataDir},
        providerAdapters: {
            mtplx: {
                id: 'mtplx',
                portType: 'llm-streaming',
                capabilities: ['stream'],
                async stream(input) {
                    calls += 1;
                    assert.equal(input.stream, false);
                    return {ok: true, json: async () => ({choices: [{message: {content: JSON.stringify(validResult())}}]})};
                }
            }
        }
    });
    try {
        const result = await runtime.application.services.interview.analyze({description: '自然语言描述'});
        assert.equal(calls, 1);
        assert.match(result.interviewId, /^interview_/);
        assert.equal(result.status, 'ready');
        assert.equal(result.source, 'llm');
        assert.equal(runtime.database.prepare('SELECT status, source FROM companion_interview_sessions WHERE id = ?').get(result.interviewId).status, 'ready');
        assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_personas').get().count, 0);
        const persona = await runtime.application.services.interview.activate({interviewId: result.interviewId, overrides: {name: '编辑后的名字'}});
        assert.equal(persona.name, '编辑后的名字');
        const stored = runtime.database.prepare('SELECT answers_json FROM companion_interview_sessions WHERE id = ?').get(result.interviewId);
        assert.doesNotMatch(stored.answers_json, /自然语言描述/);
    } finally {
        await runtime.stop().catch(() => {});
        rmSync(dataDir, {recursive: true, force: true});
    }
});

test('default companion runtime returns 502 without creating a session on provider failure', async () => {
    const dataDir = mkdtempSync(join(tmpdir(), 'persona-analyzer-failure-'));
    const runtime = createCompanionRuntime({
        Database,
        dataDir,
        workerRuntime: false,
        environment: {DATA_DIR: dataDir},
        providerAdapters: {
            mtplx: {
                id: 'mtplx',
                portType: 'llm-streaming',
                capabilities: ['stream'],
                async stream() { throw new Error('provider down'); }
            }
        }
    });
    try {
        await assert.rejects(() => runtime.application.services.interview.analyze({description: '自然语言描述'}), error => error.status === 502);
        assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_interview_sessions').get().count, 0);
        assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_personas').get().count, 0);
    } finally {
        await runtime.stop().catch(() => {});
        rmSync(dataDir, {recursive: true, force: true});
    }
});
