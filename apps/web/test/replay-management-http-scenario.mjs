import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {act,installDOM,mountReact} from './helpers/react-dom.mjs';

const base=process.env.FLUCTLIGHT_M4_HTTP_BASE;
const correlation=process.env.FLUCTLIGHT_M4_REPLAY_CORRELATION;
const published=process.env.FLUCTLIGHT_M4_REPLAY_MESSAGE;
assert.equal(new URL(base).hostname,'127.0.0.1');
assert.ok(correlation&&published);
const environment=installDOM();
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
let view;
try {
  const {BrowserClient}=await vite.ssrLoadModule('/@fs/'+fileURLToPath(new URL('../../../packages/browser-client/src/index.ts',import.meta.url)));
  const {installDiagnosticsProvider}=await vite.ssrLoadModule('/src/v3/providers/diagnostics.ts');
  const {default:Replay}=await vite.ssrLoadModule('/src/react-app/shared/DiagnosticReplayPanel.tsx');
  const requests=[];
  const client=new BrowserClient(base,(url,options={})=>{
    requests.push({url:new URL(url).pathname,method:options.method||'GET'});
    const headers=new Headers(options.headers);
    headers.set('cookie','fluctlight_session=session; fluctlight_csrf=csrf');
    headers.set('origin','http://fluctlight.local');
    headers.set('x-csrf-token','csrf');
    return fetch(url,{...options,headers});
  });
  const state=proxy({session:{actorId:'human-1'}});
  const workspace={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),services:{diagnosticsClient:installDiagnosticsProvider(client)}};
  const button=text=>[...view.host.querySelectorAll('button')].find(node=>node.textContent.trim()===text);
  const wait=async predicate=>{
    for(let attempt=0;attempt<100;attempt++){
      let ready;await act(async()=>{ready=predicate();});
      if(ready)return;
      await view.flush();
    }
    throw Error('Replay sources did not finish: '+view.host.textContent);
  };
  view=await mountReact(vite,Replay,{workspace,props:{correlationId:correlation}});
  await wait(()=>!view.host.textContent.includes('正在读取来源'));
  for(;;){
    assert.equal(view.host.querySelector('[role=alert]'),null,'real source read failed');
    const more=[...view.host.querySelectorAll('button')].find(node=>node.textContent.startsWith('继续读取'));
    if(!more)break;
    await view.click(more);
    await wait(()=>!view.host.textContent.includes('正在读取来源'));
  }
  const seen=new Set();
  let model=false,tool=false,publication=false,configuration=false,slots=false;
  for(;;){
    const stored=view.host.querySelector('[data-testid=replay-record] pre');
    assert.ok(stored,'no actual records were returned');
    const record=JSON.parse(stored.textContent),data=record.data;
    assert.ok(!seen.has(record.kind+':'+record.ref),'playback repeated a record');
    seen.add(record.kind+':'+record.ref);
    if(record.kind==='model-runs'){
      assert.equal(data.model_id,'m4-primary');
      assert.ok(Object.hasOwn(data,'prompt')&&Object.hasOwn(data,'response'));
      model=true;
    }
    if(record.kind==='events'&&data.event_type.startsWith('adk.tool.'))tool=true;
    if(record.kind==='events'&&data.event_type==='provider.physical_attempt'){
      configuration ||= data.payload?.model_configuration?.model_id==='m4-primary'&&data.payload?.model_configuration?.binding_revision>=1;
      slots ||= Array.isArray(data.payload?.prompt_slots)&&data.payload.prompt_slots.length>0;
    }
    publication ||= JSON.stringify(record).includes(published);
    if(button('下一条记录').disabled)break;
    await view.click(button('下一条记录'));
  }
  assert.ok(model&&tool&&publication&&configuration&&slots,JSON.stringify({model,tool,publication,configuration,slots}));
  if(seen.size>1)await view.click(button('上一条记录'));
  assert.ok(requests.length>=9);
  assert.ok(requests.every(request=>request.method==='GET'&&request.url.startsWith('/api/diagnostic-records/')));
  console.log('M4_REPLAY_HTTP',JSON.stringify({records:seen.size,model,tool,publication,configuration,slots,requests:requests.length,all_get:true}));
}finally{
  if(view)await view.dispose();
  await vite.close();environment.restore();
}
