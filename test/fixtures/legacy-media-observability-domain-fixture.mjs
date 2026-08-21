import {mkdtempSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';

const dataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-media-observability-legacy-'));
process.env.DATA_DIR = dataDir;
process.env.COMPANION_TEST = '1';
process.env.COMPANION_DEBUG_INSPECTOR = '1';

const legacyRoot = await import(`../../server.js?media-observability-legacy=${Date.now()}`);
const {companionTestHooks} = legacyRoot;

export function state() {
    return {
        database: companionTestHooks.database,
        createPersona: companionTestHooks.createPersona,
        createChatMediaRequest: companionTestHooks.createChatMediaRequest,
        debugContextFor: companionTestHooks.debugContextFor,
        saveSettings: companionTestHooks.saveSettings,
        publicSettings: companionTestHooks.publicSettings
    };
}

export function cleanup() {
    rmSync(dataDir, {recursive: true, force: true});
}
