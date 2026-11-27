// Synchronous, in-thread node module hooks for the unit tests (installed by
// register.mjs through module.registerHooks). They compile TS/TSX/JSX, and
// project JS that reads import.meta.env, with esbuild; resolve the "@/" alias
// and the extensionless / ".js"-for-".ts" imports Vite allows; stub CSS; and
// give each test "server" its own project-module instances via the
// ?vite-instance=N query. Compiled output is cached on disk by content hash, so
// the test processes share one compilation instead of each starting a Vite
// dev server. node_modules packages load natively, like Vite's SSR externals.
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

// The esbuild Vite itself depends on, i.e. the compiler these tests used before.
const { transformSync, version: esbuildVersion } = createRequire(createRequire(import.meta.url).resolve('vite'))('esbuild');

const HOOKS_VERSION = '1';
const INSTANCE = 'vite-instance';
const PROBE = ['.ts', '.tsx', '.js', '.jsx', '.mjs'];
const COMPILED = new Set(['.ts', '.tsx', '.jsx', '.mts']);
const SCRIPT = new Set(['.js', '.mjs']);
const tsconfigRaw = { compilerOptions: { jsx: 'react-jsx', target: 'ES2024', useDefineForClassFields: true } };

const isFile = (file) => { try { return fs.statSync(file).isFile(); } catch { return false; } };

// Package resolution is deterministic for one install, so it is remembered
// across test processes: on a slow (e.g. network-mounted) checkout each lookup
// costs several remote stats and package.json reads, and a React test resolves
// ~2000 files. Keyed by the lockfile, the install time and the node version;
// only resolutions that land in node_modules or on builtins are kept.
function createResolveMemo(file) {
  let entries = {};
  try { entries = JSON.parse(fs.readFileSync(file, 'utf8')); } catch {}
  const added = {};
  return {
    get: (key) => entries[key],
    set(key, value) { entries[key] = value; added[key] = value; },
    save() {
      if (!Object.keys(added).length) return;
      let current = {};
      try { current = JSON.parse(fs.readFileSync(file, 'utf8')); } catch {}
      const temporary = `${file}.${process.pid}`;
      fs.writeFileSync(temporary, JSON.stringify({ ...current, ...added }));
      fs.renameSync(temporary, file);
    },
  };
}

export function installKey(workspaceRoot) {
  const lock = fs.readFileSync(path.join(workspaceRoot, 'pnpm-lock.yaml'));
  const installed = fs.statSync(path.join(workspaceRoot, 'node_modules/.modules.yaml')).mtimeMs;
  return createHash('sha256').update(lock).update(`\0${installed}\0${process.version}`).digest('hex').slice(0, 16);
}

