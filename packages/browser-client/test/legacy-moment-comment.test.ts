import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';
import {BrowserClient} from '../src/index.ts';

test('legacy comments preserve an explicit retry key and unkeyed compatibility',async()=>{
 const calls:Array<{url:string;body:unknown}>=[];
 const receipt={id:'comment-1',moment_id:'moment/one',author_actor_id:'owner',text:'same'};
 const client=new BrowserClient('http://fluctlight.local',async(url,init)=>{calls.push({url:String(url),body:JSON.parse(String(init?.body))});return Response.json(receipt);});
 assert.deepEqual(await client.commentOnMoment('moment/one','same','original-key'),receipt);
 await client.commentOnMoment('moment/one','same','original-key');
 await client.commentOnMoment('moment/one','same');
 assert.deepEqual(calls[0],calls[1]);assert.match(calls[0].url,/moments\/moment%2Fone\/comments$/);
 assert.deepEqual(calls[0].body,{text:'same',idempotencyKey:'original-key'});
 assert.deepEqual(calls[2].body,{text:'same'});
 const api=JSON.parse(await readFile(new URL('../openapi.json',import.meta.url),'utf8'));
 const request=api.components.schemas.BrowserLegacyMomentCommentRequest;
 assert.equal(request.properties.text.maxLength,32000);assert.deepEqual(request.required,['text']);assert.equal(request.properties.idempotencyKey.maxLength,256);
 assert.equal(api.paths['/api/moments/{momentId}/comments'].post.requestBody.content['application/json'].schema.$ref,'#/components/schemas/BrowserLegacyMomentCommentRequest');
 assert.ok(api.paths['/api/moments/{momentId}/comments'].post.responses['409']);
});
