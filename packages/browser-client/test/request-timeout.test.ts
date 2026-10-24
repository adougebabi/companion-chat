import test from 'node:test';
import assert from 'node:assert/strict';
import {BrowserClient,BrowserApiError} from '../src/index.ts';
test('initialization analysis forwards explicit cancellation without serializing its signal',async()=>{
 const controller=new AbortController();let signal:AbortSignal|undefined,body='';
 const client=new BrowserClient('http://example.test',((_url,init)=>{
  signal=init?.signal??undefined;body=String(init?.body);
  return new Promise<Response>((_resolve,reject)=>{
   signal?.addEventListener('abort',()=>reject(new DOMException('aborted','AbortError')),{once:true});
  });
 }) as typeof fetch);
 const result=client.analyzeFluctlightCreation({description:'保留的原描述'},{signal:controller.signal});
 controller.abort();
 await assert.rejects(result,(error:unknown)=>error instanceof DOMException&&error.name==='AbortError');
 assert.equal(signal?.aborted,true);assert.deepEqual(JSON.parse(body),{description:'保留的原描述'});
});

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
