import {check,equal,hash,RegressionError} from './runtime.mjs';
export const fidPath=id=>`/api/fluctlights/${encodeURIComponent(id)}`;
export const resourcePath=id=>`/api/studio/resources/${encodeURIComponent(id)}`;
export const reason='隔离的v3正式回归验证';
export const goArgs=(names,pkg='./internal/core')=>['go','-C','apps/core-go','test','-json',pkg,'-run',`^(${names.join('|')})$`,'-count=1'];
export const nodeArgs=files=>['node','--test',...files.map(file=>`apps/web/test/${file}`)];
export async function fixture(t,key='primary',{description,initialization}={}){
 const existing=t.object(`fixture:${key}`);
 if(existing?.id){const detail=await t.get(fidPath(existing.id)+'/detail');equal(detail.id||existing.id,existing.id,'Fixture must retain its actual character identity');return {...existing,detail};}
 t.model('Create an isolated character (initialization/portrait and lifecycle tasks may call models)');
 const name=`V3回归-${t.state.runId.slice(0,8)}-${key}`;let analysis;
 if(description||initialization){
  const result=await t.command(`fixture-${key}-analysis`,'POST','/api/fluctlight-creations/analysis',()=>initialization?{json:initialization}:{description:description.replaceAll('$NAME',name)},{timeoutMs:t.options.timeoutMs,replay:'read-result'});analysis=result.value;
  check(typeof analysis?.analysis_id==='string'&&analysis.core_persona&&Array.isArray(analysis.initial_goals),'Initialization must return an actual persisted analysis source');
 }
 const activation=await t.command(`fixture-${key}-activate`,'POST','/api/fluctlight-creations/activate',()=>analysis?{requestId:t.token(`activate:${key}`),initializationMode:'llm_defined',initializationTimezone:'Asia/Shanghai',analysisId:analysis.analysis_id,schemaVersion:analysis.schema_version,name,corePersona:analysis.core_persona,developingSelf:analysis.developing_self,extensions:analysis.extensions,initialGoals:analysis.initial_goals,initialIntentions:analysis.initial_intentions,initialRelationships:analysis.initial_relationships}:{requestId:t.token(`activate:${key}`),initializationMode:'blank_slate',name,initializationTimezone:'Asia/Shanghai'},{timeoutMs:t.options.timeoutMs});
 check(typeof activation.value?.id==='string','Activation must return a persisted character ID');
 const value={id:activation.value.id,name,analysisId:analysis?.analysis_id,activationCommand:`${t.id}:fixture-${key}-activate`};await t.remember(`fixture:${key}`,value);
 return {...value,detail:await t.get(fidPath(value.id)+'/detail')};
}
export async function conversation(t,fid,key='primary'){
 const saved=t.object(`conversation:${key}`);if(saved?.id)return saved;
 const result=await t.command(`conversation-${key}`,'POST',fidPath(fid)+'/conversations',()=>({idempotencyKey:t.token(`conversation:${key}`),title:`V3回归 ${key}`}));
 const value=result.value?.conversation||result.value;check(typeof value?.id==='string','New conversation must have a real ID');return t.remember(`conversation:${key}`,{id:value.id,fid});
}
export async function readStream(response,{onEvent=()=>{}}={}){
 check(/application\/x-ndjson/i.test(response.headers.get('content-type')||''),'Turn must use the real NDJSON protocol');
 check(response.body,'Turn response must have a stream');const reader=response.body.getReader(),decoder=new TextDecoder(),events=[];let buffer='',previous=-1,turn;
 async function accept(line){if(!line.trim())return;const event=JSON.parse(line);check(Number.isSafeInteger(event.sequence)&&event.sequence>previous,'Stream sequences must strictly increase',`>${previous}`,event.sequence);previous=event.sequence;
  check(typeof event.turnId==='string'&&event.turnId,'Every public frame needs its actual turn identity');if(turn)equal(event.turnId,turn,'Stream cannot mix turn identities');turn=event.turnId;
  check(['token','message','media','completed','error','heartbeat'].includes(event.type),'Only public stream event types are accepted',undefined,event.type);events.push(event);await onEvent(event);
 }
 try{for(;;){const {value,done}=await reader.read();buffer+=decoder.decode(value||new Uint8Array(),{stream:!done});const lines=buffer.split('\n');buffer=lines.pop();for(const line of lines)await accept(line);if(done)break;}if(buffer.trim())await accept(buffer);}
 catch(error){if(error instanceof RegressionError)throw error;throw new RegressionError('STREAM_INTERRUPTED','NDJSON observation ended without an authoritative terminal result',{cause:error.message,turnId:turn,lastSequence:previous,receivedFrames:events.map(e=>({type:e.type,sequence:e.sequence,turnId:e.turnId}))});}
 finally{reader.releaseLock();}
 const terminals=events.filter(e=>['completed','error'].includes(e.type));equal(terminals.length,1,'A complete observer stream must contain exactly one terminal event');equal(events.at(-1),terminals[0],'No public frames may follow the terminal event');
 return {events,terminal:terminals[0]};
}
export async function chat(t,fid,cid,key,text,{signal,onEvent,allowError=false}={}){
 t.model(`Conversation ${key}`);
 const bodyKey=`${t.id}:chat:${key}`;let original=t.state.commands[bodyKey];
 if(!original){original={method:'POST',path:`/api/conversations/${encodeURIComponent(cid)}/turn`,body:{text,fluctlightId:fid,turnId:t.token(`turn:${key}`),idempotencyKey:t.token(`chat:${key}`),senderTimezone:'Asia/Shanghai',senderUtcOffsetMinutes:480,senderSentAt:new Date().toISOString()}};t.state.commands[bodyKey]=original;await t.save();}
 const response=await t.request(original.path,{method:'POST',body:original.body,stream:true,timeoutMs:t.options.timeoutMs,signal});const parsed=await readStream(response,{onEvent});
 if(parsed.terminal.type==='error'&&!allowError)throw new RegressionError(parsed.terminal.payload?.code||'TURN_FAILED','Conversation ended with a real error',{turnId:original.body.turnId,terminal:parsed.terminal});
 const messages=await t.get(`/api/conversations/${encodeURIComponent(cid)}/messages?limit=100`);check(Array.isArray(messages?.messages),'Persisted conversation must contain its message ledger');
 return {turnId:original.body.turnId,idempotencyKey:original.body.idempotencyKey,events:parsed.events,terminal:parsed.terminal,messages:messages.messages,original};
}
export async function ownerRequest(t,id){return t.until(`Owner request ${id}`,()=>t.get(`/api/owner-requests/${encodeURIComponent(id)}`),r=>!['pending','processing'].includes(r.status));}
export async function createResource(t,key,kind,body){
 const saved=t.object(`resource:${key}`);if(saved?.id)return t.get(resourcePath(saved.id));
 const result=await t.command(`resource-${key}`,'POST','/api/studio/resources',()=>({kind,name:`V3回归-${t.state.runId.slice(0,8)}-${key}`,body,reason,idempotencyKey:t.token(`resource:${key}`)}));
 check(result.value?.id&&result.value.revision>0,'Resource creation must return a durable definition');await t.remember(`resource:${key}`,{id:result.value.id});return result.value;
}
export async function publish(t,key,resource){const result=await t.command(`publish-${key}`,'POST',resourcePath(resource.id)+'/publish',()=>({expectedRevision:resource.revision,reason,idempotencyKey:t.token(`publish:${key}`)}));equal(result.value.status,'published','Publication must actually succeed');return t.get(resourcePath(resource.id));}
export async function ensureWorld(t,fid,key='world'){
 const existing=t.object(`world:${fid}:${key}`);
 const world=existing||(await t.command(`world-${key}`,'POST',fidPath(fid)+'/world/catalog',()=>({name:`V3回归-${key}`,background:'仅用于隔离回归的虚构世界',reason,idempotencyKey:t.token(`world:${key}`)}))).value;
 const catalog=await t.get(fidPath(fid)+'/world/catalog');
 if(catalog.active_binding?.world_id!==world.id)await t.command(`select-world-${key}-${catalog.active_binding?.revision||0}`,'PUT',fidPath(fid)+'/world/active',()=>({worldId:world.id,expectedRevision:catalog.active_binding?.revision||0,reason,idempotencyKey:t.token(`select-world:${key}:${catalog.active_binding?.revision||0}`)}));
 equal((await t.get(fidPath(fid)+'/world')).world?.id,world.id,'World selection must read back the same stable ID');return t.remember(`world:${fid}:${key}`,world);
}
export async function apply(t,key,fid,resource,extra={}){
 const preview=(await t.request(resourcePath(resource.id)+'/apply-preview',{method:'POST',body:{fluctlightId:fid,resourceVersion:resource.published_version}})).value;
 check(preview.available!==false&&Array.isArray(preview.diff),'An actual domain apply preview is required',true,preview);
 const result=(await t.command(`apply-${key}`,'POST',resourcePath(resource.id)+'/apply',()=>({fluctlightId:fid,resourceVersion:resource.published_version,expectedInstanceRevision:preview.instance_revision,expectedRuntimeRevision:preview.runtime_revision||0,...(preview.target_kind==='wardrobe_outfit'?{profileId:preview.profile_id,expectedWardrobeRevision:preview.wardrobe_revision}:{}),...(preview.world_id?{worldId:preview.world_id,expectedWorldRevision:preview.world_binding_revision}:{}),choices:preview.diff.map(d=>({field:d.field,decision:'keep'})),reason,idempotencyKey:t.token(`apply:${key}`),...extra}))).value;
 if(result.applied!==true)t.blocked('RESOURCE_NOT_APPLIED','A recorded choice is not a real domain application',{resourceId:resource.id,kind:resource.kind,result});
 return result;
}
export async function job(t,key,resource,kind,instruction,extra={}){
 t.model(`Studio ${kind}`);
 const created=(await t.command(`studio-${key}`,'POST','/api/studio/jobs',()=>({resourceId:resource.id,resourceRevision:resource.revision,kind,name:resource.name,body:resource.body,instruction,reason,idempotencyKey:t.token(`studio:${key}`),...extra}))).value;
 check(created.id&&created.source_digest,'Studio must persist a frozen job with source digest');
 const done=await t.until(`Studio job ${created.id}`,()=>t.get(`/api/studio/jobs/${encodeURIComponent(created.id)}`),j=>j.status==='completed');
 equal(done.source_digest,created.source_digest,'Studio source identity must stay frozen');check(Array.isArray(done.attempts)&&done.attempts.length>0,'Studio completion must retain actual attempt evidence');return done;
}
export async function lifeSnapshot(t,fid){
 const p=fidPath(fid),[detail,inventory,diary,media]=await Promise.all([t.get(p+'/detail'),t.pages(p+'/inventory?limit=100'),t.pages(p+'/diary?limit=100'),t.pages(p+'/media?limit=100')]);
 check(detail.current_state?.appearance&&Array.isArray(detail.memories)&&Array.isArray(detail.relationships),'Isolation checks require real appearance, memory and relationship snapshots');
 const {captured_at,...effectiveAppearance}=detail.current_state.appearance;
 return {fid,identity:detail.identity,context:detail.context,appearance:effectiveAppearance,memories:detail.memories.map(m=>({id:m.id,revision:m.revision})),relationships:detail.relationships.map(r=>({id:r.id,revision:r.revision})),inventory:inventory.map(i=>({id:i.id,revision:i.revision})),diary:diary.map(d=>({id:d.id,revision:d.revision})),media:media.map(m=>({id:m.id||m.asset_id}))};
}
export function sameInventoryAndAppearance(before,after){equal(after.inventory,before.inventory,'Operation must not grant or duplicate inventory');equal(after.appearance,before.appearance,'Operation must preserve current effective appearance');}
export async function actualModelRuns(t,turnId){
 const path=`/api/diagnostics/model-runs?correlationId=${encodeURIComponent('turn:'+turnId)}&limit=100`;
 const roots=await t.pages(path,{cursor:'nextCursor'}),all=await t.pages('/api/diagnostics/model-runs?limit=500',{cursor:'nextCursor'});
 return modelRunFamily(roots,all,'turn:'+turnId);
}
export function modelRunFamily(roots,all,correlation){
 const family=new Map(roots.map(r=>[r.id,r])),parents=new Set([correlation,...roots.map(r=>r.logicalRunId||r.correlationId)]);
 let changed=true;while(changed){changed=false;for(const row of all)if(row.parentRunId&&parents.has(row.parentRunId)&&!family.has(row.id)){family.set(row.id,row);parents.add(row.logicalRunId||row.correlationId);changed=true;}}
 return [...family.values()];
}
export function evidenceDigest(value){return {sha256:hash(value)};}
export function visualGenerationComplete(value,sessionId){return value.session?.id===sessionId&&['completed','awaiting_review'].includes(value.session.status);}
export function visualGenerationAsset(value,sessionId){
 if(!visualGenerationComplete(value,sessionId))return null;
 if(value.session.status==='completed')return value.canonical_asset_id||null;
 return value.session.attempts?.find(a=>a.attempt_number===value.session.current_attempt)?.candidate_asset_id||null;
}

export async function readImageEvidence(t,id){
 const response=await t.request(`/api/media/${encodeURIComponent(id)}`,{stream:true}),mime=response.headers.get('content-type')||'',bytes=new Uint8Array(await response.arrayBuffer());
 check(/^image\//.test(mime)&&bytes.length>64,'An actual authorized image must contain nonempty image bytes');
 check(bytes[0]===0x89&&bytes[1]===0x50||bytes[0]===0xff&&bytes[1]===0xd8||String.fromCharCode(...bytes.slice(0,4))==='RIFF','Image bytes must have a supported image signature');
 return {assetId:id,mime,bytes:bytes.length,sha256:hash(bytes)};
}
