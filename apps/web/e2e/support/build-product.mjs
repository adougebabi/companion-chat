// UI regressions must also exercise the emitted CSS, whose optimisation can
// differ from Vite dev mode. Build once before any browser worker starts.
import {execFileSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';
export default function buildProduct(){
  const web=fileURLToPath(new URL('../../',import.meta.url));
  try {execFileSync(process.execPath,['node_modules/vite/bin/vite.js','build'],{cwd:web,stdio:'pipe'});}
  catch(error){throw new Error(`Production UI build failed:\n${error.stdout || ''}\n${error.stderr || ''}`,{cause:error});}
}
