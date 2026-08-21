import assert from 'node:assert/strict';
import {mkdtempSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test, {after, before} from 'node:test';

import {createCompanionRuntime} from '../server/index.js';

const dataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-modular-identity-'));
const runtime = createCompanionRuntime({
    dataDir,
    environment: {DATA_DIR: dataDir, COMPANION_DEBUG_INSPECTOR: '0'},
    worker: false
});
const {database} = runtime;

before(async () => {
    await runtime.start({listen: false, worker: false});
});

after(async () => {
    await runtime.stop();
    rmSync(dataDir, {recursive: true, force: true});
});

function routeFor(path, method) {
    const layer = (runtime.app.router?.stack || []).find(item =>
        item.route?.path === path && item.route.methods?.[method.toLowerCase()]
    );
    assert.ok(layer, `${method} ${path} route is registered`);
    return layer.route.stack[0].handle;
}

async function invoke(path, method, {params = {}, body, query = {}} = {}) {
    const response = {
        statusCode: 200,
        body: undefined,
        headersSent: false,
        writableEnded: false,
        status(code) {
            this.statusCode = code;
            return this;
        },
        json(value) {
            this.body = value;
            this.headersSent = true;
            return this;
        },
        end() {
            this.headersSent = true;
            this.writableEnded = true;
            return this;
        }
    };
    const output = routeFor(path, method)({params, body, query}, response);
    if (output && typeof output.then === 'function') await output;
    return response;
}

async function createPersona(input = {}) {
    const response = await invoke('/api/companion/personas', 'POST', {
        body: {
            name: input.name ?? `测试人格-${Date.now()}`,
            role: input.role ?? '陪伴者',
            foundation: input.foundation ?? '用于模块化 API 验证的稳定基础设定。',
            ...(input.color ? {color: input.color} : {})
        }
    });
    assert.equal(response.statusCode, 201);
    return response.body;
}

async function deletePersona(personaId) {
    return invoke('/api/companion/personas/:personaId', 'DELETE', {params: {personaId}});
}

function futureIso(hours = 2) {
    return new Date(Date.now() + hours * 60 * 60 * 1000).toISOString();
}

test('modular persona lifecycle creates isolated durable rows and deletes one identity', async () => {
    const first = await createPersona({name: '模块人格甲'});
    const survivor = await createPersona({name: '模块人格乙'});
    try {
        assert.match(first.id, /^persona_/);
        assert.equal(first.name, '模块人格甲');
        assert.equal(first.role, '陪伴者');
        assert.equal(first.groupId, database.prepare('SELECT id FROM companion_groups WHERE is_default = 1').get().id);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_persona_foundation_revisions WHERE persona_id = ?').get(first.id).count, 1);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_persona_life_blueprints WHERE persona_id = ?').get(first.id).count, 1);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_conversations WHERE persona_id = ?').get(first.id).count, 1);

        const detail = await invoke('/api/companion/personas/:personaId', 'GET', {params: {personaId: first.id}});
        assert.equal(detail.statusCode, 200);
        assert.equal(detail.body.persona.id, first.id);
        assert.equal(detail.body.persona.imageGenerationPolicy, 'autonomous');
        assert.equal(detail.body.imageGenerationPolicy, 'autonomous');

        const deleted = await deletePersona(first.id);
        assert.equal(deleted.statusCode, 200);
        assert.deepEqual(deleted.body, {id: first.id, deleted: true, deletedMediaIds: []});
        const missing = await invoke('/api/companion/personas/:personaId', 'GET', {params: {personaId: first.id}});
        assert.equal(missing.statusCode, 404);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_personas WHERE id = ?').get(first.id).count, 0);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_persona_foundation_revisions WHERE persona_id = ?').get(first.id).count, 0);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_conversations WHERE persona_id = ?').get(first.id).count, 0);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_personas WHERE id = ?').get(survivor.id).count, 1);
    } finally {
        await deletePersona(first.id);
        await deletePersona(survivor.id);
    }
});

