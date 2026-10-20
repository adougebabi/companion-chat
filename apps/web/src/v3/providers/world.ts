import { randomId } from '../../random-id.mjs';
import { BrowserClient, type RuntimeNPC, type RuntimeWorld, type RuntimePlace, type StudioResource, type ApplyPreviewResponse,type BrowserWorldItemConfigBody,type BrowserPlaceDefaultBody } from '@fluctlight/browser-client';
// @ts-expect-error Existing renderer service boundary.
import { configureDomainService } from '../proto/services.js';
// @ts-expect-error Existing read projection boundary.
import { configureReadProjection, get, subscribe } from '../proto/store.js';
const command = (revision: number, reason: string) => ({expectedRevision: revision, reason, idempotencyKey: randomId()});
const text = (value: unknown): string => typeof value === 'string' ? value : value == null ? '' : JSON.stringify(value);
const collection = (value:unknown):string|unknown[] => {if(Array.isArray(value))return value;const raw=text(value);try {const parsed:unknown=JSON.parse(raw);if(Array.isArray(parsed))return parsed;}catch { /* Ordinary display text is a supported wire value. */ }return raw;};
const modes = {unknown:'未知',routine:'按生活规律推算',fixed:'固定地点'};
const days = {daily:'每天',weekday:'工作日',weekend:'周末'};
export const worldView = (w: RuntimeWorld) => ({...w,version:w.revision,itemPool:text(w.item_pool)});
export const placeView = (p: RuntimePlace) => ({...p,worldId:p.world_id,version:p.revision,roomItems:text(p.room_items),furniture:text(p.furniture),connections:text(p.connections),parentPlaceId:p.parent_place_id,connectionPlaceIds:p.connection_place_ids || []});
export const npcView = (n: RuntimeNPC, charId: string) => ({...n,charId,worldId:n.world_id,version:n.revision,placeId:n.home_place_id || '',bio:n.bio,avatar:'',full:n.full_fluctlight,resourceId:n.source_resource_id,archived:n.status==='archived',positionMode:modes[n.position_mode],confirmedPlaceId:n.confirmed_place_id,confirmedAt:n.confirmed_at,routine:n.routine.map(s=>({...s,day:days[s.day],placeId:s.place_id})),references:[...(n.encounters || []).flatMap(e=>[e.life_event_id,e.relationship_id,e.memory_id]).filter(Boolean),...(n.runtime_reference_ids||[])]});
export const resourceView = (r: StudioResource) => { const value = (body?:Record<string,unknown>) => ({名称:r.name,简介:text(body?.role),背景:text(body?.background)}); return {...r,type:'NPC',version:r.revision,published:r.published_version,draftVersion:r.draft_version,draft:r.draft_version ? value(r.body) : null,versions:Object.fromEntries(r.versions.map(v=>[v.version,value(v.body)]))}; };
type NPCDraft = {name:string;role:string;bio:string;placeId:string;routine:Array<{day:string;start:string;end:string;placeId:string;activity:string}>;archived:boolean;enabled:boolean;positionMode:string;expectedVersion:number};
export function createWorldProvider({client,refreshProduct}: {client:BrowserClient;refreshProduct:()=>Promise<unknown>}) {
  let owner = get('session')?.actorId,ownerGeneration=0;
 const ownedRead=async<T>(read:()=>Promise<T>):Promise<T>=>{const actor=owner,generation=ownerGeneration;if(!actor)throw new Error('请先登录');const value=await read();if(generation!==ownerGeneration||get('session')?.actorId!==actor)throw new DOMException('Owner会话已切换，旧查询未写入共享状态','AbortError');return value;};
  const epochs = new Map<string,number>();
  const pendingKeys = new Map<string,string>();
  const mutation = async <T>(scope:string, payload:unknown, execute:(key:string)=>Promise<T>):Promise<T> => {const identity=scope+JSON.stringify(payload);let key=pendingKeys.get(identity);if(!key){key=randomId();pendingKeys.set(identity,key);}const result=await execute(key);pendingKeys.delete(identity);return result;};
  const catalogs = new Map<string, Awaited<ReturnType<BrowserClient['worldCatalog']>>>();
  const projections: Record<string,Record<string,unknown>> = {worlds:{},places:{},npcs:{}};
  const active = new Map<string,{worldId:string|null;placeId:string|null;locationAuthority:string}>();
  const previews = new Map<string, ApplyPreviewResponse & {resourceVersion:string}>();
  const applyKey = (fid:string,rid:string) => `${fid}:${rid}`;
  const remember = (fid:string, view:Awaited<ReturnType<BrowserClient['world']>>) => {
    if(view.world) { for(const table of ['places','npcs']) for(const [id,row] of Object.entries(projections[table])) if((row as {worldId?:string}).worldId===view.world.id) delete projections[table][id]; projections.worlds[view.world.id]=worldView(view.world); for(const p of view.places) projections.places[p.id]=placeView(p); for(const n of view.npcs) projections.npcs[n.id]=npcView(n,fid); }
  };
  const writable = (fid:string) => {const c=get('characters',fid); if(!c) throw new Error('当前角色不可用'); if(c.status==='已退休') throw new Error('已退休角色仅供读取');};
  const refresh = async (fid:string) => { const epoch=(epochs.get(fid)||0)+1; epochs.set(fid,epoch); const [catalog,view,nearby]=await ownedRead(()=>Promise.all([client.worldCatalog(fid),client.world(fid),client.nearby(fid)])); if(epochs.get(fid)!==epoch)return view; catalogs.set(fid,catalog); remember(fid,view); active.set(fid,{worldId:view.world?.id || null,placeId:nearby.location_known ? nearby.place_id : null,locationAuthority:nearby.location_known ? view.current_context?.authority_status === 'confirmed' ? '已确认' : '预计' : '未知'}); for(const n of Object.values(projections.npcs) as Array<Record<string,unknown>>) { if(n.charId!==fid)continue;const presence=nearby.actors.find(a=>a.npc_id===n.id);n.currentPlaceId=presence?.place_id || null;n.presence=presence?.presence || 'unknown';n.presenceSource=presence?.source || ''; } return view; };
  const world = {
    items:(fid:string,options:{cursor?:string;q?:string}={})=>client.worldItems(fid,options),
    item:(fid:string,id:string)=>client.worldItem(fid,id),
    configureItem:(fid:string,id:string,body:Omit<BrowserWorldItemConfigBody,'idempotencyKey'>)=>{writable(fid);return mutation([fid,'world-item-config',id].join(':'),body,idempotencyKey=>client.configureWorldItem(fid,id,{...body,idempotencyKey}));},
    refresh,
    async worlds(fid:string) {const c=await ownedRead(()=>client.worldCatalog(fid)); catalogs.set(fid,c); return c.items.map(w=>({...worldView(w),active:w.active}));},
    async world(fid:string,wid:string) {const v=await ownedRead(()=>client.catalogWorld(fid,wid));remember(fid,v);if(!v.world)throw new Error('当前范围未找到世界');return worldView(v.world);},
    async places(fid:string,wid:string) {const v=await ownedRead(()=>client.catalogWorld(fid,wid));remember(fid,v);return v.places.map(placeView);},
    async place(fid:string,wid:string,pid:string) {const rows=await world.places(fid,wid);const p=rows.find(p=>p.id===pid);if(!p)throw new Error('当前世界未找到地点');return p;},
    async create(fid:string,name:string) {writable(fid);return mutation('world:create:'+fid,{name},async key=>{const r=await client.createCatalogWorld(fid,{name,reason:'所有者创建世界',idempotencyKey:key});await world.world(fid,r.id);return r.id;});},
    async save(fid:string,wid:string,fields:{name:string;flavor:string;description:string;itemPool:string},version:number) {writable(fid);await client.updateCatalogWorld(fid,wid,{...fields,itemPool:collection(fields.itemPool),...command(version,'所有者编辑世界')});return world.world(fid,wid);},
    async select(fid:string,wid:string) {writable(fid);const c=catalogs.get(fid);if(!c)throw new Error('请先重新读取世界目录');await client.selectWorld(fid,{worldId:wid,...command(c.active_binding?.revision || 0,'所有者显式切换世界')});await refreshProduct();await refresh(fid);},
    async removeWorld(fid:string,wid:string,revision:number) {writable(fid);await client.deleteCatalogWorld(fid,wid,command(revision,'所有者移除无引用世界'));delete projections.worlds[wid];},
    async savePlace(fid:string,wid:string,pid:string|null,fields:Record<string,unknown>,version:number) {writable(fid);const body={...command(version,'所有者保存地点'),worldId:wid,name:String(fields.name),description:text(fields.desc),alias:text(fields.alias),group:text(fields.group),floor:text(fields.floor),scene:text(fields.scene),atmosphere:text(fields.atmosphere),furniture:collection(fields.furniture),layout:text(fields.layout),connections:collection(fields.connections),roomItems:collection(fields.roomItems),parentPlaceId:fields.parentPlaceId as string|null|undefined,connectionPlaceIds:fields.connectionPlaceIds as string[]|undefined};const r=pid ? await client.updateWorldPlace(fid,pid,body):await client.createWorldPlace(fid,body);await world.place(fid,wid,r.id);await refreshProduct();return r.id;},
    async removePlace(fid:string,wid:string,pid:string,revision:number) {writable(fid);await client.deleteWorldPlace(fid,pid,{worldId:wid,...command(revision,'所有者移除无引用地点')});await refreshProduct();},
    async templates(fid:string) {return client.worldTemplates(fid);},
    async setTemplate(fid:string,id:string,enabled:boolean,revision:number) {writable(fid);await client.setWorldTemplate(fid,id,{enabled,...command(revision,'所有者调整世界事件模板')});return client.worldTemplates(fid);},
    events:(fid:string,cursor='')=>client.worldLifeEvents(fid,cursor),
    event:(fid:string,id:string)=>client.worldLifeEvent(fid,id),
    template:(fid:string,id:string)=>client.worldEventTemplate(fid,id),
    eventRuns:(fid:string,id:string,cursor='')=>client.worldEventRuns(fid,id,cursor),
    eventRun:(fid:string,id:string,run:string)=>client.worldEventRun(fid,id,run),
    eventCommand:(fid:string,id:string,operation:'toggle'|'trigger'|'retry',body:{expectedRevision:number;reason:string;idempotencyKey:string;enabled?:boolean;scheduledAt?:string;expectedRunRevision?:number},run?:string)=>{
      writable(fid);
      if(operation==='toggle')return client.setWorldTemplate(fid,id,{...body,enabled:body.enabled!});
      if(operation==='retry'){if(!run||body.expectedRunRevision==null)throw new Error('缺少原执行版本');return client.retryWorldEvent(fid,id,run,{...body,expectedRunRevision:body.expectedRunRevision});}
      return client.triggerWorldEvent(fid,id,body);
    },
    placeDefaults:(fid:string,wid:string,pid:string,source='recorded')=>client.placeDefaults(fid,wid,pid,source),
    placeDefaultHistory:(fid:string,wid:string,pid:string,cursor='')=>client.placeDefaultHistory(fid,wid,pid,cursor),
    restorePlace:(fid:string,wid:string,pid:string,body:Omit<BrowserPlaceDefaultBody,'worldId'>)=>{writable(fid);return client.savePlaceDefaults(fid,pid,{...body,worldId:wid});},
  };
  const npc = {
    npcRecords:(fid:string,id:string,kind:'encounters'|'presences'|'opportunities',cursor='')=>client.npcRecords(fid,id,kind,cursor),
    npcRecord:(fid:string,id:string,opportunity:string)=>client.npcOpportunity(fid,id,opportunity),
    retryNPCOpportunity:(fid:string,id:string,opportunity:string,body:{expectedRevision:number;reason:string})=>{writable(fid);return mutation([fid,id,opportunity,'retry'].join(':'),body,idempotencyKey=>client.retryNPCOpportunity(fid,id,opportunity,{...body,idempotencyKey}));},
    async npcs(fid:string,options:{query?:string;archived?:boolean}={}) {await refresh(fid);const page=await ownedRead(()=>client.listRuntimeNPCs(fid));if(page.next_cursor)throw new Error('人物目录分页尚未完整读取，请缩小查询范围');return page.items.map(n=>npcView(n,fid)).filter(n=>(options.archived || !n.archived)&&(!options.query || `${n.name} ${n.role}`.includes(options.query)));},
    async npc(fid:string,id:string) {const n=npcView(await ownedRead(()=>client.runtimeNPC(fid,id)),fid);projections.npcs[id]=n;return n;},
    async resource(rid:string) {const r=await client.studioResource(rid);if(r.kind!=='npc')throw new Error('该资源不是 NPC 定义');return resourceView(r);},
    async createNPCDraft(fid:string,draft:{name:string;role:string;bio:string;body?:Record<string,unknown>;idempotencyKey?:string}) {writable(fid);const r=await client.createStudioResource({kind:'npc',name:draft.name,body:{...draft.body,role:draft.role,background:draft.bio},reason:'所有者创作 NPC 草稿',idempotencyKey:draft.idempotencyKey||randomId()});return resourceView(r);},
    async saveNPCDraft(rid:string,draft:{name:string;role:string;bio:string;body?:Record<string,unknown>;draftVersion?:string;idempotencyKey?:string}) {return resourceView(await client.updateStudioResource(rid,{kind:'npc',name:draft.name,body:{...draft.body,role:draft.role,background:draft.bio},expectedVersion:draft.draftVersion,reason:'所有者保存 NPC 草稿',idempotencyKey:draft.idempotencyKey||randomId()}));},
    async publishNPCDraft(rid:string,revision:number,idempotencyKey=randomId()) {await client.publishStudioResource(rid,{expectedRevision:revision,reason:'所有者发布 NPC 定义',idempotencyKey});return npc.resource(rid);},
    async preview(fid:string,rid:string) {const r=await client.studioResource(rid);if(!r.published_version)throw new Error('请先发布定义');const p=await client.previewStudioApply(rid,{fluctlightId:fid,resourceVersion:r.published_version});previews.set(applyKey(fid,rid),{...p,resourceVersion:r.published_version});return {resource:resourceView(r),preview:p};},
    async applyNPC(fid:string,rid:string,placeId:string,choices:Record<string,string>={}) {writable(fid);const r=await client.studioResource(rid);const p=previews.get(applyKey(fid,rid));if(!r.published_version||!p)throw new Error('请先读取本次应用的差异预览');const result=await client.applyStudioResource(rid,{fluctlightId:fid,resourceVersion:p.resourceVersion,expectedInstanceRevision:p.instance_revision,expectedRuntimeRevision:p.runtime_revision,homePlaceId:placeId || null,choices:Object.entries(choices).map(([field,decision])=>({field,decision:decision==='replace'?'override':'keep'})),reason:'所有者明确应用 NPC 定义',idempotencyKey:randomId()});if(!result.applied||!result.id)throw new Error('定义尚未应用，请处理版本冲突后重试');previews.delete(applyKey(fid,rid));await refreshProduct();return npc.npc(fid,result.id);},
    async saveNPC(fid:string,id:string,draft:NPCDraft) {writable(fid);const positionMode=Object.entries(modes).find(([,v])=>v===draft.positionMode)?.[0] as RuntimeNPC['position_mode']|undefined;if(!positionMode)throw new Error('请选择有效的位置模式');await client.updateRuntimeNPC(fid,id,{...command(draft.expectedVersion,'所有者修正运行人物'),name:draft.name,role:draft.role,background:draft.bio,homePlaceId:draft.placeId||null,routine:draft.routine.map(s=>{const day=Object.entries(days).find(([,v])=>v===s.day)?.[0] as 'daily'|'weekday'|'weekend'|undefined;if(!day)throw new Error('生活规律的适用日无效');return {day,start:s.start,end:s.end,place_id:s.placeId,activity:s.activity};}),status:draft.archived?'archived':'active',enabled:draft.enabled,positionMode});return npc.npc(fid,id);},
    async removeNPC(fid:string,id:string,revision:number) {writable(fid);await client.deleteRuntimeNPC(fid,id,command(revision,'所有者移除无引用人物'));await refreshProduct();},
    async recordEncounter(fid:string,id:string,body:{placeId:string;summary:string;startAt:string;endAt:string;evidenceRefs:string[];expectedLifeContextRevision:string;idempotencyKey:string}) {writable(fid);const r=await client.recordNPCEncounter(fid,id,{...body,confirmed:true,reason:'所有者确认记录实际发生的接触'});await refreshProduct();return {...r,eventId:r.event_id};},
    async readRelation(fid:string,actorId:string,id:string) {const context=await client.actorContext(fid,actorId);const relation=context.relationships.find(r=>r.id===id);if(!relation)throw new Error('当前 Actor 范围未找到关系');return {relation,history:context.relationship_history.filter(r=>r.relationship_id===id)};},
    async restoreRelation(fid:string,actorId:string,body:{targetRevision:number;expectedRevision:number;profileId?:string;evidenceRefs:string[]}) {writable(fid);await client.rollbackRelationship(fid,{targetActorId:actorId,...body});await refreshProduct();},
    async saveRelation(fid:string,id:string,body:{profileId?:string;expectedRevision:number;summary:string;role:Record<string,unknown>;metrics:Record<string,unknown>;evidenceRefs:string[];reason:string}) {writable(fid);await client.editRelationship(fid,id,body);await refreshProduct();},
    async encounterContext(fid:string) {const detail=await client.detail(fid);const c=detail.context as {context_revision?:string}|undefined;if(!c?.context_revision)throw new Error('生活上下文版本缺失，请刷新后重试');return c.context_revision;},
  };
  return {world,npc,install() {subscribe(() => {const next=get('session')?.actorId;if(next!==owner){ownerGeneration++;owner=next;catalogs.clear();active.clear();previews.clear();epochs.clear();pendingKeys.clear();for(const table of Object.keys(projections))projections[table]={};}});configureDomainService('world',world);configureDomainService('npc',npc);for(const table of ['worlds','places','npcs'])configureReadProjection(table,(base:Record<string,unknown>)=>({...base,...projections[table]}));configureReadProjection('characters',(base:Record<string,Record<string,unknown>>)=>Object.fromEntries(Object.entries(base).map(([id,c])=>[id,active.has(id)?{...c,worldId:active.get(id)!.worldId,locationId:active.get(id)!.placeId,locationAuthority:active.get(id)!.locationAuthority}:c])));}};
}
export function installWorldProvider(options:Parameters<typeof createWorldProvider>[0]) {const provider=createWorldProvider(options);provider.install();return provider;}
