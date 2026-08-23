import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';

import {
    createIdentitySettingsService,
    publicSettings
} from '../server/application/identity-settings-service.js';

function fixture(overrides = {}) {
    const writes = [];
    let stored = {
        model: 'fixture-model',
        lmStudioApiKey: 'super-secret',
        h3Executable: '/private/tools/h3',
        h3ModelDir: '/private/models',
        h3OutputDir: '/private/outputs',
        h3AllowedRoot: '/private',
        h3Defaults: {profile: '/private/profile', steps: 12},
        h3TimeoutMs: 120000,
        activityReadAt: '2026-08-20T00:00:00.000Z',
        nested: {authorization: 'Bearer nested-secret', note: 'apiKey=inline-secret'},
        ...overrides.settings
    };
    const repositories = {
        persona: {
            listActive: () => [{
                id: 'persona_a', name: 'A', role: 'tester', color: '#fff', group_id: 'group_a',
                screened_at: '2026-08-20T00:00:00.000Z', situation: '在窗边看书', mood: '安静',
                unread_count: '2', updated_at: '2026-08-21T00:00:00.000Z'
            }, {
                id: 'persona_b', name: 'B', role: 'writer', color: '#000', groupId: null,
                screened: false, currentSituation: '在路上', mood: '轻快', unreadCount: 0, updatedAt: 'now'
            }]
        },
        group: {
            list: () => [{id: 'group_a', name: '工作', is_default: 1, persona_count: '1'}]
        },
        activity: {hasUnread: ({readAt}) => readAt === '2026-08-20T00:00:00.000Z'},
        settings: {
            read: () => stored,
            update: next => {
                writes.push(structuredClone(next));
                stored = structuredClone(next);
                return stored;
            }
        }
    };
    const service = createIdentitySettingsService({
        repositories: {...repositories, ...(overrides.repositories ?? {})},
        defaultTimezone: 'Asia/Shanghai',
        debugInspector: true,
        h3ConfigSummary: () => ({
            executable: {configured: true, valid: true, displayName: '…/h3'},
            modelDir: {configured: true, valid: true, displayName: '…/models'},
            leakedPath: '/private/should-not-leak'
        }),
        mediaProviders: () => [{id: 'comfyui', capabilities: ['image']}],
        settingsPolicy: overrides.settingsPolicy,
        ...overrides.options
    });
    return {service, writes, readStored: () => stored};
}

test('bootstrap returns stable identity/group DTOs from injected repositories', () => {
    const {service} = fixture();
    assert.deepEqual(service.bootstrap.read(), {
        settings: {
            model: 'fixture-model',
            h3Defaults: {steps: 12},
            h3TimeoutMs: 120000,
            activityReadAt: '2026-08-20T00:00:00.000Z',
            nested: {note: 'apiKey=[redacted]'},
            hasH3Configuration: true,
            hasLmStudioApiKey: true,
            h3ConfigSummary: {
                executable: {configured: true, valid: true, displayName: '…/h3'},
                modelDir: {configured: true, valid: true, displayName: '…/models'},
                leakedPath: '[path redacted]'
            },
            mediaProviders: [{id: 'comfyui', capabilities: ['image']}]
        },
        personas: [{
            id: 'persona_a', initializationMode: 'llm_defined', name: 'A', role: 'tester', color: '#fff', groupId: 'group_a', groupName: '工作',
            screened: true, currentSituation: '在窗边看书', mood: '安静', unreadCount: 2, updatedAt: '2026-08-21T00:00:00.000Z'
        }, {
            id: 'persona_b', initializationMode: 'llm_defined', name: 'B', role: 'writer', color: '#000', groupId: null, groupName: null,
            screened: false, currentSituation: '在路上', mood: '轻快', unreadCount: 0, updatedAt: 'now'
        }],
        groups: [{id: 'group_a', name: '工作', isDefault: true, personaCount: 1}],
        activityUnread: true,
        defaultTimezone: 'Asia/Shanghai',
        debugInspector: true
    });
});

