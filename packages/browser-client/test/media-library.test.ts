import test from 'node:test';
import assert from 'node:assert/strict';
import {BrowserClient} from '../src/index.ts';
test('reference first-page reads omit undefined cursor and retain exact target scope',async()=>{
 const calls:URL[]=[];const client=new BrowserClient('http://reference.test',async input=>{calls.push(new URL(String(input)));return Response.json({items:[],next_cursor:''});});
 await client.mediaReferenceGrants({target_id:'resource/one',cursor:undefined});assert.equal(calls[0].searchParams.has('cursor'),false);assert.equal(calls[0].searchParams.get('target_id'),'resource/one');
 await client.mediaReferenceGrants({asset_id:'image',cursor:'actual|cursor'});assert.equal(calls[1].searchParams.get('cursor'),'actual|cursor');
});
test('the shared v3 client extension provides the typed item-photo transport and real request envelope',async()=>{
 let actualURL='',actualBody:unknown;const client=new BrowserClient('http://reference.test',async(input,init)=>{actualURL=String(input);actualBody=JSON.parse(String(init?.body));return Response.json({request:{id:'actual-request',fluctlight_id:'role'},audit_id:'actual-audit'},{status:202});});
 const body={prompt:'Image only',expectedItemRevision:3,reason:'Explicit request',idempotencyKey:'original'};const receipt=await client.requestItemPhoto('role','item/one',body);assert.match(actualURL,/wardrobe\/items\/item%2Fone\/photos$/);assert.deepEqual(actualBody,body);assert.equal(receipt.request.id,'actual-request');
});