test('modular contact groups keep membership and counts persona-scoped', async () => {
    const first = await createPersona({name: '分组人格甲'});
    const second = await createPersona({name: '分组人格乙'});
    try {
        const bootstrap = await invoke('/api/companion/bootstrap', 'GET');
        assert.equal(bootstrap.statusCode, 200);
        const defaultGroup = bootstrap.body.groups.find(group => group.isDefault);
        assert.ok(defaultGroup);
        assert.equal(defaultGroup.personaCount, 2);

        const created = await invoke('/api/companion/groups', 'POST', {body: {name: '  工作  '}});
        assert.equal(created.statusCode, 201);
        assert.equal(created.body.name, '工作');
        assert.equal(created.body.personaCount, 0);

        const assigned = await invoke('/api/companion/personas/:personaId/group', 'PUT', {
            params: {personaId: first.id},
            body: {groupId: created.body.id}
        });
        assert.equal(assigned.statusCode, 200);
        assert.equal(assigned.body.id, first.id);
        assert.equal(assigned.body.groupId, created.body.id);
        assert.equal(assigned.body.groupName, '工作');

        const persisted = await invoke('/api/companion/bootstrap', 'GET');
        assert.equal(persisted.body.personas.find(persona => persona.id === first.id).groupId, created.body.id);
        assert.equal(persisted.body.personas.find(persona => persona.id === second.id).groupId, defaultGroup.id);
        assert.equal(persisted.body.groups.find(group => group.id === created.body.id).personaCount, 1);
        assert.equal(persisted.body.groups.find(group => group.id === defaultGroup.id).personaCount, 1);

        for (const [body, expectedStatus] of [
            [{name: '工作'}, 400],
            [{name: '   '}, 400],
            [{name: 'x'.repeat(61)}, 400],
            [[], 400]
        ]) {
            const invalid = await invoke('/api/companion/groups', 'POST', {body});
            assert.equal(invalid.statusCode, expectedStatus);
        }
        const unknownGroup = await invoke('/api/companion/personas/:personaId/group', 'PUT', {
            params: {personaId: first.id},
            body: {groupId: 'group_missing'}
        });
        assert.equal(unknownGroup.statusCode, 404);
        const unknownPersona = await invoke('/api/companion/personas/:personaId/group', 'PUT', {
            params: {personaId: 'persona_missing'},
            body: {groupId: created.body.id}
        });
        assert.equal(unknownPersona.statusCode, 404);
    } finally {
        await deletePersona(first.id);
        await deletePersona(second.id);
    }
});

test('modular interview routes support create, answer, read, and activation lifecycle', async () => {
    const created = await invoke('/api/companion/interviews', 'POST', {
        body: {answers: {name: '访谈人格', role: '学生', foundation: '访谈基础设定。'}}
    });
    assert.equal(created.statusCode, 201);
    assert.match(created.body.id, /^interview_/);
    assert.equal(created.body.status, 'draft');
    assert.deepEqual(created.body.answers, {name: '访谈人格', role: '学生', foundation: '访谈基础设定。'});

    const read = await invoke('/api/companion/interviews/:interviewId', 'GET', {params: {interviewId: created.body.id}});
    assert.equal(read.statusCode, 200);
    assert.equal(read.body.id, created.body.id);

    const answered = await invoke('/api/companion/interviews/:interviewId/answers', 'POST', {
        params: {interviewId: created.body.id},
        body: {answers: {interests: '摄影', languageStyle: '自然简短'}}
    });
    assert.equal(answered.statusCode, 200);
    assert.equal(answered.body.status, 'ready');
    assert.equal(answered.body.answers.interests, '摄影');

    const activated = await invoke('/api/companion/interviews/:interviewId/activate', 'POST', {
        params: {interviewId: created.body.id},
        body: {color: '#123456'}
    });
    assert.equal(activated.statusCode, 201);
    assert.match(activated.body.id, /^persona_/);
    assert.equal(activated.body.name, '访谈人格');
    assert.equal(database.prepare('SELECT status FROM companion_interview_sessions WHERE id = ?').get(created.body.id).status, 'activated');
    assert.equal(database.prepare('SELECT color FROM companion_personas WHERE id = ?').get(activated.body.id).color, '#123456');

    const missing = await invoke('/api/companion/interviews/:interviewId', 'GET', {params: {interviewId: 'interview_missing'}});
    assert.equal(missing.statusCode, 404);
    await deletePersona(activated.body.id);
});

