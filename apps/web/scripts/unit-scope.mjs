#!/usr/bin/env node
// Daily unit-test run limited to the test files a change can affect.
//
//   pnpm --filter @fluctlight/web test:changed            # uncommitted + untracked changes
//   pnpm --filter @fluctlight/web test:changed --base master
//   pnpm --filter @fluctlight/web test:changed --list      # only print the selection
//
// Each passing test file records the project files it loaded or read
// (test/support/register.mjs; local cache, see test/support/cache-root.mjs). A test file
// runs when it changed, when any recorded file changed, or when it has no record
// yet. Changes to the test harness, dependencies or compiler settings run
// everything. This is a fast feedback loop only: acceptance is always the full
// `pnpm --filter @fluctlight/web test`.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { depsDir, webRoot, workspaceRoot } from '../test/support/cache-root.mjs';
import { runUnitTests } from './unit-test.mjs';

const web = path.relative(workspaceRoot, webRoot);
const FULL = [
  `${web}/package.json`, 'pnpm-lock.yaml', 'package.json', 'pnpm-workspace.yaml',
  `${web}/tsconfig.json`, `${web}/scripts/unit-scope.mjs`, `${web}/scripts/unit-test.mjs`,
];
const FULL_DIRS = [`${web}/test/support/`, `${web}/test/helpers/`];

const args = process.argv.slice(2);
const listOnly = args.includes('--list');
const baseIndex = args.indexOf('--base');
const base = baseIndex >= 0 ? args[baseIndex + 1] : 'HEAD';
if (baseIndex >= 0 && !base) { console.error('用法：test:changed [--base REF]'); process.exit(2); }

const git = (...argv) => execFileSync('git', ['-C', workspaceRoot, ...argv], { encoding: 'utf8' }).split('\n').filter(Boolean);
const changed = [...new Set([...git('diff', '--name-only', base), ...git('ls-files', '--others', '--exclude-standard')])];

const tests = fs.readdirSync(path.join(webRoot, 'test')).filter((name) => name.endsWith('.test.mjs')).sort();
const full = changed.filter((file) => FULL.includes(file) || FULL_DIRS.some((dir) => file.startsWith(dir)));

function record(name) {
  try { return JSON.parse(fs.readFileSync(path.join(depsDir, `test__${name}.json`), 'utf8')).files; } catch { return undefined; }
}

const selected = [];
const unrecorded = [];
const covered = new Set();
for (const name of tests) {
  const own = `${web}/test/${name}`;
  const files = record(name);
  if (!files) { unrecorded.push(name); selected.push(name); continue; }
  const hits = changed.filter((file) => file === own || files.some((used) => used === file || file.startsWith(used + '/')));
  hits.forEach((file) => covered.add(file));
  if (full.length || hits.length) selected.push(name);
}

const sources = changed.filter((file) => /^(apps\/web\/src|packages)\//.test(file) && !covered.has(file));
console.log(`变更文件 ${changed.length} 个（相对 ${base}）`);
if (full.length) console.log(`测试环境或依赖有改动，运行全部 ${tests.length} 个测试文件：\n  ${full.join('\n  ')}`);
else console.log(`受影响的测试文件 ${selected.length}/${tests.length} 个`);
if (unrecorded.length && !full.length) console.log(`其中 ${unrecorded.length} 个尚无依赖记录（首次或上次未通过），一并运行`);
if (sources.length && !full.length) console.log(`以下源码改动没有任何已记录的测试读取：\n  ${sources.join('\n  ')}`);
console.log('这是日常快速反馈，不能代替全量 pnpm --filter @fluctlight/web test 验收。');
if (listOnly) { for (const name of selected) console.log(`  test/${name}`); process.exit(0); }
if (!selected.length) process.exit(0);

process.exit(runUnitTests(selected.map((name) => `test/${name}`)));
