import assert from 'node:assert/strict';
import {execFile} from 'node:child_process';
import {createHash} from 'node:crypto';
import {cp,mkdir,mkdtemp,readFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';
import {fileURLToPath} from 'node:url';
import {promisify} from 'node:util';
import {BrowserClient} from '../src/index.ts';

const execute=promisify(execFile);
const digest=(value:Uint8Array)=>createHash('sha256').update(value).digest('hex');

test('expression learning client preserves scoped paths, request identities, and review actions',async()=>{
 const calls:Array<{url:URL;method:string;body:unknown}>=[];
 const client=new BrowserClient('http://local.test',async(input,init)=>{
  calls.push({url:new URL(String(input)),method:init?.method??'GET',body:init?.body?JSON.parse(String(init.body)):null});
  return Response.json({status:'completed',candidate_ids:[],no_effect:false});
 });
 const settings={learningEnabled:true,useEnabled:false,expectedRevision:0,idempotencyKey:'settings-key'};
 const extraction={sourceMessageId:'message/1',idempotencyKey:'extract-key'};
 const review={expectedRevision:1,reason:'审核',triggerText:'进度',idempotencyKey:'review-key'};
 await client.expressionLearning('character/1');
 await client.expressionLearningSources('character/1','cursor/1');
 await client.expressionLearningSource('character/1','message/1');
 await client.updateExpressionLearning('character/1',settings);
 await client.extractExpressionLearning('character/1',extraction);
 await client.expressionExtractionStatus('character/1','extract/key');
 for(const action of ['enable','disable','revoke'] as const)await client.reviewExpressionLearning('character/1','candidate/1',action,review);
 const base='/api/fluctlights/character%2F1/expression-learning';
 assert.deepEqual(calls.map(call=>`${call.method} ${call.url.pathname}`),[
  `GET ${base}`,`GET ${base}/sources`,`GET ${base}/sources/message%2F1`,
  `PUT ${base}/settings`,`POST ${base}/extractions`,`GET ${base}/extractions/by-key`,
  ...['enable','disable','revoke'].map(action=>`POST ${base}/candidates/candidate%2F1/${action}`),
 ]);
 assert.equal(calls[1].url.searchParams.get('cursor'),'cursor/1');
 assert.equal(calls[5].url.searchParams.get('key'),'extract/key');
 assert.deepEqual(calls[3].body,settings);
 assert.deepEqual(calls[4].body,extraction);
 assert.deepEqual(calls[6].body,review);
});

test('expression routes and client survive regeneration from their source contracts',async()=>{
 const root=fileURLToPath(new URL('../',import.meta.url));
 const api=JSON.parse(await readFile(join(root,'openapi.json'),'utf8'));
 const base='/api/fluctlights/{fluctlightId}/expression-learning';
 const expected:Array<[string,string]>=[['','get'],['/sources','get'],['/sources/{messageId}','get'],['/settings','put'],['/extractions','post'],['/extractions/by-key','get'],...['enable','disable','revoke'].map(action=>[`/candidates/{candidateId}/${action}`,'post'] as [string,string])];
 for(const [path,method] of expected)assert.ok(api.paths[base+path]?.[method],`${method} ${base+path}`);
 assert.equal(api.paths[base+'/settings'].put.requestBody.content['application/json'].schema.$ref,'#/components/schemas/BrowserExpressionSettingsBody');
 assert.equal(api.paths[base+'/candidates/{candidateId}/enable'].post.requestBody.content['application/json'].schema.$ref,'#/components/schemas/BrowserExpressionEnableBody');
 assert.ok(api.components.schemas.BrowserExpressionEnableBody.required.includes('triggerText'));
 assert.equal(api.paths[base+'/extractions/by-key'].get.parameters.find((item:{name:string})=>item.name==='key').required,true);
 const temporary=await mkdtemp(join(tmpdir(),'fluctlight-client-generation-'));
 try{
  await cp(join(root,'scripts'),join(temporary,'scripts'),{recursive:true});
  await mkdir(join(temporary,'src'));
  await execute(process.execPath,[join(temporary,'scripts/generate-openapi.mjs')]);
  assert.equal(digest(await readFile(join(temporary,'openapi.json'))),digest(await readFile(join(root,'openapi.json'))));
  await execute(process.execPath,[join(temporary,'scripts/generate.mjs')]);
  assert.equal(digest(await readFile(join(temporary,'src/index.ts'))),digest(await readFile(join(root,'src/index.ts'))));
 }finally{await rm(temporary,{recursive:true,force:true});}
});
