import assert from 'node:assert/strict';
import {mkdtempSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';
import Database from 'better-sqlite3';

import {startModularRuntime} from '../server/index.js';

test('server/index.js exposes a side-effect-free modular start factory', async () => {
    const dataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-modular-entry-'));
    const runtime = await startModularRuntime({
        Database,
        dataDir,
        workerRuntime: false,
        environment: {DATA_DIR: dataDir},
        startOptions: {listen: false, worker: false}
    });
    try {
        assert.equal(runtime.state, 'running');
        assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_schema_migrations').get().count, 13);
        await runtime.stop();
        assert.equal(runtime.state, 'stopped');
    } finally {
        if (runtime.state !== 'stopped') await runtime.stop().catch(() => {});
        rmSync(dataDir, {recursive: true, force: true});
    }
});
