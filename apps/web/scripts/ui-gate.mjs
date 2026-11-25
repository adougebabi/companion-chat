#!/usr/bin/env node
// UI quality gate. Picks the browser checks a change needs, runs them in real
// Chromium and reports exactly one of:
//   pass            all selected checks ran and passed            (exit 0)
//   not-applicable  no apps/web change needs a browser check      (exit 0)
//   fail            a layout, interaction or visual check failed  (exit 1)
//   unverified      something could not be verified: browser unavailable,
//                   no approved baselines, or changed UI files with no
//                   browser coverage                              (exit 2)
// Only `pass` (or `not-applicable`) may be reported as UI acceptance.
//
//   node scripts/ui-gate.mjs                 # changes vs merge-base with origin default branch
//   node scripts/ui-gate.mjs --base <ref>    # changes vs <ref>
//   node scripts/ui-gate.mjs --files a,b     # explicit file list (repo- or apps/web-relative)
//   node scripts/ui-gate.mjs --full          # every spec + every visual baseline (nightly / release)
import { execFileSync, spawnSync } from 'node:child_process';
import { existsSync, mkdirSync, readFileSync, writeFileSync, appendFileSync } from 'node:fs';
import { join } from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { repoRoot, verifyBaselines } from '../e2e/support/baselines.mjs';

const web = fileURLToPath(new URL('../', import.meta.url));
const require = createRequire(import.meta.url);
const args = process.argv.slice(2);
const flag = name => args.includes(`--${name}`);
const option = name => { const i = args.indexOf(`--${name}`); return i >= 0 ? args[i + 1] : undefined; };
const git = (...a) => execFileSync('git', a, { cwd: repoRoot, encoding: 'utf8' }).trim();
const map = JSON.parse(readFileSync(join(web, 'e2e/impact-map.json'), 'utf8'));

function globToRegExp(glob) {
  let re = '';
  for (let i = 0; i < glob.length; i++) {
    const c = glob[i];
    if (c === '*' && glob[i + 1] === '*') { re += glob[i + 2] === '/' ? '(?:.*/)?' : '.*'; i += glob[i + 2] === '/' ? 2 : 1; }
    else if (c === '*') re += '[^/]*';
    else re += c.replace(/[.+?^${}()|[\]\\]/g, '\\$&');
  }
  return new RegExp(`^${re}$`);
}
const matches = (file, globs) => globs.some(g => globToRegExp(g).test(file));

