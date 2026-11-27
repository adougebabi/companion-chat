// Loaded with `node --import ./test/support/register.mjs --test …`; node passes
// the flag on to every test-file process. Installs module-hooks.mjs in-thread
// (module.registerHooks: no loader worker round trip per resolved file) and
// records which project files each test file loaded or read, so `test:changed`
// can run only the tests a change can affect (scripts/unit-scope.mjs).
import fs from 'node:fs';
import { registerHooks, syncBuiltinESMExports } from 'node:module';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { cacheRoot, depsDir, webRoot, workspaceRoot } from './cache-root.mjs';
import { createHooks } from './module-hooks.mjs';

// The values a Vite dev server exposes to SSR modules (the app has no .env files).
const env = { BASE_URL: '/', MODE: 'development', DEV: true, PROD: false, SSR: true };
for (const [key, value] of Object.entries(process.env)) if (key.startsWith('VITE_')) env[key] = value;

const used = new Set();
function note(target) {
  try {
    const file = path.resolve(String(target instanceof URL ? fileURLToPath(target) : target));
    if (file.startsWith(workspaceRoot + path.sep) && !file.includes(`${path.sep}node_modules${path.sep}`)) used.add(path.relative(workspaceRoot, file));
  } catch {}
}

const { hooks, save } = createHooks({ webRoot, workspaceRoot, cacheDir: cacheRoot, env, onFile: note });
registerHooks(hooks);
process.on('exit', () => { try { save(); } catch {} });

const testFile = process.argv[1] && path.resolve(process.argv[1]);
if (testFile?.startsWith(path.join(webRoot, 'test') + path.sep)) {
  // Tests that assert on source text read it through fs; count those reads too.
  for (const name of ['readFileSync', 'readdirSync', 'existsSync', 'statSync']) {
    const original = fs[name];
    fs[name] = function (target, ...rest) { note(target); return original.call(this, target, ...rest); };
  }
  for (const name of ['readFile', 'readdir', 'stat']) {
    const original = fs.promises[name];
    fs.promises[name] = function (target, ...rest) { note(target); return original.call(this, target, ...rest); };
  }
  syncBuiltinESMExports();
  const record = path.join(depsDir, path.relative(webRoot, testFile).replaceAll(path.sep, '__') + '.json');
  process.on('exit', (code) => {
    // Only a complete, passing run shows everything the test depends on.
    if (code !== 0) return;
    try {
      fs.mkdirSync(path.dirname(record), { recursive: true });
      const temporary = `${record}.${process.pid}`;
      fs.writeFileSync(temporary, JSON.stringify({ test: path.relative(workspaceRoot, testFile), files: [...used].sort() }));
      fs.renameSync(temporary, record);
    } catch {}
  });
}