test('modular foundation routes append immutable revisions and restore by owner', async () => {
    const persona = await createPersona({name: '基础人格'});
    try {
        const initial = await invoke('/api/companion/personas/:personaId/foundation/draft', 'GET', {params: {personaId: persona.id}});
        assert.equal(initial.statusCode, 200);
        assert.equal(initial.body.version, 1);
        assert.equal(initial.body.foundation, persona.foundation ?? '用于模块化 API 验证的稳定基础设定。');
        const initialRevision = database.prepare('SELECT id FROM companion_persona_foundation_revisions WHERE persona_id = ? AND version = 1').get(persona.id);

        const updated = await invoke('/api/companion/personas/:personaId/foundation', 'PUT', {
            params: {personaId: persona.id},
            body: {foundation: '第二版基础设定。', reason: '用户修订'}
        });
        assert.equal(updated.statusCode, 201);
        assert.equal(updated.body.version, 2);
        assert.equal(updated.body.foundation, '第二版基础设定。');
        const third = await invoke('/api/companion/personas/:personaId/foundation', 'PUT', {
            params: {personaId: persona.id},
            body: {foundation: '第三版基础设定。'}
        });
        assert.equal(third.body.version, 3);

        const restored = await invoke('/api/companion/personas/:personaId/foundation-revisions/:revisionId/restore', 'POST', {
            params: {personaId: persona.id, revisionId: initialRevision.id}
        });
        assert.equal(restored.statusCode, 201);
        assert.equal(restored.body.restored, true);
        assert.equal(restored.body.foundation, initial.body.foundation);
        const latest = await invoke('/api/companion/personas/:personaId/foundation/draft', 'GET', {params: {personaId: persona.id}});
        assert.equal(latest.body.version, 4);
        assert.equal(latest.body.foundation, initial.body.foundation);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_persona_foundation_revisions WHERE persona_id = ?').get(persona.id).count, 4);

        const unknown = await invoke('/api/companion/personas/:personaId/foundation-revisions/:revisionId/restore', 'POST', {
            params: {personaId: persona.id, revisionId: 'foundation_missing'}
        });
        assert.equal(unknown.statusCode, 404);
    } finally {
        await deletePersona(persona.id);
    }
});

test('modular schedule routes validate accepted future windows and mutate one owned row', async () => {
    const persona = await createPersona({name: '日程人格甲'});
    const other = await createPersona({name: '日程人格乙'});
    try {
        const rejected = await invoke('/api/companion/personas/:personaId/schedule', 'POST', {
            params: {personaId: persona.id},
            body: {title: '未确认安排', startsAt: futureIso(), explicitlyAccepted: false}
        });
        assert.equal(rejected.statusCode, 400);

        const startsAt = futureIso(3);
        const endsAt = futureIso(4);
        const created = await invoke('/api/companion/personas/:personaId/schedule', 'POST', {
            params: {personaId: persona.id},
            body: {title: '周末看展', startsAt, endsAt, scene: '美术馆', explicitlyAccepted: true, source: 'explicit_chat_plan'}
        });
        assert.equal(created.statusCode, 201);
        assert.match(created.body.id, /^schedule_/);
        assert.equal(created.body.startsAt, startsAt);
        assert.equal(created.body.endsAt, endsAt);
        assert.equal(created.body.details.scene, '美术馆');
        assert.equal(database.prepare('SELECT status FROM companion_schedule_items WHERE id = ? AND persona_id = ?').get(created.body.id, persona.id).status, 'active');

        const movedStart = futureIso(6);
        const moved = await invoke('/api/companion/personas/:personaId/schedule/:scheduleId', 'PATCH', {
            params: {personaId: persona.id, scheduleId: created.body.id},
            body: {startsAt: movedStart, title: '傍晚看展', scene: '新馆'}
        });
        assert.equal(moved.statusCode, 200);
        assert.equal(moved.body.id, created.body.id);
        assert.equal(moved.body.startsAt, movedStart);
        assert.equal(moved.body.title, '傍晚看展');

        const foreign = await invoke('/api/companion/personas/:personaId/schedule/:scheduleId', 'PATCH', {
            params: {personaId: other.id, scheduleId: created.body.id},
            body: {startsAt: futureIso(7)}
        });
        assert.equal(foreign.statusCode, 404);
        const invalidPast = await invoke('/api/companion/personas/:personaId/schedule/:scheduleId', 'PATCH', {
            params: {personaId: persona.id, scheduleId: created.body.id},
            body: {startsAt: new Date(Date.now() - 60_000).toISOString()}
        });
        assert.equal(invalidPast.statusCode, 400);

        const cancelled = await invoke('/api/companion/personas/:personaId/schedule/:scheduleId/cancel', 'POST', {
            params: {personaId: persona.id, scheduleId: created.body.id}
        });
        assert.equal(cancelled.statusCode, 204);
        assert.equal(database.prepare('SELECT status FROM companion_schedule_items WHERE id = ?').get(created.body.id).status, 'cancelled');
    } finally {
        await deletePersona(persona.id);
        await deletePersona(other.id);
    }
});