test('settings reads and updates are redacted, merged, and preserve a configured key', () => {
    const {service, writes, readStored} = fixture();
    const initial = service.settings.read();
    assert.equal(Object.hasOwn(initial, 'lmStudioApiKey'), false);
    assert.equal(Object.hasOwn(initial, 'h3Executable'), false);
    assert.equal(Object.hasOwn(initial, 'h3ModelDir'), false);
    assert.equal(Object.hasOwn(initial, 'h3OutputDir'), false);
    assert.equal(Object.hasOwn(initial, 'h3AllowedRoot'), false);
    assert.equal(Object.hasOwn(initial.h3Defaults, 'profile'), false);
    assert.equal(JSON.stringify(initial).includes('/private/tools/h3'), false);
    assert.equal(JSON.stringify(initial).includes('nested-secret'), false);
    assert.equal(initial.hasLmStudioApiKey, true);

    const updated = service.settings.update({model: 'next-model', lmStudioApiKey: 'configured'});
    assert.equal(writes.length, 1);
    assert.equal(writes[0].model, 'next-model');
    assert.equal(writes[0].lmStudioApiKey, 'super-secret');
    assert.equal(writes[0].h3OutputDir, '/private/outputs');
    assert.equal(updated.model, 'next-model');
    assert.equal(readStored().h3TimeoutMs, 120000);
});

test('settings policy validates the merged value before the port is written', () => {
    const calls = [];
    const {service, writes} = fixture({
        settingsPolicy: {
            validate(next, context) {
                calls.push({next: structuredClone(next), patch: context.patch});
                if (next.model === 'blocked') throw new Error('model blocked');
                return {...next, policyApplied: true};
            }
        }
    });
    assert.throws(() => service.settings.update({model: 'blocked'}), /model blocked/);
    assert.equal(writes.length, 0);
    assert.equal(calls.length, 1);
    assert.throws(() => service.settings.update([]), /Settings command must be an object/);
    assert.throws(() => service.settings.update(null), /Settings command must be an object/);

    const result = service.settings.update({model: 'allowed'});
    assert.equal(result.model, 'allowed');
    assert.equal(writes[0].policyApplied, true);
});

test('public settings never mutates the repository object and handles malformed optional summaries', () => {
    const raw = {
        model: 'fixture',
        lmStudioApiKey: 'key',
        h3Defaults: {profile: 'private', enabled: true},
        nested: {token: 'token', value: 'Bearer secret'},
        list: [{password: 'secret', value: 'ok'}]
    };
    const output = publicSettings(raw, {h3ConfigSummary: ['invalid'], mediaProviders: undefined});
    assert.deepEqual(output, {
        model: 'fixture',
        h3Defaults: {enabled: true},
        hasH3Configuration: false,
        hasLmStudioApiKey: true,
        nested: {value: 'Bearer [redacted]'},
        list: [{value: 'ok'}]
    });
    assert.deepEqual(raw, {
        model: 'fixture',
        lmStudioApiKey: 'key',
        h3Defaults: {profile: 'private', enabled: true},
        nested: {token: 'token', value: 'Bearer secret'},
        list: [{password: 'secret', value: 'ok'}]
    });
});

test('service rejects missing or asynchronous ports and has no transport/storage ownership', async () => {
    assert.throws(() => createIdentitySettingsService(), /persona repository/);
    assert.throws(() => createIdentitySettingsService({repositories: {persona: {}, group: {}, settings: {}}}), /listActive\(\) or list\(\)/);
    assert.throws(() => createIdentitySettingsService({
        repositories: {
            persona: {listActive: async () => []},
            group: {list: () => []},
            settings: {read: () => ({})}
        }
    }).bootstrap.read(), /must be synchronous/);

    const source = await readFile(new URL('../server/application/identity-settings-service.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /(?:from|import)\s+['"][^'"]*(?:server\.js|express|better-sqlite3|provider)/i);
    assert.doesNotMatch(source, /fetch\s*\(/i);
});
