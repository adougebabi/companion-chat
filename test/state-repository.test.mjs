import assert from 'node:assert/strict';
import test from 'node:test';
import Database from 'better-sqlite3';

import {createStateRepository} from '../server/infrastructure/state-repository.js';

function fixture() {
    const database = new Database(':memory:');
    database.exec(`
        CREATE TABLE companion_persona_states (
            persona_id TEXT PRIMARY KEY,
            situation TEXT NOT NULL,
            mood TEXT NOT NULL,
            appearance_json TEXT NOT NULL,
            checkpoint_at TEXT NOT NULL,
            updated_at TEXT NOT NULL,
            source_event_id TEXT,
            shared_scene_json TEXT NOT NULL
        )
    `);
    const repository = createStateRepository({database, clock: () => '2026-08-23T00:00:00.000Z'});
    repository.updateProjection({
        personaId: 'persona_1',
        situation: 'reading',
        mood: 'calm',
        appearance: {coat: 'blue'},
        sharedScene: null,
        sourceEventId: 'scene_1'
    });
    return {database, repository};
}

test('state projection CAS rejects stale scene and appearance updates without changing the row', () => {
    const {database, repository} = fixture();
    const before = repository.read({personaId: 'persona_1'});

    const stale = repository.updateProjection({
        personaId: 'persona_1',
        situation: 'stale',
        mood: 'stale',
        appearance: {coat: 'red'},
        sourceEventId: 'scene_2',
        sharedScene: {location: 'cafe'},
        expected: {sourceEventId: 'scene_old', sharedSceneJson: '{}'}
    });
    assert.equal(stale.changes, 0);
    assert.equal(stale.changed, false);
    assert.deepEqual(repository.read({personaId: 'persona_1'}), before);

    const appearance = repository.updateProjection({
        personaId: 'persona_1',
        situation: 'reading',
        mood: 'calm',
        appearance: {coat: 'blue', outfit: 'white shirt'},
        sourceEventId: 'scene_1',
        expected: {appearance: {coat: 'blue'}}
    });
    assert.equal(appearance.changes, 1);
    assert.equal(repository.read({personaId: 'persona_1'}).source_event_id, 'scene_1');
    assert.deepEqual(JSON.parse(repository.read({personaId: 'persona_1'}).appearance_json), {coat: 'blue', outfit: 'white shirt'});
    database.close();
});
