import {mkdtempSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';

const dataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-chat-plan-mode-legacy-'));
process.env.DATA_DIR = dataDir;
process.env.COMPANION_TEST = '1';
process.env.COMPANION_DEBUG_INSPECTOR = '0';

const legacyModule = await import(`../../server.js?chat-plan-mode-legacy=${Date.now()}`);
const {companionApp, companionTestHooks} = legacyModule;
const {
    database,
    createPersona,
    deletePersona,
    appendMessage,
    buildInitialBlueprint,
    publicSettings,
    saveSettings
} = companionTestHooks;

export function mediaConcept(kind) {
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

export function mediaCall(request = 'a natural photo') {
    return {
        schemaVersion: 2,
        kind: 'image',
        request,
        count: 1,
        personaMediaConcept: mediaConcept('image'),
        currentEvent: null,
        temporaryAppearance: {}
    };
}

export function sceneCall() {
    return {
        operation: 'start',
        location: 'quiet cafe',
        room: 'window seat',
        activity: 'talking',
        situation: 'talking together by the cafe window',
        mood: 'calm',
        objects: ['tea'],
        participants: ['user', 'persona']
    };
}

export function streamResponse(chunks) {
    return {
        ok: true,
        body: {
            getReader() {
                let index = 0;
                return {
                    read: async () => index < chunks.length
                        ? {value: new TextEncoder().encode(chunks[index++]), done: false}
                        : {value: undefined, done: true}
                };
            }
        }
    };
}

export async function invokeChatRoute(personaId, text, chatAt = '2026-08-21T15:00:00.000Z') {
    const layer = (companionApp.router?.stack || []).find(item => item.route?.path === '/api/companion/chat' && item.route.methods?.post);
    if (!layer) throw new Error('POST /api/companion/chat route is not registered');
    const frames = [];
    const response = {
        statusCode: 200,
        headersSent: false,
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

export function utcPersona(name) {
    const blueprint = buildInitialBlueprint({name, role: 'companion', foundation: 'chat capability plan mode legacy comparison'});
    blueprint.timezone = 'UTC';
    return createPersona({name, role: 'companion', foundation: 'chat capability plan mode legacy comparison', blueprint});
}

export function legacyState() {
    return {database, createPersona, deletePersona, appendMessage, publicSettings, saveSettings};
}

export function cleanup() {
    rmSync(dataDir, {recursive: true, force: true});
}

