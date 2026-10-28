// Explicit task-owned real HTTP verification. All identities and retry bodies
// are retained by the existing regression harness in a private run directory.
import {pathToFileURL} from 'node:url';
const {runScenario,check,equal}=await import(pathToFileURL(process.cwd()+'/scripts/e2e/v3-regression/runtime.mjs'));
const {createResource,publish,resourcePath,reason}=await import(pathToFileURL(process.cwd()+'/scripts/e2e/v3-regression/domain.mjs'));
await runScenario('UI-CATALOG','全站审计：真实资源持久化与发布',async t=>{
 const definitions={
  world:{background:'一座靠海、被河流穿过的小城。',rules:'时间自然流动，人物保留连续生活。',extensions:{nested:[{name:'扩展字段',count:0,enabled:false,note:null}]}},
  place:{description:'临河的独立书店',alias:'迟山',floor:'1F',atmosphere:'安静自然',furniture:['原木书架',{name:'阅读桌',count:2}],layout:'前区新书、中间阅读桌'},
  item:{schema:'item-definition.v1',item_kind:'wearable',category:'upper_body',slot:'upper_body',description:'浅色棉质衬衫'},
  event_template:{schema_version:'world-event.v1',description:'一阵微风',category:'environment',intensity:'low',duration_minutes:30,trigger:{mode:'manual',interval_minutes:60,cooldown_minutes:120,probability:1},condition:{day:'daily',start:'00:00',end:'00:00',place_ids:[],require_presence:false},effects:[{kind:'atmosphere',scope:'world',value:'轻柔的风'}]},
  character_template:{schema:'character-template.v1',description:'林晚，独立书店店主，温柔而有主见，喜欢雨天与读书。'},
  persona_fragment:{schema:'persona-fragment.v1',changes:{identity:{occupation:'独立书店店主'},personality:{openness:.72}}},
  visual_style:{schema:'visual-style.v1',appearance:'自然柔和',clothing:'舒适的针织衣着',constraints:'保留已确认的身体与实际穿着',references:'没有外部图片授权'},
  npc:{role:'咖啡馆店主',background:'在小城经营一家咖啡馆，有独立的工作和日常。'},
 };
 const published={};
 for(const [kind,body]of Object.entries(definitions)){
  const result=await t.step('create-publish-'+kind,'真实保存、同key重放、读取、精确版本发布：'+kind,async()=>{
   const draft=await createResource(t,kind,kind,body),read=await t.get(resourcePath(draft.id));equal(read.body,body,'Saved definition retains exact nested values');
   const original=t.state.commands['UI-CATALOG:resource-'+kind],replay=(await t.request(original.path,{method:original.method,body:original.body,expected:[200,201]})).value;equal(replay.id,draft.id,'Creation replay must retain original resource ID');
   const p=await publish(t,kind,read),exact=await t.get(resourcePath(p.id)+'/versions/'+encodeURIComponent(p.published_version));equal(exact.immutable,true,'Publication must be immutable');equal(exact.body,body,'Publication preserves original full body');
   return {id:p.id,kind,revision:p.revision,published_version:p.published_version};
  });if(result)published[kind]=result;
 }
 if(published.item)await t.step('create-publish-outfit','真实搭配定义引用精确已发布物品，不生成库存或穿着',async()=>{
  const body={schema:'outfit-definition.v1',description:'小城日常',members:[{resource_id:published.item.id,resource_version:published.item.published_version}]};const p=await publish(t,'outfit',await createResource(t,'outfit','outfit',body));equal((await t.get(resourcePath(p.id))).body,body,'Outfit uses exact immutable definition reference');return {id:p.id,published_version:p.published_version};
 });
 if(published.world)await t.step('draft-version-copy-archive','新草稿不改变发布版本，复制独立草稿且归档恢复可读',async()=>{
  const p=await t.get(resourcePath(published.world.id)),original=await t.get(resourcePath(p.id)+'/versions/'+encodeURIComponent(p.published_version));
  const saved=(await t.command('world-draft','PUT',resourcePath(p.id),()=>({kind:'world',name:p.name,body:{...original.body,background:'只在新草稿中的背景'},expectedVersion:p.draft_version,reason,idempotencyKey:t.token('world-draft')}))).value;
  equal((await t.get(resourcePath(p.id)+'/versions/'+encodeURIComponent(p.published_version))).body,original.body,'Saved draft cannot rewrite publication');
  const copy=(await t.command('world-copy','POST',resourcePath(p.id)+'/copy',()=>({source:'published',sourceVersion:p.published_version,name:'全站审计独立世界副本',expectedRevision:saved.revision,reason,idempotencyKey:t.token('world-copy')}))).value;
  check(copy.id!==p.id&&!copy.published_version,'Copied definition is independent unpublished draft');
  const archived=(await t.command('world-copy-archive','DELETE',resourcePath(copy.id),()=>({expectedRevision:copy.revision,reason,idempotencyKey:t.token('world-copy-archive')}))).value;equal(archived.status,'archived','Archive persisted');
  const restored=(await t.command('world-copy-restore','POST',resourcePath(copy.id)+'/restore',()=>({expectedRevision:archived.revision,reason,idempotencyKey:t.token('world-copy-restore')}))).value;check(restored.status!=='archived','Restore persisted');
  return {source:p.id,copy:copy.id,restored:restored.status,originalPublication:p.published_version};
 });
 await t.step('notification-configuration','真实配置CAS保存、同key回放、旧版本拒绝与重新读取',async()=>{
  const key='product.notifications';let base=t.object('notification-baseline');if(!base){const c=await t.get('/api/configuration-catalog');base={value:c.effective_values[key],version:c.versions[key]||0};check(base.value,'Actual notification defaults required');await t.remember('notification-baseline',base);}
  const body={values:{[key]:base.value},expectedVersions:{[key]:base.version},idempotencyKey:t.token('notification-setting')};
  const saved=(await t.command('notification-setting','PUT','/api/settings',body)).value;const current=await t.get('/api/settings');equal(current.values[key],base.value,'Readback retained actual preference');equal(current.versions[key],saved.versions[key],'Readback matches saved version');
  equal((await t.request('/api/settings',{method:'PUT',body})).value.versions[key],saved.versions[key],'Replay cannot advance version');
  await t.request('/api/settings',{method:'PUT',body:{...body,idempotencyKey:t.token('notification-setting-stale')},expected:[409]});return {revision:current.versions[key],staleRejected:true};
 });
 t.review('browser','API/持久化验证不代表该页视觉、焦点、弹窗和浏览器状态通过。');
});
