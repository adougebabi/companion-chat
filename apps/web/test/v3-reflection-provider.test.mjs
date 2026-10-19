import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';

// Explicit transport fixture only; actual V2 production/consumption has its PG
// tests and live acceptance separately. A 202 remains pending here.
test('Reflection adapter keeps V2 structure, request target, scope and retry identity',async()=>{
  const old={window:globalThis.window,document:globalThis.document};globalThis.window={location:{origin:'http://reflection.test'}};globalThis.document={cookie:'fluctlight_csrf=fixture',addEventListener(){},createElement(){return {};}};
  const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
  let fail=true,foreign=false,missing=false;const calls=[];
  const json=(v,status=200)=>new Response(JSON.stringify(v),{status,headers:{'Content-Type':'application/json'}});
  const fetcher=async(url,init={})=>{
    const parsed=new URL(String(url)),path=parsed.pathname,body=init.body?JSON.parse(init.body):undefined;calls.push({path,query:parsed.searchParams,body});
    if(missing)return json({code:'reflection_not_found',message:'No scoped reflection'},404);
    if(path.endsWith('/feedback')){
      if(init.method==='POST'){if(fail){fail=false;throw new TypeError('controlled transport failure');}return json({request:{id:'request-one',kind:'reflection_feedback',fluctlight_id:'fl-one',status:'pending',payload:{reflection_id:'ref-one',text:body.text}}},202);}
      return json({items:[],next_cursor:'feedback-page'});
    }
    if(path.endsWith('/reflections'))return json({items:[{id:'ref-one',fluctlight_id:'fl-one',status:'no_change',result:{counts:{}}}],next_cursor:'ref-page',window:{watermark:17},latest_task:{status:'failed'}});
    return json({id:'ref-one',fluctlight_id:foreign?'other':'fl-one',proposal:{memory_candidates:[]},dispositions:[],source_window_status:'available',result:{status:'no_change'}});
  };
  try{
    const {BrowserClient}=await vite.ssrLoadModule('/@fs/'+fileURLToPath(new URL('../../../packages/browser-client/src/index.ts',import.meta.url)));
    const {createReflectionProvider}=await vite.ssrLoadModule('/src/v3/providers/reflection.ts');const store=await vite.ssrLoadModule('/src/v3/proto/store.js');store.replaceSnapshot({...store.get(),session:{actorId:'owner'},characters:{'fl-one':{id:'fl-one',status:'正常'}}});
    const reflection=createReflectionProvider(new BrowserClient('http://reflection.test',fetcher));const page=await reflection.list('fl-one','cursor');assert.equal(page.latest_task.status,'failed');assert.equal(calls[0].query.get('cursor'),'cursor');assert.equal(page.items[0].status,'no_change');
    const detail=await reflection.read('fl-one','ref-one');assert.deepEqual(detail.proposal.memory_candidates,[]);assert.equal(detail.source_window_status,'available');
    await assert.rejects(reflection.submit('fl-one','ref-one','明确意见','disagree'),/controlled transport/);const accepted=await reflection.submit('fl-one','ref-one','明确意见','disagree');assert.equal(accepted.request.status,'pending');
    const writes=calls.filter(c=>c.body);assert.equal(writes[0].body.idempotencyKey,writes[1].body.idempotencyKey);assert.equal(writes[1].body.stance,'disagree');
    const history=await reflection.feedback('fl-one','ref-one','old-feedback');assert.equal(history.next_cursor,'feedback-page');assert.equal(calls.at(-1).query.get('cursor'),'old-feedback');
    foreign=true;await assert.rejects(reflection.read('fl-one','ref-one'),/范围不匹配/);foreign=false;missing=true;await assert.rejects(reflection.read('fl-one','ref-one'),error=>error.status===404);
  }finally{await vite.close();globalThis.window=old.window;globalThis.document=old.document;}
});
