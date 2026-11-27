import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from './support/module-server.mjs';
import {fileURLToPath} from 'node:url';
import {proxy, snapshot, subscribe} from 'valtio/vanilla';
import {act, installDOM, mountReact} from './helpers/react-dom.mjs';

const environment = installDOM();
const vite = await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async () => {await vite.close();environment.restore();});
const {default:Replay} = await vite.ssrLoadModule('/src/react-app/shared/DiagnosticReplayPanel.tsx');
const button = (root, text) => [...root.querySelectorAll('button')].find(node => node.textContent.trim() === text);
const record = (ref, at, data={}, kind='events') => ({ref,kind,occurred_at:at,data});
const page = (items=[], cursor='', snapshot='fixture-snapshot') => ({items,next_cursor:cursor,snapshot});
function workspace(read) {
  const state = proxy({session:{actorId:'replay-owner'}}), reads=[], writes=[];
  const forbidden = name => () => {writes.push(name);throw Error('Unexpected side effect '+name);};
  return {state,reads,writes,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),services:{
    diagnosticsClient:{page:async(kind,query)=>{reads.push({kind,...query});return read(kind,query);},control:forbidden('workflow-control'),mediaRetry:forbidden('media-retry'),clear:forbidden('clear'),export:forbidden('export')},
    modelClient:{run:forbidden('provider')},tools:{execute:forbidden('tool')},messages:{send:forbidden('publish')},
  }};
}
const mount = w => mountReact(vite,Replay,{workspace:w,props:{correlationId:'run-parent'}});

test('readonly playback retains failed and successful attempts and separate tool phases without writes',async()=>{
  const w=workspace(async kind=>kind==='events'?page([
    record('request','2026-11-13T01:00:00Z',{event_type:'adk.tool.requested',call_id:'same-call',arguments:{item:'fixture'}}),
    record('failure','2026-11-13T01:00:01Z',{event_type:'adk.tool.result',call_id:'same-call',status:'failed'}),
    record('success','2026-11-13T01:00:02Z',{event_type:'adk.tool.result',call_id:'same-call',status:'completed'}),
    record('publication','2026-11-13T01:00:03Z',{event_type:'reply.tool.result',committed:true}),
  ]):page());
  const view=await mount(w);
  try {
    const shown=()=>view.host.querySelector('[data-testid=replay-record]').textContent;
    assert.match(shown(),/adk.tool.requested/);
    await view.click(button(view.host,'下一条记录'));assert.match(shown(),/failed/);
    await view.click(button(view.host,'下一条记录'));assert.match(shown(),/completed/);
    await view.click(button(view.host,'下一条记录'));assert.match(shown(),/committed/);
    assert.equal(button(view.host,'下一条记录').disabled,true);
    await view.click(button(view.host,'上一条记录'));assert.match(shown(),/completed/);
    assert.equal(w.reads.length,9);
    assert.ok(w.reads.every(read=>read.correlationId==='run-parent'&&read.limit===30));
    assert.deepEqual(w.writes,[]);
  } finally {await view.dispose();}
});

test('pagination failure keeps confirmed records and retries the exact opaque cursor',async()=>{
  let fail=true;
  const w=workspace(async(kind,query)=>{
    if(kind!=='events')return page();
    if(!query.cursor)return page([record('retained','2026-11-13T01:00:01Z')],'opaque:/next?');
    if(fail)throw Error('fetch failed');
    return page([record('earlier','2026-11-13T01:00:00Z')]);
  });
  const view=await mount(w);
  try {
    assert.match(view.host.textContent,/1 个来源仍有后续页/);
    await view.click(button(view.host,'继续读取事件与工具'));
    assert.match(view.host.textContent,/读取失败，已读记录保留/);
    assert.match(view.host.querySelector('[data-testid=replay-record]').textContent,/retained/);
    fail=false;await view.click(button(view.host,'重试事件与工具'));
    assert.match(view.host.textContent,/已读取 2 条记录/);
    assert.match(view.host.querySelector('[data-testid=replay-record]').textContent,/retained/);
    assert.deepEqual(w.reads.filter(read=>read.cursor).map(read=>read.cursor),['opaque:/next?','opaque:/next?']);
    await view.click(button(view.host,'上一条记录'));
    assert.match(view.host.querySelector('[data-testid=replay-record]').textContent,/earlier/);
    assert.deepEqual(w.writes,[]);
  } finally {await view.dispose();}
});

test('missing content, absent time and empty sources keep their actual storage meanings',async()=>{
  const w=workspace(async kind=>kind==='model-runs'?page([record('missing-time',null,{prompt:null,status:'completed'},kind)]):page());
  const view=await mount(w);
  try {
    assert.match(view.host.textContent,/时间未记录/);
    assert.match(view.host.textContent,/缺失或已清理/);
    assert.match(view.host.querySelector('pre').textContent,/"prompt": null/);
    assert.doesNotMatch(view.host.textContent,/已发送|已交付|回放完整/);
  } finally {await view.dispose();}
});

test('owner and correlation changes discard unresolved previous scope results',async()=>{
  const pending=[];
  const w=workspace((kind,query)=>new Promise(resolve=>pending.push({kind,query,resolve})));
  const view=await mount(w);
  try {
    const original=pending.slice();
    await act(async()=>{w.state.session.actorId='another-owner';});await view.flush();
    const previous=pending.filter(entry=>!original.includes(entry));
    await view.update({correlationId:'run-new'});
    await act(async()=>{for(const entry of [...original,...previous])entry.resolve(page([record('STALE','2026-11-13T01:00:00Z',{},entry.kind)]));});await view.flush();
    assert.doesNotMatch(view.host.textContent,/STALE/);
    const current=pending.filter(entry=>entry.query.correlationId==='run-new');
    await act(async()=>{for(const entry of current)entry.resolve(page(entry.kind==='events'?[record('CURRENT',null)]:[]));});await view.flush();
    assert.match(view.host.textContent,/CURRENT/);assert.doesNotMatch(view.host.textContent,/STALE/);
    assert.deepEqual(w.writes,[]);
  } finally {await view.dispose();}
});

test('snapshot change and nonadvancing cursor retain earlier evidence and explicit incompleteness',async()=>{
  for(const mode of ['snapshot','cursor']) {
    const w=workspace(async(kind,query)=>kind!=='events'?page():!query.cursor?page([record('confirmed',null)],'next'):page([record('UNVERIFIED',null)],mode==='cursor'?'next':'',mode==='snapshot'?'changed':'fixture-snapshot'));
    const view=await mount(w);
    try {
      await view.click(button(view.host,'继续读取事件与工具'));
      assert.match(view.host.textContent,/读取失败/);
      assert.match(view.host.textContent,/confirmed/);assert.doesNotMatch(view.host.textContent,/UNVERIFIED/);
    } finally {await view.dispose();}
  }
});

test('permission denial stays explicit across every source',async()=>{
  const w=workspace(async()=>{throw Object.assign(Error('disabled'),{code:'diagnostic_debug_read_disabled'});});
  const view=await mount(w);
  try {
    assert.match(view.host.textContent,/9 个来源读取失败/);
    assert.match(view.host.textContent,/诊断读取未开启，需要管理员在服务端开启后再查看/);
    assert.doesNotMatch(view.host.textContent,/FLUCTLIGHT_DIAGNOSTIC_DEBUG_READ|重启 API/);
    assert.equal(view.host.querySelector('[data-testid=replay-record]'),null);
    assert.doesNotMatch(view.host.textContent,/没有可回放记录/);
    assert.deepEqual(w.writes,[]);
  } finally {await view.dispose();}
});
