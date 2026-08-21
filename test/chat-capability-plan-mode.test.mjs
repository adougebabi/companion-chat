import assert from 'node:assert/strict';
import {mkdtempSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';

const dataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-chat-plan-mode-'));
process.env.DATA_DIR = dataDir;
process.env.COMPANION_TEST = '1';
process.env.COMPANION_DEBUG_INSPECTOR = '0';

const {companionApp, companionTestHooks} = await import(`../server.js?chat-plan-mode-test=${Date.now()}`);
const {
    database, createPersona, deletePersona, appendMessage, buildInitialBlueprint,
    dispatchCapabilityCalls, listMessages, publicSettings, saveSettings
} = companionTestHooks;

function mediaConcept(kind) {
    return {
        schemaVersion: 1,
        mediaKind: kind,
        scene: 'test scene',
        action: 'test action',
        mood: 'calm',
        narrative: 'test media concept',
        humanSubjects: [{label: 'persona', role: 'subject', inFrame: true}],
        nonHumanObjects: [{label: 'window', kind: 'environment', inFrame: true}],
        capture: {mode: 'external_capture', operator: 'friend', deviceVisibility: 'out_of_frame', framingIntent: 'natural medium shot'},
        compositionIntent: 'preserve the relationship between subject and environment'
    };
}

function mediaCall(request = 'a natural photo') {
    return {
        schemaVersion: 2, kind: 'image', request, count: 1,
        personaMediaConcept: mediaConcept('image'), currentEvent: null, temporaryAppearance: {}
    };
}

function sceneCall() {
    return {
        operation: 'start', location: 'quiet cafe', room: 'window seat', activity: 'talking',
        situation: 'talking together by the cafe window', mood: 'calm', objects: ['tea'],
        participants: ['user', 'persona']
    };
}

function pendingCall() {
    return {
        schemaVersion: 1, summary: 'follow up after the interview',
        notBefore: new Date(Date.now() + 60_000).toISOString(),
        expiresAt: new Date(Date.now() + 2 * 60 * 60_000).toISOString(),
        dedupeKey: `chat-plan-${Date.now()}`
    };
}

function streamResponse(chunks) {
    return {ok: true, body: {getReader() {
        let index = 0;
        return {
            read: async () => index < chunks.length
                ? {value: new TextEncoder().encode(chunks[index++]), done: false}
                : {value: undefined, done: true}
        };
    }}};
}

async function invokeChatRoute(personaId, text, chatAt = '2026-08-21T15:00:00.000Z') {
    const layer = (companionApp.router?.stack || []).find(item => item.route?.path === '/api/companion/chat' && item.route.methods?.post);
    assert.ok(layer, 'POST /api/companion/chat route is registered');
    const frames = [];
    const response = {
        statusCode: 200, headersSent: false,
        status(code) { this.statusCode = code; return this; },
        set() { return this; },
        flushHeaders() { this.headersSent = true; },
        write(value) { frames.push(String(value)); },
        end() { this.headersSent = true; }
    };
    const output = layer.route.stack[0].handle({body: {personaId, text, chatAt}}, response);
    if (output?.then) await output;
    return frames.join('');
}

function utcPersona(name) {
    const blueprint = buildInitialBlueprint({name, role: 'companion', foundation: 'chat capability plan mode test'});
    blueprint.timezone = 'UTC';
    return createPersona({name, role: 'companion', foundation: 'chat capability plan mode test', blueprint});
}

test('plan dispatch previews continuation IDs without creating capability rows', () => {
    const persona = utcPersona('capability preview');
    try {
        const source = appendMessage(persona.id, {role: 'user', text: 'remember these capability calls'});
        const calls = [
            {index: 0, id: 'call_pending', type: 'function', function: {name: 'pending_event', arguments: JSON.stringify(pendingCall())}},
            {index: 1, id: 'call_media', type: 'function', function: {name: 'media_event', arguments: JSON.stringify(mediaCall())}},
            {index: 2, id: 'call_scene', type: 'function', function: {name: 'scene_event', arguments: JSON.stringify(sceneCall())}}
        ];
        const before = {
            scenes: database.prepare("SELECT COUNT(*) AS count FROM companion_life_events WHERE persona_id = ? AND type IN ('shared_scene', 'shared_scene_end')").get(persona.id).count,
            pending: database.prepare('SELECT COUNT(*) AS count FROM companion_pending_events WHERE persona_id = ?').get(persona.id).count,
            jobs: database.prepare('SELECT COUNT(*) AS count FROM companion_jobs WHERE persona_id = ? AND job_type IN (\'chat_image\', \'chat_video\', \'pending_event\')').get(persona.id).count,
            messages: listMessages(persona.id, {markRead: false}).items.length
        };
        const dispatch = dispatchCapabilityCalls(persona, {
            toolCalls: calls,
            completion: {doneSeen: true},
            markerText: 'visible text',
            causationId: source.id,
            mode: 'plan'
        });

        assert.equal(dispatch.mode, 'plan');
        assert.deepEqual(dispatch.attempts.map(attempt => attempt.call.name), ['pending_event', 'media_event', 'scene_event']);
        assert.equal(dispatch.attempts.every(attempt => attempt.plan), true);
        assert.equal(dispatch.continuationEntries[0].result.pendingEvent.summary, 'follow up after the interview');
        assert.equal(dispatch.continuationEntries[1].result.jobId, dispatch.byCapability.media_event.plan.jobId);
        assert.equal(dispatch.continuationEntries[2].result.eventId, dispatch.byCapability.scene_event.plan.eventId);
        assert.deepEqual(dispatch.visibleText, 'visible text');
        assert.deepEqual({
            scenes: database.prepare("SELECT COUNT(*) AS count FROM companion_life_events WHERE persona_id = ? AND type IN ('shared_scene', 'shared_scene_end')").get(persona.id).count,
            pending: database.prepare('SELECT COUNT(*) AS count FROM companion_pending_events WHERE persona_id = ?').get(persona.id).count,
            jobs: database.prepare('SELECT COUNT(*) AS count FROM companion_jobs WHERE persona_id = ? AND job_type IN (\'chat_image\', \'chat_video\', \'pending_event\')').get(persona.id).count,
            messages: listMessages(persona.id, {markRead: false}).items.length
        }, before);
    } finally {
        deletePersona(persona.id);
    }
});

test('chat commit applies planned media and continuation receives preview IDs', async () => {
    const persona = utcPersona('capability route commit');
    const previousSettings = publicSettings();
    const previousFetch = globalThis.fetch;
    const calls = [];
    const argument = JSON.stringify(mediaCall('one planned photo'));
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

test('assistant commit failure rolls back a planned scene effect and assistant rows', async () => {
    const persona = utcPersona('capability route rollback');
    const previousSettings = publicSettings();
    const previousFetch = globalThis.fetch;
    const sourceText = '场景已经记下。';
    const firstResponse = [
        `data: ${JSON.stringify({choices: [{delta: {content: sourceText, tool_calls: [{index: 0, id: 'call_scene_rollback', type: 'function', function: {name: 'scene_event', arguments: JSON.stringify(sceneCall())}}]}}]})}\n\n`,
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

test.after(() => rmSync(dataDir, {recursive: true, force: true}));
