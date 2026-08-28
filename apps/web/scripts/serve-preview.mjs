// Local-only helper for previewing a Vite production build. The production
// image serves the build through Nginx instead.
import { spawn } from 'node:child_process';
import { writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';

import { runtimeConfigSource } from './runtime-config.mjs';

const bffOrigin = process.env.FLUCTLIGHT_BFF_ORIGIN?.trim();
if (!bffOrigin) {
  throw new Error('FLUCTLIGHT_BFF_ORIGIN must be set before starting the Web container');
}

await writeFile(
  resolve('apps/web/dist/runtime-config.js'),
  runtimeConfigSource(bffOrigin),
  'utf8',
);

const preview = spawn(
  'pnpm',
  ['--filter', '@fluctlight/web', 'exec', 'vite', 'preview', '--host', '0.0.0.0', '--port', '4173'],
  { stdio: 'inherit' },
);

for (const signal of ['SIGINT', 'SIGTERM']) {
  process.on(signal, () => preview.kill(signal));
}

const exitCode = await new Promise((resolveExit) => {
  preview.once('exit', (code) => resolveExit(code ?? 1));
});
process.exit(exitCode);
