import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';

import {
    LEGACY_PRODUCTION_BOUNDARIES,
    LEGACY_PRODUCTION_SYMBOLS,
    auditLegacyProductionBoundaries,
    getLegacyProductionBoundary
} from '../server/application/legacy-production-boundaries.js';

test('legacy production inventory covers every requested server symbol', () => {
    assert.deepEqual(
        LEGACY_PRODUCTION_BOUNDARIES.map(boundary => boundary.key),
        LEGACY_PRODUCTION_SYMBOLS
    );
    for (const key of LEGACY_PRODUCTION_SYMBOLS) {
        const boundary = getLegacyProductionBoundary(key);
        assert.ok(boundary);
        assert.equal(boundary.legacy.file, 'server.js');
        assert.ok(boundary.legacy.location.includes('server.js:'));
        assert.ok(boundary.targetModules.length > 0);
        assert.ok(boundary.adapters.length > 0);
        assert.ok(boundary.blockers.length > 0);
        assert.ok(boundary.deletionChecks.length > 0);
    }
});

test('audit reports blockers instead of claiming the legacy root is deletable', () => {
    const audit = auditLegacyProductionBoundaries();
    assert.equal(audit.readyForLegacyDeletion, false);
    assert.equal(audit.statusCounts.blocked, 1);
    assert.equal(audit.statusCounts.partial, 5);
    assert.ok(audit.blockers.some(blocker => blocker.boundary === 'createEvent'));
    assert.ok(audit.blockers.some(blocker => blocker.boundary === 'jobHandlers'));
    assert.strictEqual(audit.boundaries, LEGACY_PRODUCTION_BOUNDARIES);
});

test('boundary inventory is side-effect free and does not depend on the legacy root', async () => {
    const source = await readFile(new URL('../server/application/legacy-production-boundaries.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /from\s+['"][^'"]*server\.js['"]/i);
    assert.doesNotMatch(source, /better-sqlite3|express|child_process|fetch\s*\(/i);
});
