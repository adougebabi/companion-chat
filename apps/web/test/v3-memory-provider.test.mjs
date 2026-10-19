import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';

// This test uses an explicit HTTP transport fixture. It does not certify Core,
// persisted sources, or real model behavior; the PG and browser cases do that.
test('memory adapter scopes every operation and reads back actual revision without a detail-summary fallback',async()=>{
  const previous={window:globalThis.window,document:globalThis.document};
  globalThis.window={location:{origin:'http://memory.test'}};
  globalThis.document={cookie:'fluctlight_csrf=fixture',addEventListener(){},createElement(){return {};}};
  const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
  const calls=[];let fail=false,foreign=false,conflict=false;
  let row={id:'mem-1',owner_fluctlight_id:'fl-1',content:'旧记忆',revision:0,status:'active'};
  const json=(value,status=200)=>new Response(JSON.stringify(value),{status,headers:{'Content-Type':'application/json'}});
  const fetcher=async(url,init={})=>{
    const parsed=new URL(String(url)),path=parsed.pathname,body=init.body?JSON.parse(init.body):undefined;calls.push({path,query:parsed.searchParams,body,method:init.method||'GET'});
    if(fail)return json({code:'memory_read_failed',message:'explicit dependency failure'},503);
    if(path.endsWith('/fl-1/memories'))return json({items:[row],next_cursor:'opaque-page'});
    if(path.endsWith('/sources'))return json({items:[{memory_revision:0,live:false,status:'unknown'}],next_cursor:''});
    if(path.endsWith('/revisions'))return json({items:[{revision:0,content:'旧记忆'}],next_cursor:''});
    if(path==='/api/memories/mem-1'){
      if(init.method==='PUT'){
        if(conflict)return json({code:'memory_revision_conflict',message:'Version changed'},409);
        row={...row,content:body.content,revision:1};return json({memory_id:row.id,revision:1});
      }
      return json({...row,owner_fluctlight_id:foreign?'fl-other':row.owner_fluctlight_id});
    }
    if(path.endsWith('/rollback')){row={...row,revision:2,content:'旧记忆'};return json({memory_id:row.id,revision:2});}
    throw new Error('Unexpected route '+path);
  };
  try{
    const {BrowserClient}=await vite.ssrLoadModule('/@fs/'+fileURLToPath(new URL('../../../packages/browser-client/src/index.ts',import.meta.url)));
    const {createMemoryProvider}=await vite.ssrLoadModule('/src/v3/providers/memory.ts');
    const store=await vite.ssrLoadModule('/src/v3/proto/store.js');store.replaceSnapshot({...store.get(),session:{actorId:'owner-1'},characters:{'fl-1':{id:'fl-1',status:'正常'}}});
    const tells=[];const memory=createMemoryProvider(new BrowserClient('http://memory.test',fetcher),async(...args)=>{tells.push(args);});
    const page=await memory.list('fl-1',{q:'百分号%',type:'semantic',status:'forgotten',cursor:'cursor',limit:30});assert.equal(page.next_cursor,'opaque-page');assert.equal(calls[0].query.get('q'),'百分号%');assert.equal(calls[0].query.get('status'),'forgotten');
    await memory.sources('fl-1','mem-1',0);assert.equal(calls.at(-1).query.get('revision'),'0');
    const original=await memory.read('fl-1','mem-1');await memory.revise('fl-1',original,'新记忆','更正说明');assert.equal(row.content,'新记忆');
    const write=calls.find(c=>c.method==='PUT');assert.equal(write.body.expectedRevision,0);assert.deepEqual(write.body.evidenceRefs,['owner:owner-1:更正说明']);assert.equal(calls.at(-1).method,'GET');
    conflict=true;await assert.rejects(memory.revise('fl-1',original,'保留的草稿','并发测试'),error=>error.status===409);assert.equal(row.content,'新记忆');conflict=false;
    await memory.rollback('fl-1',row,0,'恢复原版');assert.equal(calls.find(c=>c.path.endsWith('/rollback')).body.targetRevision,0);
    await memory.tell('fl-1','mem-1','这条有误');assert.equal(tells[0][0],'fl-1');assert.match(tells[0][1],/旧记忆.*第 2 版/);
    foreign=true;const before=calls.filter(c=>c.method!=='GET').length;await assert.rejects(memory.revise('fl-1',row,'不应写入','错误范围'),/不属于/);assert.equal(calls.filter(c=>c.method!=='GET').length,before);foreign=false;
    fail=true;await assert.rejects(memory.list('fl-1',{}),/explicit dependency failure/);assert.equal(calls.some(c=>c.path.endsWith('/detail')),false);
  }finally{await vite.close();globalThis.window=previous.window;globalThis.document=previous.document;}
});
