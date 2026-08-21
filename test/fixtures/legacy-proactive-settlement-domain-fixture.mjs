import {mkdtempSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';

const dataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-job-worker-legacy-'));
process.env.DATA_DIR = dataDir;
process.env.COMPANION_TEST = '1';
process.env.COMPANION_DEBUG_INSPECTOR = '0';

const legacyRoot = await import(`../../server.js?job-worker-legacy=${Date.now()}`);
const {companionTestHooks} = legacyRoot;

export function state() {
    return {
        database: companionTestHooks.database,
        createPersona: companionTestHooks.createPersona,
        createEvent: companionTestHooks.createEvent,
        deletePersona: companionTestHooks.deletePersona,
        appendMessage: companionTestHooks.appendMessage,
        completeProactiveMessageJob: companionTestHooks.completeProactiveMessageJob
    };
}

export function cleanup() {
    rmSync(dataDir, {recursive: true, force: true});
}
