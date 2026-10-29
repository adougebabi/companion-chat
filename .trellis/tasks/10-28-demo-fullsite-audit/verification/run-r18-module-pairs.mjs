import {createServer} from 'node:http';
import {readFile} from 'node:fs/promises';
import {resolve,extname} from 'node:path';
import {spawn} from 'node:child_process';
import {startOwnedFixture} from './owned-fixture.mjs';
const root='/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo',base='http://127.0.0.1:4496';
const reference=createServer(async(req,res)=>{try{const path=resolve(root,'.'+decodeURIComponent(new URL(req.url,base).pathname));if(!path.startsWith(root+'/'))throw Error('outside reference');const body=await readFile(path);res.writeHead(200,{'content-type':({'.html':'text/html','.js':'text/javascript','.css':'text/css','.svg':'image/svg+xml','.png':'image/png'})[extname(path)]||'application/octet-stream'});res.end(body);}catch{res.writeHead(404);res.end();}});
await new Promise((ok,no)=>{reference.once('error',no);reference.listen(4496,'127.0.0.1',ok);});
try{for(const version of (process.env.PAIR_VERSION?[process.env.PAIR_VERSION]:['before','after'])){
 const fixture=await startOwnedFixture({port:4497,env:{FLUCTLIGHT_UI_FIXTURE_DEMO:'normal',...(version==='before'?{FLUCTLIGHT_UI_FIXTURE_DIST:'/private/tmp/fluctlight-demo-audit-20261028/r18-baseline-source/apps/web/dist'}:{})}});
 try{await new Promise((ok,no)=>{const child=spawn(process.execPath,['.trellis/tasks/10-28-demo-fullsite-audit/verification/compare-pages.mjs',process.env.PAIR_PAGES||'now,schedule,wardrobe,diary,memory,profile,world,feed,studio,library'],{env:{...process.env,AUDIT_REFERENCE_BASE:base,AUDIT_ACTUAL_BASE:fixture.base,AUDIT_CAPTURE_PHASE:(process.env.PAIR_PHASE||'r18-module')+'-'+version},stdio:'inherit'});child.once('error',no);child.once('exit',code=>code===0?ok():no(Error('comparison exited '+code)));});}
 finally{await fixture.stop();}
}}finally{await new Promise(ok=>reference.close(ok));}