export function createHooks({ webRoot, workspaceRoot, cacheDir, env, onFile = () => {} }) {
  const optionsKey = JSON.stringify({ HOOKS_VERSION, esbuildVersion, env, tsconfigRaw });
  const define = { 'import.meta.env': JSON.stringify(env) };
  const install = installKey(workspaceRoot);
  const mirror = path.join(cacheDir, `packages-${install}`);
  fs.mkdirSync(path.join(cacheDir, 'modules'), { recursive: true });
  fs.mkdirSync(mirror, { recursive: true });
  const memo = createResolveMemo(path.join(cacheDir, `resolve-${install}.json`));
  const inProject = (file) => file.startsWith(workspaceRoot + path.sep) && !file.split(path.sep).includes('node_modules');

  function probe(file) {
    if (isFile(file)) return file;
    const ext = path.extname(file);
    if (ext === '.js' || ext === '.jsx' || ext === '.mjs') {
      const stem = file.slice(0, -ext.length);
      for (const candidate of ['.ts', '.tsx']) if (isFile(stem + candidate)) return stem + candidate;
    }
    for (const candidate of PROBE) if (isFile(file + candidate)) return file + candidate;
    for (const candidate of PROBE) if (isFile(path.join(file, 'index' + candidate))) return path.join(file, 'index' + candidate);
    return undefined;
  }

  const instanceOf = (url) => (url?.startsWith('file:') ? new URL(url).searchParams.get(INSTANCE) ?? undefined : undefined);

  function withInstance(url, instance) {
    if (!instance || !url.startsWith('file:') || !inProject(fileURLToPath(url))) return url;
    const next = new URL(url);
    next.searchParams.set(INSTANCE, instance);
    return next.href;
  }

  function resolve(specifier, context, nextResolve) {
    let instance = instanceOf(context.parentURL);
    let target;
    if (specifier.startsWith('file:')) {
      const url = new URL(specifier);
      instance = url.searchParams.get(INSTANCE) ?? instance;
      const file = probe(fileURLToPath(url));
      if (file) return { url: withInstance(pathToFileURL(file).href, instance), shortCircuit: true };
    } else if (specifier.startsWith('@/')) target = path.join(webRoot, 'src', specifier.slice(2));
    else if (context.parentURL?.startsWith('file:') && (specifier.startsWith('./') || specifier.startsWith('../') || specifier.startsWith('/'))) {
      const [bare] = specifier.split('?');
      target = specifier.startsWith('/') ? bare : fileURLToPath(new URL(bare, context.parentURL));
    }
    if (target && inProject(target)) {
      const file = probe(target);
      if (file) {
        const query = specifier.includes('?') ? specifier.slice(specifier.indexOf('?')) : '';
        return { url: withInstance(pathToFileURL(file).href + query, instance), shortCircuit: true };
      }
    }
    const bare = !specifier.startsWith('.') && !specifier.startsWith('/') && !specifier.startsWith('file:');
    const fromPackage = context.parentURL?.includes('/node_modules/');
    const memoKey = (bare || fromPackage) && context.parentURL?.startsWith('file:')
      ? `${new URL('.', context.parentURL).pathname}\0${specifier}\0${(context.conditions ?? []).join(',')}\0${JSON.stringify(context.importAttributes ?? {})}`
      : undefined;
    let result = memoKey && memo.get(memoKey);
    if (!result) {
      result = nextResolve(specifier, context);
      if (memoKey && (result.url.startsWith('node:') || result.url.includes('/node_modules/'))) memo.set(memoKey, { url: result.url, format: result.format });
    }
    return instance ? { ...result, url: withInstance(result.url, instance), shortCircuit: true } : { ...result, shortCircuit: true };
  }

  function compile(file, source, loader) {
    const key = createHash('sha256').update(optionsKey).update('\0').update(file).update('\0').update(source).digest('hex');
    const cached = path.join(cacheDir, 'modules', key + '.js');
    try { return fs.readFileSync(cached, 'utf8'); } catch {}
    const { code } = transformSync(source, {
      loader, format: 'esm', target: 'es2024', jsx: 'automatic', sourcemap: 'inline', sourcefile: file, tsconfigRaw, define,
    });
    const temporary = `${cached}.${process.pid}.${Math.random().toString(36).slice(2)}`;
    fs.writeFileSync(temporary, code);
    fs.renameSync(temporary, cached);
    return code;
  }

  // Installed package files are immutable for one install key: serve them from a
  // local copy instead of re-reading the (possibly remote) checkout every process.
  function loadPackage(url, context, nextLoad) {
    const copy = path.join(mirror, createHash('sha256').update(url).update('\0').update(JSON.stringify(context.importAttributes ?? {})).digest('hex'));
    try {
      const saved = JSON.parse(fs.readFileSync(copy, 'utf8'));
      return { format: saved.format, source: saved.source, shortCircuit: true };
    } catch {}
    const result = nextLoad(url, context);
    if (['module', 'commonjs', 'json'].includes(result.format) && result.source != null) {
      const source = typeof result.source === 'string' ? result.source : Buffer.from(result.source).toString('utf8');
      const temporary = `${copy}.${process.pid}`;
      fs.writeFileSync(temporary, JSON.stringify({ format: result.format, source }));
      fs.renameSync(temporary, copy);
    }
    return result;
  }

  function load(url, context, nextLoad) {
    if (!url.startsWith('file:')) return nextLoad(url, context);
    const file = fileURLToPath(url);
    if (url.includes('/node_modules/')) return loadPackage(url, context, nextLoad);
    if (!inProject(file)) return nextLoad(url, context);
    onFile(file);
    const ext = path.extname(file);
    if (ext === '.css') return { format: 'module', source: '', shortCircuit: true };
    if (!COMPILED.has(ext) && !SCRIPT.has(ext)) return nextLoad(url, context);
    const source = fs.readFileSync(file, 'utf8');
    if (SCRIPT.has(ext) && !source.includes('import.meta.env')) return { format: 'module', source, shortCircuit: true };
    const loader = ext === '.mts' ? 'ts' : SCRIPT.has(ext) ? 'js' : ext.slice(1);
    return { format: 'module', source: compile(file, source, loader), shortCircuit: true };
  }

  return { hooks: { resolve, load }, save: () => memo.save() };
}
