import assert from 'node:assert/strict';
import test from 'node:test';

import {createCapabilityPlanFixture, capabilityCall} from './fixtures/capability-plan-flow-fixture.mjs';
import {
    cleanup as cleanupLegacy,
    legacyState,
    mediaCall as legacyMediaCall,
    sceneCall as legacySceneCall,
    streamResponse,
    invokeChatRoute,
    utcPersona
} from './fixtures/legacy-chat-capability-plan-mode.mjs';

test('modular plan dispatch is native-first, read-only, and preserves preview IDs', () => {
    const fixture = createCapabilityPlanFixture();
    const persona = fixture.createPersona('capability preview');
    try {
        const source = fixture.appendUserMessage(persona.id);
        const before = fixture.counts(persona.id);
        const dispatch = fixture.dispatch({
            calls: fixture.callsFor(persona.id, source.id),
            personaId: persona.id,
            causationUserMessageId: source.id,
            completion: {doneSeen: true},
            markerText: fixture.markerText,
            mode: 'plan'
        });

        assert.equal(dispatch.mode, 'plan');
        assert.deepEqual(dispatch.attempts.map(attempt => attempt.call.name), ['pending_event', 'media_event', 'scene_event']);
        assert.equal(dispatch.attempts.every(attempt => attempt.plan && attempt.call.source === 'native'), true);
        assert.deepEqual([...dispatch.nativeCapabilities], ['pending_event', 'media_event', 'scene_event']);
        assert.equal(dispatch.continuationEntries[0].result.pendingEvent.id, dispatch.byCapability.pending_event.plan.pendingEventId);
        assert.equal(dispatch.continuationEntries[1].result.jobId, dispatch.byCapability.media_event.plan.jobId);
        assert.equal(dispatch.continuationEntries[2].result.eventId, dispatch.byCapability.scene_event.plan.eventId);
        assert.equal(dispatch.visibleText, 'visible text');
        assert.deepEqual(fixture.counts(persona.id), before);
    } finally {
        fixture.deletePersona(persona.id);
        fixture.cleanup();
    }
});

test('modular plan dispatch fails closed for malformed, unknown, and duplicate native calls', () => {
    const fixture = createCapabilityPlanFixture();
    const persona = fixture.createPersona('capability validation');
    try {
        const source = fixture.appendUserMessage(persona.id);
        const before = fixture.counts(persona.id);
        const calls = fixture.callsFor(persona.id, source.id, 'validation');

        const malformed = fixture.dispatch({
            calls: [{...calls[1], error: 'malformed native arguments'}],
            personaId: persona.id,
            causationUserMessageId: source.id,
            completion: {doneSeen: true},
            markerText: '<media-intent>reply',
            mode: 'plan'
        });
        assert.equal(malformed.byCapability.media_event.error, 'malformed native arguments');
        assert.equal(malformed.byCapability.media_event.plan, null);
        assert.equal(malformed.visibleText, 'reply');

        const unknown = fixture.dispatch({
            calls: [capabilityCall('unknown_event', {}, 0, persona.id, source.id, 'unknown-key')],
            personaId: persona.id,
            causationUserMessageId: source.id,
            completion: {doneSeen: true},
            markerText: fixture.markerText,
            mode: 'plan'
        });
        assert.equal(unknown.unknownNative, true);
        assert.match(unknown.attempts[0].error, /Unknown native capability/);
        assert.equal(unknown.visibleText, 'visible text');

        const duplicate = fixture.dispatch({
            calls: [calls[1], {...calls[1], id: 'call_media_duplicate', index: 3}],
            personaId: persona.id,
            causationUserMessageId: source.id,
            completion: {doneSeen: true},
            markerText: '<media-intent>reply',
            mode: 'plan'
        });
        assert.match(duplicate.byCapability.media_event.error, /cardinality/);
        assert.equal(duplicate.attempts.every(attempt => !attempt.plan), true);
        assert.equal(duplicate.visibleText, 'reply');
        assert.deepEqual(fixture.counts(persona.id), before);
    } finally {
        fixture.deletePersona(persona.id);
        fixture.cleanup();
    }
});