function changedFiles() {
  // --files accepts repo-relative or apps/web-relative paths.
  if (option('files') !== undefined) return option('files').split(',').map(f => f.trim()).filter(Boolean).map(f => /^(apps|packages|scripts|docs|infra)\//.test(f) || f.startsWith('.') ? f : `apps/web/${f}`);
  let base = option('base') || process.env.UI_GATE_BASE;
  if (!base) {
    const remoteHead = (() => { try { return git('symbolic-ref', '--quiet', 'refs/remotes/origin/HEAD'); } catch { return 'refs/remotes/origin/master'; } })();
    base = git('merge-base', 'HEAD', remoteHead);
  }
  const committed = git('diff', '--name-only', `${base}...HEAD`).split('\n');
  const working = git('diff', '--name-only', 'HEAD').split('\n');
  const untracked = git('ls-files', '--others', '--exclude-standard').split('\n');
  return [...new Set([...committed, ...working, ...untracked].filter(Boolean))];
}

function plan(files) {
  const web = files.filter(f => f.startsWith('apps/web/')).map(f => f.slice('apps/web/'.length));
  const tags = new Set(), uncovered = [], shared = [], removed = [];
  for (const file of web) {
    if (matches(file, map.nonUi)) continue;
    if (matches(file, map.shared)) { shared.push(file); continue; }
    let hit = false;
    for (const [tag, globs] of Object.entries(map.areas)) if (matches(file, globs)) { tags.add(tag); hit = true; }
    // A deleted file has nothing left to render. Deleted shared/area files still
    // run their checks above (e.g. a removed stylesheet changes what remains);
    // an unmapped deletion is listed, not reported as missing coverage —
    // dangling imports are caught by typecheck and build.
    if (!hit && file.startsWith('src/')) (existsSync(join(repoRoot, 'apps/web', file)) ? uncovered : removed).push(file);
  }
  // Full runs execute every existing check; coverage gaps are judged per
  // change (affected/shared runs), not against the whole branch diff.
  if (flag('full')) return { scope: 'full', grep: undefined, files: [], shared: [], uncovered: [], removed: [] };
  if (shared.length) return { scope: 'shared', grep: undefined, files: web, shared, uncovered, removed };
  if (tags.size) return { scope: 'affected', grep: ['@gate', ...tags].join('|'), files: web, tags: [...tags], uncovered, removed };
  return { scope: uncovered.length ? 'uncovered' : 'none', grep: undefined, files: web, uncovered, removed };
}

function chromiumAvailable() {
  try {
    const { chromium } = require(join(web, 'node_modules/@playwright/test'));
    const path = chromium.executablePath();
    return path && existsSync(path) ? { ok: true, path } : { ok: false, detail: `Chromium 未安装（${path}）。运行 pnpm --filter @fluctlight/web test:ui:install` };
  } catch (error) { return { ok: false, detail: `无法加载 Playwright：${error.message}` }; }
}

function runPlaywright(grep) {
  const out = join(web, 'test-results/ui-gate-report.json');
  mkdirSync(join(web, 'test-results'), { recursive: true });
  const cli = join(web, 'node_modules/@playwright/test/cli.js');
  const argv = [cli, 'test', '-c', 'e2e/playwright.config.ts', '--reporter=list,json', ...(grep ? ['--grep', grep] : [])];
  const run = spawnSync(process.execPath, argv, { cwd: web, stdio: 'inherit', env: { ...process.env, PLAYWRIGHT_JSON_OUTPUT_NAME: out } });
  if (!existsSync(out)) return { crashed: true, exit: run.status };
  const report = JSON.parse(readFileSync(out, 'utf8'));
  const failures = [], unverified = [];
  const walk = suite => {
    for (const spec of suite.specs || []) for (const t of spec.tests) for (const r of t.results) {
      if (['passed', 'skipped'].includes(r.status)) continue;
      const message = (r.error?.message || r.errors?.map(e => e.message).join('\n') || r.status).replace(/\x1b\[[0-9;]*m/g, '');
      const entry = `[${t.projectName}] ${spec.file} › ${spec.title}: ${message.split('\n').find(l => l.trim()) || r.status}`;
      (message.includes('UI_BASELINE_UNVERIFIED') ? unverified : failures).push(entry);
    }
    for (const child of suite.suites || []) walk(child);
  };
  for (const suite of report.suites || []) walk(suite);
  for (const e of report.errors || []) failures.push(`运行错误：${(e.message || '').replace(/\x1b\[[0-9;]*m/g, '').split('\n')[0]}`);
  return { stats: report.stats, failures, unverified, exit: run.status };
}

// CI: the committed lock file pins the approved baseline digest; anything else
// (missing artifact, other digest, empty lock) is UNVERIFIED.
if (process.env.UI_BASELINE_LOCK) {
  const lockPath = join(repoRoot, process.env.UI_BASELINE_LOCK);
  const lock = existsSync(lockPath) ? JSON.parse(readFileSync(lockPath, 'utf8')) : {};
  process.env.UI_BASELINE_EXPECTED_DIGEST = lock.digest || '';
}

const files = flag('full') ? [] : changedFiles();
const p = plan(files);
const result = { status: 'pass', scope: p.scope, grep: p.scope === 'none' ? null : p.scope === 'uncovered' ? '@gate' : p.grep || '(all)', changed: p.files, uncovered: p.uncovered, removed: p.removed, reasons: [], failures: [], stats: null };

if (p.scope === 'none') {
  result.status = 'not-applicable';
  result.reasons.push('apps/web 没有需要浏览器检查的改动。');
} else {
  const browser = chromiumAvailable();
  if (!browser.ok) {
    result.status = 'unverified';
    result.reasons.push(`浏览器环境不可用：${browser.detail}`);
  } else {
    const baselines = verifyBaselines();
    if (baselines.status !== 'ok') result.reasons.push(`视觉基准未验证：${baselines.detail}`);
    const run = runPlaywright(p.scope === 'uncovered' ? '@gate' : p.grep);
    if (run.crashed) { result.status = 'unverified'; result.reasons.push(`Playwright 未产生报告（退出码 ${run.exit}），无法确认结果。`); }
    else {
      result.stats = run.stats;
      result.failures = run.failures;
      if (run.failures.length) result.status = 'fail';
      else if (run.unverified.length) { result.status = 'unverified'; result.reasons.push(`${run.unverified.length} 项视觉比对因缺少已审核基准未执行。`); }
    }
  }
  if (result.status === 'pass' && p.uncovered.length) {
    result.status = 'unverified';
    result.reasons.push(`以下改动没有浏览器检查覆盖，需补充 UI Lab fixture / e2e 或在 impact-map.json 中如实归类：${p.uncovered.join('、')}`);
  }
}

const label = { pass: '通过', 'not-applicable': '无需检查', fail: '失败', unverified: '未验证' }[result.status];
const summary = [
  `## UI 质量门禁：${label}（${result.status}）`,
  `- 范围：${result.scope}${result.grep ? `（${result.grep}）` : ''}`,
  result.stats ? `- 结果：通过 ${result.stats.expected}，失败 ${result.stats.unexpected}，跳过 ${result.stats.skipped}，不稳定 ${result.stats.flaky}` : '',
  ...result.reasons.map(r => `- ${r}`),
  result.removed.length ? `- 已删除、无需浏览器检查（引用由 typecheck / build 保证）：${result.removed.join('、')}` : '',
  ...result.failures.slice(0, 30).map(f => `- ❌ ${f}`),
  result.status === 'pass' || result.status === 'not-applicable' ? '' : '\n**此结果不能作为 UI 验收通过。**',
].filter(Boolean).join('\n');
mkdirSync(join(web, 'test-results'), { recursive: true });
writeFileSync(join(web, 'test-results/ui-gate.json'), JSON.stringify(result, null, 2) + '\n');
if (process.env.GITHUB_STEP_SUMMARY) appendFileSync(process.env.GITHUB_STEP_SUMMARY, summary + '\n');
console.log('\n' + summary);
process.exit({ pass: 0, 'not-applicable': 0, fail: 1, unverified: 2 }[result.status]);
