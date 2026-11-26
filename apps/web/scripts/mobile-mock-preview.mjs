#!/usr/bin/env node
// Production React pages against an isolated same-origin HTTP fixture. The
// fixture never proxies to Core or a provider; no production data is written.
import {spawn} from 'node:child_process';
import {fileURLToPath} from 'node:url';
const cwd=fileURLToPath(new URL('../',import.meta.url));
const port=Number(process.env.UI_MOBILE_PORT || 5197);
const apiPort=Number(process.env.UI_MOBILE_FIXTURE_PORT || 5196);
if(!Number.isInteger(port)||!Number.isInteger(apiPort)||port<1024||port>65535||apiPort<1024||apiPort>65535||port===apiPort)throw new Error('Mock ports must be distinct integers from 1024 to 65535.');
const children=[];let stopping=false;
function stop(code=0){if(stopping)return;stopping=true;for(const child of children)child.kill('SIGTERM');process.exitCode=code;}
function start(args,env){
  const child=spawn(process.execPath,args,{cwd,env:{...process.env,...env},stdio:'inherit'});
  children.push(child);
  child.on('error',error=>{console.error(error.message);stop(1);});
  child.on('exit',(code)=>{if(!stopping)stop(code || 1);});
}
start(['test/fixtures/ui-quality-server.mjs'],{
  FLUCTLIGHT_UI_FIXTURE_PORT:String(apiPort),
  ...Object.fromEntries(['CREATION','PROFILE','WORLD','PEOPLE','FEED','MEDIA','STUDIO','MANAGEMENT','DIARY','GROWTH'].map(name=>['FLUCTLIGHT_UI_FIXTURE_'+name,'normal'])),
  FLUCTLIGHT_UI_FIXTURE_WARDROBE_WORLD:'1',
});
start(['node_modules/vite/bin/vite.js','--port',String(port),'--strictPort','--host','127.0.0.1'],{
  VITE_API_SCENARIOS:'0',VITE_DEV_API_PROXY_TARGET:`http://127.0.0.1:${apiPort}`,VITE_DEV_API_PROXY_ORIGIN:`http://127.0.0.1:${port}`,
});
console.log(`Mobile mock preview: http://127.0.0.1:${port}/#/library (local fixture only)`);
process.on('SIGINT',()=>stop());process.on('SIGTERM',()=>stop());
