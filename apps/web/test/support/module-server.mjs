// Drop-in for the slice of Vite's dev server the unit tests use:
// `createServer(config)` → `{ssrLoadModule(url), close()}`. Modules are loaded by
// node through test/support/module-hooks.mjs (registered by register.mjs), so no
// per-file Vite server is started. Like separate Vite servers, every server has
// its own instances of project modules; node_modules packages are shared, as
// with Vite's SSR externals.
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const defaultRoot = fileURLToPath(new URL('../../', import.meta.url));
let servers = 0;

export async function createServer(config = {}) {
  const root = config.root ?? defaultRoot;
  const instance = String(++servers);
  return {
    async ssrLoadModule(url) {
      const file = url.startsWith('/@fs/') ? url.slice('/@fs'.length) : path.join(root, url);
      const target = new URL(pathToFileURL(file));
      target.searchParams.set('vite-instance', instance);
      return import(target.href);
    },
    async close() {},
  };
}
