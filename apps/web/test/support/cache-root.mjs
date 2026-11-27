// Unit-test caches (compiled modules, package resolution and sources, per-test
// dependency records) live on local disk, not in the checkout (which may be
// network-mounted), one directory per checkout.
import { createHash } from 'node:crypto';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const webRoot = fileURLToPath(new URL('../../', import.meta.url)).replace(/\/$/, '');
export const workspaceRoot = path.resolve(webRoot, '../..');
export const cacheRoot = path.join(
  process.env.XDG_CACHE_HOME || path.join(os.homedir(), '.cache'),
  'fluctlight-web-unit-tests',
  createHash('sha256').update(webRoot).digest('hex').slice(0, 12),
);
export const depsDir = path.join(cacheRoot, 'deps');
