import test from 'node:test';
import assert from 'node:assert/strict';
import {BrowserClient} from '../src/index.ts';

test('creation template and raw inputs use the shared client transport without field rewriting',async()=>{
 const calls:Array<{url:URL;body:unknown}>=[];
 const client=new BrowserClient('http://catalogue.test',async(input,init)=>{
  calls.push({url:new URL(String(input)),body:init?.body?JSON.parse(String(init.body)):null});
  return Response.json({});
 });
 await client.listStudioResources({kind:'character_template',q:'%_ 角色',published:true,cursor:'cursor|id',limit:1});
 assert.equal(calls[0].url.searchParams.get('published'),'true');assert.equal(calls[0].url.searchParams.get('q'),'%_ 角色');
 await client.studioPublication('res/one','version/one');assert.match(calls[1].url.pathname,/res%2Fone\/versions\/version%2Fone/);
 await client.analyzeFluctlightCreation({resourceId:'res',resourceVersion:'1'});assert.deepEqual(calls[2].body,{resourceId:'res',resourceVersion:'1'});
 await client.analyzeFluctlightCreation({json:{core_persona:{customCamelField:'source'}}});assert.deepEqual(calls[3].body,{json:{core_persona:{customCamelField:'source'}}});
});
