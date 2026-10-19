import assert from 'node:assert/strict';
import test from 'node:test';
import { readFile } from 'node:fs/promises';
import { acceptancePath } from '../src/v3/mock-navigation.mjs';
for (const [view, section, expected] of [['settings', 'kev', '/settings/kev'], ['diagnostics', 'kev-decisions', '/diag/kev']]) {
  test(`${section} bookmarks resolve to the Mock acceptance route`, () => assert.equal(acceptancePath('/' + view, '?section=' + section), expected));
  test(`${section} has a naturally reachable settings or diagnostics entry`, async () => {
    const source = await readFile(new URL('../src/v3/proto/pages/operations.js', import.meta.url), 'utf8');
    const modules = await readFile(new URL('../src/v3/proto/modules/创作治理.js', import.meta.url), 'utf8');
    assert.match(source, view === 'settings' ? /href="#\/settings\/\$\{section.id\}/ : /href="#\/diag\/\$\{section.id\}/);
    assert.match(modules, view === 'settings' ? /configurationSections.forEach/ : /diagnosticSections.forEach/);
  });
}
