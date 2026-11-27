#!/usr/bin/env node
// Runs the jsdom unit tests (all of test/*.test.mjs, or the given files) under
// the module hooks in test/support/register.mjs.
//
// node --test runs availableParallelism() - 1 files at a time by default, which
// is a single file on a machine limited to 2 CPUs. Test processes partly wait
// on file I/O, so one more file than there are usable CPUs keeps them busy
// (measured on 2 CPUs: 1 → 127 s, 2 → 78 s, 3 → 71 s for a 40-file sample).
// UNIT_TEST_CONCURRENCY overrides it.
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import { availableParallelism } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const webRoot = fileURLToPath(new URL('../', import.meta.url));

export function runUnitTests(files) {
  const concurrency = Number(process.env.UNIT_TEST_CONCURRENCY) || availableParallelism() + 1;
  const result = spawnSync(process.execPath, ['--import', './test/support/register.mjs', '--test', `--test-concurrency=${concurrency}`, ...files], { cwd: webRoot, stdio: 'inherit' });
  return result.status ?? 1;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const given = process.argv.slice(2);
  const files = given.length ? given : fs.readdirSync(path.join(webRoot, 'test')).filter((name) => name.endsWith('.test.mjs')).sort().map((name) => `test/${name}`);
  process.exit(runUnitTests(files));
}
