import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';
import ts from 'typescript';
const source=await readFile(new URL('../src/index.ts',import.meta.url),'utf8');
const js=ts.transpileModule(source,{compilerOptions:{target:ts.ScriptTarget.ES2022,module:ts.ModuleKind.ESNext}}).outputText;
const {CoreClient}=await import(`data:text/javascript;base64,${Buffer.from(js).toString('base64')}`);
test('Core legacy comment retries bind the original human session, target and key',async()=>{
 const calls=[];const receipt={id:'comment-1',moment_id:'moment/one',author_actor_id:'owner',text:'same'};
 const client=new CoreClient('http://core.invalid','fixture-service',async(url,init)=>{calls.push({url:String(url),headers:init.headers,body:JSON.parse(init.body)});return Response.json(receipt);});
 assert.deepEqual(await client.commentOnMoment('session','moment/one','same','original-key'),receipt);
 await client.commentOnMoment('session','moment/one','same','original-key');await client.commentOnMoment('session','moment/one','same');
 assert.deepEqual(calls[0],calls[1]);assert.match(calls[0].url,/moments\/moment%2Fone\/comments$/);assert.equal(calls[0].headers['x-fluctlight-human-session'],'session');
 assert.deepEqual(calls[0].body,{text:'same',idempotency_key:'original-key'});assert.deepEqual(calls[2].body,{text:'same'});
 const api=JSON.parse(await readFile(new URL('../openapi.json',import.meta.url),'utf8'));const request=api.components.schemas.CoreLegacyMomentCommentRequest;
 assert.equal(request.properties.text.maxLength,32000);assert.deepEqual(request.required,['text']);assert.equal(request.properties.idempotency_key.maxLength,256);
 assert.equal(api.paths['/internal/moments/{moment_id}/comments'].post.requestBody.content['application/json'].schema.$ref,'#/components/schemas/CoreLegacyMomentCommentRequest');
});