test('legacy chat commit still applies a planned media effect and returns preview IDs', async () => {
    const {database, publicSettings, saveSettings, deletePersona} = legacyState();
    const persona = utcPersona('capability route commit');
    const previousSettings = publicSettings();
    const previousFetch = globalThis.fetch;
    const calls = [];
    const argument = JSON.stringify(legacyMediaCall('one planned photo'));
    const firstResponse = [
        `data: ${JSON.stringify({choices: [{delta: {tool_calls: [{index: 0, id: 'call_media_preview', type: 'function', function: {name: 'media_event', arguments: argument}}]}}]})}\n\n`,
        'data: [DONE]\n\n'
    ];
    const continuationResponse = [
        `data: ${JSON.stringify({choices: [{delta: {content: '照片已经准备好了。'}}]})}\n\n`,
        'data: [DONE]\n\n'
    ];
    globalThis.fetch = async (_url, options) => {
        calls.push(JSON.parse(options.body));
        return streamResponse(calls.length === 1 ? firstResponse : continuationResponse);
    };
    saveSettings({model: 'chat-plan-mode-model', lmStudioUrl: 'http://test/v1'});
    try {
        const source = await invokeChatRoute(persona.id, '请准备一张照片。');
        assert.equal(calls.length, 2);
        const toolResult = calls[1].messages.find(message => message.role === 'tool');
        assert.ok(toolResult);
        const previewResult = JSON.parse(toolResult.content);
        const job = database.prepare("SELECT id FROM companion_jobs WHERE persona_id = ? AND job_type = 'chat_image'").get(persona.id);
        assert.equal(previewResult.jobId, job.id);
        assert.match(source, /照片已经准备好了/);
        assert.equal(database.prepare("SELECT COUNT(*) AS count FROM companion_messages messages JOIN companion_conversations conversations ON conversations.id = messages.conversation_id WHERE conversations.persona_id = ? AND messages.role = 'assistant'").get(persona.id).count, 2);
        assert.equal(database.prepare("SELECT COUNT(*) AS count FROM companion_jobs WHERE persona_id = ? AND job_type = 'chat_image'").get(persona.id).count, 1);
    } finally {
        globalThis.fetch = previousFetch;
        saveSettings({model: previousSettings.model, lmStudioUrl: previousSettings.lmStudioUrl});
        deletePersona(persona.id);
    }
});

test('legacy chat commit failure rolls back a planned scene effect and assistant rows', async () => {
    const {database, publicSettings, saveSettings, deletePersona} = legacyState();
    const persona = utcPersona('capability route rollback');
    const previousSettings = publicSettings();
    const previousFetch = globalThis.fetch;
    const sourceText = '场景已经记下。';
    const firstResponse = [
        `data: ${JSON.stringify({choices: [{delta: {content: sourceText, tool_calls: [{index: 0, id: 'call_scene_rollback', type: 'function', function: {name: 'scene_event', arguments: JSON.stringify(legacySceneCall())}}]}}]})}\n\n`,
        'data: [DONE]\n\n'
    ];
    globalThis.fetch = async () => streamResponse(firstResponse);
    saveSettings({model: 'chat-plan-mode-rollback-model', lmStudioUrl: 'http://test/v1'});
    database.exec(`
        CREATE TRIGGER fail_visible_chat_commit
        BEFORE INSERT ON companion_messages
        WHEN NEW.role = 'assistant' AND length(NEW.text) > 0
        BEGIN
            SELECT RAISE(ABORT, 'forced assistant commit failure');
        END;
    `);
    try {
        const source = await invokeChatRoute(persona.id, '请记住我们在咖啡馆。');
        assert.match(source, /forced assistant commit failure/);
        assert.equal(database.prepare("SELECT COUNT(*) AS count FROM companion_life_events WHERE persona_id = ? AND type IN ('shared_scene', 'shared_scene_end')").get(persona.id).count, 0);
        assert.equal(database.prepare("SELECT COUNT(*) AS count FROM companion_messages messages JOIN companion_conversations conversations ON conversations.id = messages.conversation_id WHERE conversations.persona_id = ? AND messages.role = 'assistant'").get(persona.id).count, 0);
    } finally {
        database.exec('DROP TRIGGER fail_visible_chat_commit');
        globalThis.fetch = previousFetch;
        saveSettings({model: previousSettings.model, lmStudioUrl: previousSettings.lmStudioUrl});
        deletePersona(persona.id);
    }
});

test.after(() => cleanupLegacy());