test('modular memory deletion requires the owning persona and leaves other memories intact', async () => {
    const owner = await createPersona({name: '记忆人格甲'});
    const other = await createPersona({name: '记忆人格乙'});
    try {
        const createdAt = new Date().toISOString();
        database.prepare(`
            INSERT INTO companion_memories
                (id, persona_id, memory_key, value, confidence, status, source_type, source_id, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, 'active', ?, ?, ?, ?)
        `).run('memory_modular_owner', owner.id, '偏好', '喜欢摄影', .9, 'test', null, createdAt, createdAt);
        database.prepare(`
            INSERT INTO companion_memories
                (id, persona_id, memory_key, value, confidence, status, source_type, source_id, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, 'active', ?, ?, ?, ?)
        `).run('memory_modular_other', other.id, '偏好', '喜欢旧书', .9, 'test', null, createdAt, createdAt);

        const deleted = await invoke('/api/companion/personas/:personaId/memories/:memoryId', 'DELETE', {
            params: {personaId: owner.id, memoryId: 'memory_modular_owner'}
        });
        assert.equal(deleted.statusCode, 204);
        assert.equal(database.prepare('SELECT status FROM companion_memories WHERE id = ?').get('memory_modular_owner').status, 'deleted');
        assert.equal(database.prepare('SELECT status FROM companion_memories WHERE id = ?').get('memory_modular_other').status, 'active');

        const foreign = await invoke('/api/companion/personas/:personaId/memories/:memoryId', 'DELETE', {
            params: {personaId: owner.id, memoryId: 'memory_modular_other'}
        });
        assert.equal(foreign.statusCode, 404);
        const missing = await invoke('/api/companion/personas/:personaId/memories/:memoryId', 'DELETE', {
            params: {personaId: owner.id, memoryId: 'memory_missing'}
        });
        assert.equal(missing.statusCode, 404);
    } finally {
        await deletePersona(owner.id);
        await deletePersona(other.id);
    }
});

test('modular image generation policy validates and persists per persona', async () => {
    const first = await createPersona({name: '策略人格甲'});
    const second = await createPersona({name: '策略人格乙'});
    try {
        const allowed = ['ask', 'always', 'important', 'user_only', 'autonomous'];
        for (const policy of allowed) {
            const response = await invoke('/api/companion/personas/:personaId/image-generation-policy', 'PUT', {
                params: {personaId: first.id},
                body: {policy}
            });
            assert.equal(response.statusCode, 200);
            assert.equal(response.body.personaId, first.id);
            assert.equal(response.body.imageGenerationPolicy, policy);
            assert.equal(database.prepare('SELECT image_generation_policy FROM companion_personas WHERE id = ?').get(first.id).image_generation_policy, policy);
            assert.equal(database.prepare('SELECT image_generation_policy FROM companion_personas WHERE id = ?').get(second.id).image_generation_policy, 'autonomous');
        }
        const invalid = await invoke('/api/companion/personas/:personaId/image-generation-policy', 'PUT', {
            params: {personaId: first.id},
            body: {policy: '拍照就生成'}
        });
        assert.equal(invalid.statusCode, 400);
        assert.equal(database.prepare('SELECT image_generation_policy FROM companion_personas WHERE id = ?').get(first.id).image_generation_policy, 'autonomous');
    } finally {
        await deletePersona(first.id);
        await deletePersona(second.id);
    }
});
