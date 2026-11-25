// Visual baseline store. Images live in <repo>/.ui-baselines/<env>/ and are
// NEVER committed (AGENTS.md). Each environment directory carries a text
// manifest recording who approved which images, why, and their sha256, so a
// baseline changed outside the approval flow is detected before comparison.
import { createHash } from 'node:crypto';
import { existsSync, readdirSync, readFileSync, statSync, writeFileSync } from 'node:fs';
import { join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

export const repoRoot = fileURLToPath(new URL('../../../../', import.meta.url));
export const baselineRoot = join(repoRoot, '.ui-baselines');

/** Rendering differs between machines (fonts, AA), so baselines are per environment. */
export function baselineEnv() {
  return process.env.UI_BASELINE_ENV || (process.env.CI ? 'ci-linux' : `local-${process.platform}`);
}
export const baselineDir = (env = baselineEnv()) => join(baselineRoot, env);
export const manifestPath = (env = baselineEnv()) => join(baselineDir(env), 'manifest.json');

function walk(dir) {
  if (!existsSync(dir)) return [];
  const out = [];
  for (const name of readdirSync(dir)) {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) out.push(...walk(path));
    else if (name.endsWith('.png')) out.push(path);
  }
  return out;
}
const sha256 = path => createHash('sha256').update(readFileSync(path)).digest('hex');

export function hashBaselines(env = baselineEnv()) {
  const dir = baselineDir(env);
  return Object.fromEntries(walk(dir).map(path => [relative(dir, path).replaceAll('\\', '/'), sha256(path)]).sort(([a], [b]) => a.localeCompare(b)));
}

export function readManifest(env = baselineEnv()) {
  const path = manifestPath(env);
  return existsSync(path) ? JSON.parse(readFileSync(path, 'utf8')) : null;
}

export function writeManifest({ env = baselineEnv(), reason, approver, commit }) {
  const files = hashBaselines(env);
  const digest = createHash('sha256').update(JSON.stringify(files)).digest('hex');
  const manifest = { schema: 1, env, approved_at: new Date().toISOString(), approver, reason, commit, digest, files };
  writeFileSync(manifestPath(env), JSON.stringify(manifest, null, 2) + '\n');
  return manifest;
}

/**
 * Status of the baseline store for comparison:
 * - missing: no approved baselines → visual checks are UNVERIFIED, never passed.
 * - tampered: files differ from the approved manifest → UNVERIFIED.
 * - ok: files exactly match the approved manifest.
 */
export function verifyBaselines(env = baselineEnv()) {
  const manifest = readManifest(env);
  if (!manifest) return { status: 'missing', env, detail: `没有已审核的视觉基准（${relative(repoRoot, baselineDir(env))}/manifest.json 不存在）` };
  // In CI the committed lock file names the approved baseline set by digest.
  const expected = process.env.UI_BASELINE_EXPECTED_DIGEST;
  if (expected !== undefined && manifest.digest !== expected)
    return { status: 'unapproved', env, detail: expected ? `基准摘要 ${manifest.digest} 与提交的审核锁 ${expected} 不一致` : '提交的审核锁为空，CI 视觉基准尚未审核', manifest };
  const actual = hashBaselines(env);
  const problems = [];
  for (const [file, hash] of Object.entries(manifest.files)) {
    if (!actual[file]) problems.push(`缺失 ${file}`);
    else if (actual[file] !== hash) problems.push(`与审核记录不一致 ${file}`);
  }
  for (const file of Object.keys(actual)) if (!manifest.files[file]) problems.push(`未经审核 ${file}`);
  return problems.length ? { status: 'tampered', env, detail: problems.slice(0, 20).join('；'), manifest } : { status: 'ok', env, manifest };
}
