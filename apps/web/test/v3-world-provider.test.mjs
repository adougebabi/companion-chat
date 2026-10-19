import test from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'vite';
import { fileURLToPath } from 'node:url';

// Exercises the real BrowserClient and adapter together; the transport is an
// explicit fixture. This does not claim PostgreSQL or Worker acceptance.
test('world adapter preserves identities, selection CAS, NPC publication and explicit application',async()=>{
  const original={window:globalThis.window,document:globalThis.document,localStorage:globalThis.localStorage,fetch:globalThis.fetch};
  globalThis.window={location:{origin:'http://world.test'}};globalThis.document={cookie:'fluctlight_csrf=fixture-csrf',addEventListener(){},createElement(){return {};}};globalThis.localStorage={getItem:()=>null,setItem(){},removeItem(){}};
  const server=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
  const calls=[];let active='world-active',revision=4;
  const w=id=>({id,fluctlight_id:'fl-real',name:id,background:'',rules:'',flavor:'真实基调',description:'世界描述',item_pool:['书'],revision:3,source_resource_id:null,source_version:null});
  const place={id:'place-real',world_id:active,name:'真实地点',desc:'描述',status:'active',revision:8,parent_place_id:null,source_resource_id:null,source_version:null,room_items:['桌子'],furniture:'椅子',connections:'走廊',connection_place_ids:['place-link'],scene:''};
  const n={id:'npc-real',world_id:active,actor_id:'actor-npc',name:'真人物',role:'身份',bio:'背景',home_place_id:null,routine:[{day:'weekday',start:'09:00',end:'17:00',place_id:place.id,activity:'工作'}],status:'active',revision:7,source_resource_id:'resource-real',source_version:'v1',confirmed_place_id:null,confirmed_at:null,full_fluctlight:false,enabled:true,position_mode:'unknown',local_overrides:{role:'局部修正'}};
  let resource={id:'resource-real',kind:'npc',revision:2,name:'人物定义',published_version:'v1',draft_version:'v2',body:{role:'新身份',background:'新背景'},versions:[{version:'v1',body:{role:'旧身份',background:'旧背景'}}],referenced_by:[]};
  const json=(v,status=200)=>new Response(JSON.stringify(v),{status,headers:{'Content-Type':'application/json'}});
  globalThis.fetch=async(url,init={})=>{const path=new URL(String(url),'http://world.test').pathname,body=init.body?JSON.parse(init.body):undefined;calls.push({path,method:init.method||'GET',body});
    if(path==='/api/session')return json({authenticated:true,actorId:'owner-real',csrfToken:'fixture-csrf'});
    if(path.endsWith('/world/catalog')&&init.method==='POST')return json({id:'world-new',revision:1,status:'created',audit_id:'audit',active:false});
    if(path.endsWith('/world/catalog'))return json({items:[{...w(active),active:true},{...w('world-new'),active:false}],active_binding:{world_id:active,revision},next_cursor:null});
    if(path.endsWith('/world/active')){assert.equal(body.expectedRevision,revision);active=body.worldId;revision++;return json({id:active,revision,status:'selected',audit_id:'audit'});}
    if(path.includes('/world/catalog/')&&init.method==='PUT')return json({id:'world-new',revision:4,status:'saved',audit_id:'audit'});
    if(path.includes('/world/catalog/')&&init.method==='DELETE')return json({code:'world_referenced',message:'地点被引用'},409);
    if(path.includes('/world/catalog/')||path.endsWith('/world'))return json({available:true,world:w(path.includes('/catalog/')?path.split('/').at(-1):active),places:[place],npcs:[n],scenes:[]});
    if(path.endsWith('/world/places/place-real/defaults')&&init.method==='POST')return json({id:place.id,revision:9,default_revision:1,status:'restored',audit_id:'audit-default'});
    if(path.endsWith('/world/places/place-real/defaults'))return json({place_id:place.id,world_id:active,place_revision:8,default_revision:1,baseline:{name:'original'},diff:[{field:'name',from:place.name,to:'original'}],source:'recorded',current:place,semantics:'restore definition fields only'});
    if(path.endsWith('/nearby'))return json({place_id:null,actors:[],available:true,location_known:false});
    if(path.endsWith('/world/places/place-real')&&init.method==='PUT')return json({id:place.id,revision:9,status:'saved',audit_id:'audit'});
    if(path.endsWith('/npcs/npc-real')&&init.method==='PUT'){assert.equal(body.positionMode,'routine');assert.equal(body.routine[0].day,'weekday');n.revision++;return json({id:n.id,revision:n.revision,status:'saved',audit_id:'audit'});}
    if(path.endsWith('/npcs/npc-real'))return json(n);
    if(path.endsWith('/apply-preview'))return json({diff:[{field:'role',from:'局部修正',to:'新身份'}],conflicts:[],instance_revision:11,runtime_revision:7,runtime_id:n.id});
    if(path.endsWith('/apply'))return json({applied:true,status:'applied',id:n.id,instance_revision:12,conflicts:[]});
    if(path.endsWith('/publish')){resource={...resource,published_version:'v2',revision:3};return json({id:resource.id,revision:3,published_version:'v2',status:'published'});}
    if(path.endsWith('/studio/resources/resource-real'))return json(resource);
    throw new Error('Unexpected fixture route '+path);
  };
  try{
    const {BrowserClient}=await server.ssrLoadModule('/@fs/'+fileURLToPath(new URL('../../../packages/browser-client/src/index.ts',import.meta.url)));
    const {createWorldProvider}=await server.ssrLoadModule('/src/v3/providers/world.ts');
    const store=await server.ssrLoadModule('/src/v3/proto/store.js');store.replaceSnapshot({...store.get(),session:{actorId:'owner-real'},characters:{'fl-real':{id:'fl-real',name:'角色',status:'正常'}}});
    const p=createWorldProvider({client:new BrowserClient('http://world.test'),refreshProduct:async()=>{}});p.install();
    const worlds=await p.world.worlds('fl-real');assert.equal(worlds[0].version,3);assert.equal(worlds[0].itemPool,'["书"]');
    const before=calls.filter(c=>c.method!=='GET').length;await p.world.refresh('fl-real');await p.world.world('fl-real','world-new');assert.equal(calls.filter(c=>c.method!=='GET').length,before,'browsing issues no write');assert.equal(store.get('characters','fl-real').locationId,null);
    assert.equal(await p.world.create('fl-real','新世界'),'world-new');assert.equal(active,'world-active','creation must not select');
    await p.world.save('fl-real','world-new',{name:'新世界',flavor:'风味',description:'描述',itemPool:'纸'},3);
    const save=calls.find(c=>c.method==='PUT'&&c.path.includes('/catalog/'));assert.equal(save.body.expectedRevision,3);assert.equal(save.body.flavor,'风味');
    await p.world.select('fl-real','world-new');assert.equal(active,'world-new');await p.world.select('fl-real','world-active');assert.equal(active,'world-active');
    await assert.rejects(p.world.removeWorld('fl-real','world-new',3),/地点被引用/);
    const baseline=await p.world.placeDefaults('fl-real','world-active',place.id);assert.equal(baseline.default_revision,1);
    const restored=await p.world.restorePlace('fl-real','world-active',place.id,{expectedRevision:8,expectedDefaultRevision:1,operation:'restore',source:'recorded',reason:'restore saved fields',idempotencyKey:'restore-place-1'});assert.equal(restored.status,'restored');
    const restore=calls.find(c=>c.method==='POST'&&c.path.endsWith('/defaults'));assert.deepEqual(restore.body,{worldId:'world-active',expectedRevision:8,expectedDefaultRevision:1,operation:'restore',source:'recorded',reason:'restore saved fields',idempotencyKey:'restore-place-1'});assert.equal(store.get('characters','fl-real').locationId,null,'restoring fields never moves a character');
    const npc=await p.npc.npc('fl-real',n.id);assert.equal(npc.actor_id,'actor-npc');assert.equal(npc.positionMode,'未知');assert.equal(npc.routine[0].day,'工作日');assert.equal(npc.confirmedPlaceId,null);
    await p.npc.saveNPC('fl-real',n.id,{...npc,positionMode:'按生活规律推算',expectedVersion:7});
    const runtime=calls.find(c=>c.method==='PUT'&&c.path.endsWith('/npcs/npc-real'));assert.equal(runtime.body.expectedRevision,7);assert.equal(runtime.body.homePlaceId,null);
    await p.npc.preview('fl-real',resource.id);resource.published_version='v99';await p.npc.applyNPC('fl-real',resource.id,'',{role:'keep'});
    const apply=calls.find(c=>c.path.endsWith('/apply'));assert.equal(apply.body.resourceVersion,'v1','application pins previewed publication');assert.equal(apply.body.expectedRuntimeRevision,7);assert.deepEqual(apply.body.choices,[{field:'role',decision:'keep'}]);
    await p.npc.publishNPCDraft(resource.id,2);assert.equal(calls.find(c=>c.path.endsWith('/publish')).body.expectedRevision,2);
    const writes=calls.filter(c=>c.method!=='GET'&&!c.path.endsWith('/apply-preview'));assert(writes.every(c=>c.body.idempotencyKey));
    store.replaceSnapshot({...store.get(),session:null,characters:{}});assert.deepEqual(store.list('npcs'),[],'logout clears cached owner projections');
  } finally {await server.close();Object.assign(globalThis,original);}
});
