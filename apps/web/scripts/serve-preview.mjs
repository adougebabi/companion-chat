// Local-only helper for previewing a Vite production build. The production
// image serves the build through Nginx instead.
import { spawn } from 'node:child_process';
import { writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';

import { runtimeConfigSource } from './runtime-config.mjs';

const apiOrigin = process.env.FLUCTLIGHT_API_ORIGIN?.trim() ?? "";

await writeFile(
  resolve('apps/web/dist/runtime-config.js'),
  runtimeConfigSource(apiOrigin),
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
