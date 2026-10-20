import test from 'node:test';
import assert from 'node:assert/strict';
import {BrowserClient,BrowserApiError} from '../src/index.ts';

test('session timeout aborts an unresponsive read and never becomes a logged-out result',async()=>{
 let aborted=false;
 const client=new BrowserClient('http://example.test',((_url,init)=>new Promise<Response>(()=>{init?.signal?.addEventListener('abort',()=>{aborted=true;});})) as typeof fetch,5);
 await assert.rejects(client.session(),(error:unknown)=>error instanceof BrowserApiError&&error.code==='browser_request_timeout'&&error.status===0);
 assert.equal(aborted,true);
});
test('JSON deadline includes a body that stalls after successful headers',async()=>{
 const response=Response.json({});response.json=()=>new Promise(()=>{});
 const client=new BrowserClient('http://example.test',(async()=>response) as typeof fetch,5);
 await assert.rejects(client.health('/health/ready'),(error:unknown)=>error instanceof BrowserApiError&&error.code==='browser_request_timeout');
});
