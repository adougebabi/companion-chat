import test from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'vite';
import { fileURLToPath } from 'node:url';

// Explicit transport fixture: verifies adapter/client behavior, not live Core,
// PostgreSQL, Worker execution or model acceptance.
test('life adapter retains aggregate and opaque CAS, failed-request idempotency and request scope',async()=>{
  const previous={window:globalThis.window,document:globalThis.document};
  globalThis.window={location:{origin:'http://life.test'}};
  globalThis.document={cookie:'fluctlight_csrf=fixture',addEventListener(){},createElement(){return {};}};
  const server=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
  const calls=[];let first=true,failWardrobe=false,failGoals=false;
  const json=(value,status=200)=>new Response(JSON.stringify(value),{status,headers:{'Content-Type':'application/json'}});
  const fetcher=async(url,init={})=>{
    const parsed=new URL(String(url)),path=parsed.pathname,body=init.body?JSON.parse(init.body):null;calls.push({path,query:Object.fromEntries(parsed.searchParams),body,method:init.method || 'GET'});
    if(path.endsWith('/goals'))return failGoals?json({code:'dependency_failed',message:'controlled goals failure'},503):json({items:parsed.searchParams.get('history')==='true'?[{id:'completed-one',status:'completed'}]:[{id:'active-one',status:'active'},{id:'active-two',status:'active'}],next_cursor:'opaque+cursor/='});
    if(path.endsWith('/wardrobe/wear')){if(first){first=false;throw new TypeError('controlled transport interruption');}return json({request:{id:'request-1',fluctlight_id:'fl-one',status:'pending',kind:'wardrobe_wear'}});}
    if(path.endsWith('/schedules/sc-one/items'))return json({request:{id:'schedule-request',fluctlight_id:'fl-one',status:'pending'}});
    if(path.endsWith('/wardrobe/items/item-one/use'))return json({request:{id:'use-request',fluctlight_id:'fl-one',status:'pending'}});
    if(path.includes('/owner-requests/'))return json({id:'foreign',fluctlight_id:'fl-two',status:'completed'});
    if(path.endsWith('/wardrobe/items/item-one/photos'))return json({asset_id:'media-one',status:'queued',factual:false,label:'生成图·非事实'});
    if(path.endsWith('/wardrobe'))return failWardrobe?json({code:'dependency_failed',message:'controlled DB failure'},503):json({revision:7,items:[],worn_items:[],used_items:[],has_more:false,next_cursor:'',inventory_complete:false});
    if(path.endsWith('/detail'))return json({context:{context_revision:'life_ctx_opaque',timezone:'Asia/Shanghai'},schedule:{id:'sc-one',revision:12,items:[]}});
    if(path.endsWith('/nearby'))return json({place_id:null,actors:[],location_known:false});
    throw new Error('Unexpected fixture route '+path);
  };
  try {
    const {BrowserClient}=await server.ssrLoadModule('/@fs/'+fileURLToPath(new URL('../../../packages/browser-client/src/index.ts',import.meta.url)));
    const {createLifeProvider}=await server.ssrLoadModule('/src/v3/providers/life.ts');
    const store=await server.ssrLoadModule('/src/v3/proto/store.js');
    store.replaceSnapshot({...store.get(),session:{actorId:'owner-one'},characters:{'fl-one':{id:'fl-one',status:'正常'},'fl-retired':{id:'fl-retired',status:'已退休'}}});
    const life=createLifeProvider(new BrowserClient('http://life.test',fetcher));
    await assert.rejects(life.wear('fl-one','item-one','upper_body',7,'换装'),/controlled transport/);
    const accepted=await life.wear('fl-one','item-one','upper_body',7,'换装');assert.equal(accepted.request.status,'pending');
    const wears=calls.filter(c=>c.path.endsWith('/wardrobe/wear'));assert.equal(wears[0].body.idempotencyKey,wears[1].body.idempotencyKey);assert.equal(wears[1].body.expectedRevision,7);assert.equal(wears[1].body.confirm,undefined);
    const goals=await life.goals('fl-one');assert.deepEqual(goals.items.map(g=>g.id),['active-one','active-two']);assert.equal(goals.next_cursor,'opaque+cursor/=');
    await life.goals('fl-one',goals.next_cursor);const goalCalls=calls.filter(c=>c.path.endsWith('/goals'));assert.equal(goalCalls[0].query.history,'false');assert.equal(goalCalls[0].query.limit,'50');assert.equal(goalCalls[1].query.cursor,goals.next_cursor);
    failGoals=true;await assert.rejects(life.goals('fl-one'),/controlled goals failure/);
    const detail=await life.detail('fl-one');
    await life.scheduleItem('fl-one',detail,null,{activity:'阅读',scene:'书屋',location:'同名地点',placeId:'place-one',startAt:'2026-10-17T10:00:00+08:00',endAt:'2026-10-17T11:00:00+08:00',reason:'建议'},'create');
    const schedule=calls.find(c=>c.path.endsWith('/schedules/sc-one/items'));assert.equal(schedule.body.expectedLifeContextRevision,'life_ctx_opaque');assert.equal(schedule.body.expectedRevision,12);assert.equal(schedule.body.placeId,'place-one');
    await life.use('fl-one','item-one',true,7,'使用');assert.deepEqual(calls.find(c=>c.path.endsWith('/use')).body.evidenceRefs,['owner:owner-one']);
    await assert.rejects(life.request('fl-one','foreign'),/范围不匹配/);
    const photo=await life.photo('fl-one','item-one','物品静物','stable-photo-key');assert.equal(photo.status,'queued');assert.equal(calls.find(c=>c.path.endsWith('/photos')).body.idempotencyKey,'stable-photo-key');
    await assert.rejects(life.wear('fl-retired','item-one','upper_body',7,'换装'),/不可写入/);
    failWardrobe=true;await assert.rejects(life.now('fl-one'),/controlled DB failure/);
  } finally {await server.close();Object.assign(globalThis,previous);}
});
