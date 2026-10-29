// Only for this task's browser scripts. Never reuses or terminates a listener.
import assert from 'node:assert/strict';
import net from 'node:net';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
export async function startOwnedFixture({port,env={}}){
  assert.ok(Number.isSafeInteger(port)&&port>=1024&&port<=65535);
  const base=`http://127.0.0.1:${port}`,probe=net.createServer();
  await new Promise((ok,no)=>{probe.once('error',no);probe.listen(port,'127.0.0.1',ok);});await new Promise(ok=>probe.close(ok));
  const child=spawn(process.execPath,['apps/web/test/fixtures/ui-quality-server.mjs'],{cwd:process.cwd(),env:{...Object.fromEntries(Object.entries(process.env).filter(([k])=>!k.startsWith('FLUCTLIGHT_UI_FIXTURE_'))),...env,FLUCTLIGHT_UI_FIXTURE_PORT:String(port)},stdio:['ignore','pipe','pipe']});
  const stop=async()=>{if(child.exitCode!==null||child.signalCode!==null||!child.pid)return;const done=once(child,'exit');child.kill('SIGTERM');await done;};
  try{
    await new Promise((ok,no)=>{let stdout='',stderr='';const timer=setTimeout(()=>no(Error('fixture startup timeout: '+stderr)),10000);const fail=e=>{clearTimeout(timer);no(e);};child.once('error',fail);child.once('exit',(code,signal)=>fail(Error('fixture exit '+(signal||code)+': '+stderr)));child.stderr.on('data',v=>{stderr+=v;});child.stdout.on('data',v=>{stdout+=v;if(stdout.split('\n').includes('Isolated UI fixture: '+base)){clearTimeout(timer);ok();}});});
    return {base,stop};
  }catch(e){await stop();throw e;}
}
