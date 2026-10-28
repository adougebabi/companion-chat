import {pathToFileURL} from 'node:url';
const {runScenario,check,equal}=await import(pathToFileURL(process.cwd()+'/scripts/e2e/v3-regression/runtime.mjs'));
const {fixture,ensureWorld,fidPath,conversation,chat,reason}=await import(pathToFileURL(process.cwd()+'/scripts/e2e/v3-regression/domain.mjs'));
await runScenario('UI-RUNTIME','全站审计：真实模型配置与角色运行',async t=>{
 const endpoint='demo-audit-local',model='huihui-ai-Huihui-Qwen3.8-27B-abliterated-MTPLX';
 const configure=await t.step('endpoint','通过正式配置接口保存已有本地模型服务并读回',async()=>{
  const c=await t.get('/api/configuration-catalog');await t.command('endpoint','PUT','/api/providers/endpoints',()=>({endpointId:endpoint,kind:'openai-compatible',baseUrl:'http://host.docker.internal:11234/v1',secretPurpose:'provider:'+endpoint,expectedVersions:{['endpoint:'+endpoint]:c.versions['endpoint:'+endpoint]||0},idempotencyKey:t.token('endpoint')}),{expected:[200,204]});
  const saved=await t.get('/api/providers/endpoints');check(saved.some(e=>e.id===endpoint&&e.base_url==='http://host.docker.internal:11234/v1'),'实际服务连接读回');return {endpoint};
 });if(!configure)return;
 for(const role of ['generic_llm','conversation_reply','vision_understanding','embedding'])await t.step('binding-'+role,'正式用途绑定、实际能力检查与回读：'+role,async()=>{
  const c=await t.get('/api/configuration-catalog'),keys=['endpoint:'+endpoint,'secret:provider:'+endpoint,'binding:'+role];t.model('Actual provider binding preflight: '+role);
  const result=await t.command('binding-'+role,'PUT','/api/providers/roles',()=>({role,endpointId:endpoint,modelId:role==='embedding'?'mlx-community/Qwen3-Embedding-0.6B-4bit-DWQ':model,timeoutSeconds:600,expectedVersions:Object.fromEntries(keys.map(k=>[k,c.versions[k]||0])),idempotencyKey:t.token('binding-'+role)}),{timeoutMs:t.options.timeoutMs,expected:[200,204]});
  const latest=await t.get('/api/providers');check(latest.some(r=>r.role===role&&r.endpoint_id===endpoint),'用途绑定持久化');return {role,status:result.status};
 });
 const actor=await t.step('blank-activation','真实空白激活只创建原请求对应角色，重新读取持久身份',()=>fixture(t,'fullsite-audit'));if(!actor)return;
 const world=await t.step('world-binding','显式创建并选用真实生活世界，读回相同世界ID',()=>ensureWorld(t,actor.id,'fullsite-world'));if(!world)return;
 await t.step('read-scope','真实目录、角色、世界、衣柜、日程、治理与诊断读取',async()=>{
  const result={fid:actor.id,worldId:world.id,reads:[]};for(const suffix of ['/detail','/world','/inventory','/schedules','/goals','/diary','/memories','/autonomy-policy','/visual-identity','/display-avatar']){const value=await t.get(fidPath(actor.id)+suffix);result.reads.push({path:suffix,shape:Array.isArray(value)?'array':typeof value,empty:Array.isArray(value)?value.length===0:undefined});}return result;
 });
 await t.step('real-conversation','真实会话请求、NDJSON、持久消息和原会话回读',async()=>{
  const c=await conversation(t,actor.id,'fullsite');const result=await chat(t,actor.id,c.id,'first','你好，我们先聊一件简单的小事：你想怎样度过今天？',{allowError:true});
  check(result.terminal.type==='completed','真实模型回复需要completed；失败不会写成成功',result.terminal);
  check(result.messages.some(m=>m.kind==='assistant'&&m.turnId===result.turnId),'实际assistant消息已持久化');return {fid:actor.id,conversationId:c.id,turnId:result.turnId,terminal:result.terminal.type,messageIds:result.messages.map(m=>m.id)};
 });
 t.review('browser-and-preparation','真实HTTP、模型、持久消息均单独记录；初始化各阶段/视觉产物及浏览器呈现仍须独立核查。');
});
