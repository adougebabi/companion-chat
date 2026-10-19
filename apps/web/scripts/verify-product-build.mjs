import { readdir, readFile, rm } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { join } from 'node:path';

// Fixture builds require explicit process-level opt-in. Never ship QA automation
// alongside the real authenticated application.
if (process.env.VITE_PRODUCT_MODE === 'mock') process.exit(0);
const output = fileURLToPath(new URL('../dist/', import.meta.url));
await rm(join(output,'v3-qa'),{recursive:true,force:true});
const forbidden=['fluctlight.v3.acceptance.mock.v1','createMockSeed','mock-correlation-','provider_timeout（Mock 示例）','reset-demo','window.__store','window.__app'];
for(const filename of await readdir(join(output,'assets'))){
  if(!filename.endsWith('.js'))continue;
  const source=await readFile(join(output,'assets',filename),'utf8');
  for(const marker of forbidden)if(source.includes(marker))throw new Error(`Production bundle contains isolated fixture marker: ${marker} (${filename})`);
}
console.log('Verified real production bundle: no fixture store, seed, diagnostic samples, debug writers or QA page.');
