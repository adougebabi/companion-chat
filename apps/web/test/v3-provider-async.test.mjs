import test from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'vite';
import { fileURLToPath } from 'node:url';

test('persona provider observes the owned accepted task and aborting observation sends no cancellation',async()=>{
  const originalWindow=globalThis.window,originalStorage=globalThis.localStorage,originalDocument=globalThis.document;
  globalThis.window={location:{origin:'http://fluctlight.test'}};
  globalThis.document={addEventListener(){},createElement(){return {};}};
  globalThis.localStorage={getItem:()=>null,setItem(){},removeItem(){}};
  const server=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
  try{
    const {client,profile}=await server.ssrLoadModule('/src/v3/providers/real.ts');
    const command={targetPersonaId:'focused',expectedRevision:3,expectedRuntimeRevision:5,reason:'Owner请求',confirm:true,idempotencyKey:'stable-id'};
    let submissions=0,observations=0,cancellations=0;
    client.switchPersona=async(id,body)=>{submissions++;assert.equal(id,'fl-real');assert.deepEqual(body,command);return{request_id:'request-real',status:'pending'};};
    client.ownerRequest=async requestId=>{observations++;assert.equal(requestId,'request-real');return{fluctlight_id:'fl-real',kind:'persona_switch',status:'completed',result:{status:'completed',active_profile_id:'focused'}};};
    client.cancelTurn=async()=>{cancellations++;};
    const completed=await profile.switchPersona('fl-real',command);
    assert.equal(completed.active_profile_id,'focused');assert.equal(submissions,1);assert.equal(observations,1);
    const abort=new AbortController();abort.abort();
    await assert.rejects(profile.switchPersona('fl-real',command,{signal:abort.signal}),error=>error.name==='AbortError');
    assert.equal(submissions,2);assert.equal(observations,1);assert.equal(cancellations,0);
  }finally{await server.close();globalThis.window=originalWindow;globalThis.localStorage=originalStorage;globalThis.document=originalDocument;}
});

test('a disposed async page cannot reload over the next page in the same host',async()=>{
  const originalWindow=globalThis.window,originalStorage=globalThis.localStorage,originalDocument=globalThis.document;
  globalThis.window={location:{origin:'http://fluctlight.test'}};
  globalThis.document={addEventListener(){},createElement(){return {};}};
  globalThis.localStorage={getItem:()=>null,setItem(){},removeItem(){}};
  const server=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
  try{
    const {asyncPage}=await server.ssrLoadModule('/src/v3/proto/components/async-page.js');
    const host={isConnected:true,innerHTML:''};let reads=0,reload;
    const dispose=asyncPage(host,async()=>++reads,(_,again)=>{reload=again;host.innerHTML='first page';});
    await new Promise(resolve=>setImmediate(resolve));assert.equal(host.innerHTML,'first page');
    dispose();host.innerHTML='next page';await reload();
    assert.equal(reads,1);assert.equal(host.innerHTML,'next page');
  }finally{await server.close();globalThis.window=originalWindow;globalThis.localStorage=originalStorage;globalThis.document=originalDocument;}
});
