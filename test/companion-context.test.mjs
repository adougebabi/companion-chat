import assert from 'node:assert/strict';
import {existsSync, mkdtempSync, readFileSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';

import {createCompanionTestContext} from '../server/testing/companion-context.js';

test('creates an isolated context with every table-scoped repository', () => {
    const context = createCompanionTestContext({
        clock: () => '2026-08-21T00:00:00.000Z',
        idGenerator: prefix => `${prefix}_fixture`
    });
    const dataDir = context.dataDir;
    try {
        assert.equal(context.clock(), '2026-08-21T00:00:00.000Z');
        assert.equal(context.id('persona'), 'persona_fixture');
        assert.equal(context.database.prepare('SELECT COUNT(*) AS count FROM companion_schema_migrations').get().count, 13);
        assert.deepEqual(Object.keys(context.repositories).sort(), [
            'activity', 'conversation', 'group', 'job', 'life', 'lifeEvent', 'memory',
            'pending', 'pendingEvent', 'persona', 'relationship', 'settings'
        ]);
        assert.equal(context.repositories.pending, context.repositories.pendingEvent);
        assert.equal(context.repositories.life, context.repositories.lifeEvent);
        assert.equal(context.repositories.settings.read().imageProvider, 'comfyui');
        assert.equal(existsSync(dataDir), true);
    } finally {
        assert.equal(context.cleanup(), true);
        assert.equal(context.cleanup(), false);
    }
    assert.equal(existsSync(dataDir), false);
});

test('shares deterministic clock/id/settings across repositories', () => {
    let generated = 0;
    const context = createCompanionTestContext({
        clock: {now: () => new Date('2026-08-21T01:02:03.000Z')},
        idGenerator: {next: prefix => `${prefix}_deterministic_${++generated}`},
        settings: {model: 'fixture-model', imageProvider: 'fixture-image'}
    });
    try {
        const group = context.repositories.group.create({name: 'Fixture'});
        assert.equal(group.id, 'group_deterministic_2');
        assert.equal(group.created_at, '2026-08-21T01:02:03.000Z');
        assert.equal(context.repositories.settings.read().model, 'fixture-model');
        assert.equal(context.repositories.settings.read().imageProvider, 'fixture-image');

        const defaultGroup = context.repositories.group.defaultGroup();
        context.database.prepare(`
            INSERT INTO companion_personas (
                id, name, role, color, enabled, screened_at, created_at, updated_at, deleted_at,
                group_id, image_generation_policy
            ) VALUES (?, ?, ?, ?, 1, NULL, ?, ?, NULL, ?, 'autonomous')
        `).run(
            'persona_fixture',
            'Fixture Persona',
            'Tester',
            '#123456',
            '2026-08-21T01:02:03.000Z',
            '2026-08-21T01:02:03.000Z',
            defaultGroup.id
        );
        const event = context.repositories.lifeEvent.createEvent({
            id: 'event_fixture',
            personaId: 'persona_fixture',
            type: 'fixture',
            payload: {source: 'context'},
            createdAt: '2026-08-21T01:02:03.000Z'
        });
        assert.equal(event.id, 'event_fixture');
        assert.deepEqual(JSON.parse(event.payload_json), {source: 'context'});
    } finally {
        context.cleanup();
    }
});

test('registers injected provider adapters without invoking them during construction', async () => {
    let calls = 0;
    const context = createCompanionTestContext({
        providerAdapters: {
            fixture: {
                id: 'fixture',
                portType: 'llm-streaming',
                stream() {
                    calls += 1;
                    return {text: 'dry-run'};
                }
            }
        }
    });
    try {
        assert.equal(calls, 0);
        assert.equal(context.providers.has('fixture'), true);
        assert.equal(context.providers.get('fixture'), context.providerRegistry.get('fixture'));
        await context.providers.invoke('fixture', 'stream', {});
        assert.equal(calls, 1);
    } finally {
        context.cleanup();
    }
});

test('does not delete caller-owned data directories during cleanup', () => {
    const dataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-context-owned-'));
    const context = createCompanionTestContext({dataDir});
    try {
        assert.equal(context.dataDir, dataDir);
        assert.equal(existsSync(dataDir), true);
    } finally {
        context.cleanup();
    }
    assert.equal(existsSync(dataDir), true);
    rmSync(dataDir, {recursive: true, force: true});
});

test('context module is independent of the legacy server and HTTP hooks', async () => {
    const source = readFileSync(new URL('../server/testing/companion-context.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /server\.js|express|companionTestHooks/);
});
