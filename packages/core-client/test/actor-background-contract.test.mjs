import assert from 'node:assert/strict';
import test from 'node:test';
import { readFile } from 'node:fs/promises';

test('all Core background schemas preserve the same 19 strict nullable fields as Browser', async () => {
  const core = JSON.parse(await readFile(new URL('../openapi.json', import.meta.url), 'utf8'));
  const browser = JSON.parse(await readFile(new URL('../../browser-client/openapi.json', import.meta.url), 'utf8'));
  const expected = browser.components.schemas.BrowserActorUserSettings.properties.background;
  assert.equal(Object.keys(expected.properties).length, 19);
  for (const name of ['ActorUserSettings', 'ActorUserBackgroundUpdateRequest', 'CoreActorContext', 'CoreActorContextUpdate']) {
    const background = core.components.schemas[name].properties.background;
    assert.equal(background.additionalProperties, false, name);
    assert.deepEqual(background.properties, expected.properties, name);
  }
});
