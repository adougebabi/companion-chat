#!/usr/bin/env node
// Visual baseline management. Baselines live in <repo>/.ui-baselines/<env>/
// and are never committed.
//
//   node scripts/ui-baselines.mjs status
//   node scripts/ui-baselines.mjs verify
//   node scripts/ui-baselines.mjs approve --reason "<设计变更及审核说明>" [--approver NAME] [--grep @patterns]
//
// `approve` is the ONLY way to (re)record baselines. It must name the reviewed
// design change; it is never run automatically after a failed comparison.
import { spawnSync, execFileSync } from 'node:child_process';
import { rmSync } from 'node:fs';
import { relative } from 'node:path';
import { fileURLToPath } from 'node:url';
import { baselineDir, baselineEnv, repoRoot, verifyBaselines, writeManifest } from '../e2e/support/baselines.mjs';

const web = fileURLToPath(new URL('../', import.meta.url));
const [command = 'status', ...rest] = process.argv.slice(2);
const option = name => { const i = rest.indexOf(`--${name}`); return i >= 0 ? rest[i + 1] : undefined; };
const git = (...args) => { try { return execFileSync('git', args, { cwd: repoRoot, encoding: 'utf8' }).trim(); } catch { return ''; } };

if (command === 'status' || command === 'verify') {
  const result = verifyBaselines();
  const where = relative(repoRoot, baselineDir());
  if (result.status === 'ok') {
    const m = result.manifest;
    console.log(`视觉基准 ${where}：已审核，${Object.keys(m.files).length} 张；${m.approved_at} 由 ${m.approver || '未记录'} 审核（提交 ${m.commit || '未知'}）\n原因：${m.reason}`);
    process.exit(0);
  }
  console.error(`视觉基准 ${where}：未验证（${result.status}）。${result.detail}`);
  process.exit(command === 'verify' ? 2 : 0);
}

if (command === 'approve') {
  const reason = option('reason')?.trim();
  if (!reason || reason.length < 8) {
    console.error('必须用 --reason 说明已审核的设计变更（至少 8 个字符），例如：--reason "日程抽屉手机全宽，设计评审 2026-11-24 通过"');
    process.exit(64);
  }
  if (git('status', '--porcelain', '--', 'apps/web/src', 'apps/web/ui-lab', 'apps/web/e2e'))
    console.warn('注意：apps/web 有未提交改动；基准将按当前工作区渲染，审核记录中的提交号不包含这些改动。');
  const grep = option('grep') || '@visual';
  // Replace the whole environment store so removed snapshots do not linger.
  if (grep === '@visual') rmSync(baselineDir(), { recursive: true, force: true });
  const run = spawnSync(process.execPath, [fileURLToPath(new URL('../node_modules/@playwright/test/cli.js', import.meta.url)), 'test', '-c', 'e2e/playwright.config.ts', '--grep', grep], {
    cwd: web, stdio: 'inherit', env: { ...process.env, UI_BASELINE_APPROVAL_REASON: reason },
  });
  if (run.status !== 0) { console.error('基准录制未完成，审核记录未写入。'); process.exit(run.status || 1); }
  const manifest = writeManifest({ reason, approver: option('approver') || process.env.GITHUB_ACTOR || git('config', 'user.name') || '未记录', commit: git('rev-parse', 'HEAD') });
  console.log(`已写入 ${Object.keys(manifest.files).length} 张已审核基准（${baselineEnv()}），digest ${manifest.digest}`);
  process.exit(0);
}

console.error(`未知命令：${command}`);
process.exit(64);
