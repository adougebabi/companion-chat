import assert from 'node:assert/strict';
import test from 'node:test';
import {BrowserClient} from '../src/index.ts';
test('structured wardrobe operations preserve scope, metadata, CAS and explicit command identity',async()=>{
 const calls:Array<{url:string;method:string;body:Record<string,unknown>}>=[];
 const client=new BrowserClient('http://fixture.invalid',async(input,init)=>{calls.push({url:String(input),method:init?.method||'GET',body:init?.body?JSON.parse(String(init.body)):{}});return Response.json({tags:['在家'],revision:4,status:'queued'});});
 const command={expectedRevision:4,reason:'维护穿搭',idempotencyKey:'original-command'};
 await client.saveInventoryTags('role/one',{...command,tags:['在家','外出'],renames:{居家:'在家'}});
 await client.renameInventoryOutfit('role/one','plan/two',{...command,name:'新名称',profileId:'profile-a',expectedOutfitRevision:3});
 await client.prepareInventory('role/one',command);
 await client.inventoryPreparation('role/one');
 assert.match(calls[0].url,/role%2Fone\/inventory\/tags$/);assert.equal(calls[0].method,'PUT');assert.deepEqual(calls[0].body.renames,{居家:'在家'});
 assert.match(calls[1].url,/outfits\/plan%2Ftwo$/);assert.equal(calls[1].method,'PATCH');assert.equal(calls[1].body.expectedOutfitRevision,3);assert.equal(calls[1].body.itemIds,undefined);assert.equal(calls[1].body.idempotencyKey,'original-command');
 assert.equal(calls[2].method,'POST');assert.deepEqual(calls[2].body,command);assert.equal(calls[3].method,'GET');
});
