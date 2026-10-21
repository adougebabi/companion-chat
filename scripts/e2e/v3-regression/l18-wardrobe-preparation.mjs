import {runScenario,check,equal} from './runtime.mjs';
import {fixture,fidPath,goArgs,nodeArgs} from './domain.mjs';
await runScenario('L18','初始穿搭准备、场景标签与实际穿着',async t=>{
 const f=await t.step('fixture','Activate an isolated character with the real wardrobe initializer',()=>fixture(t,'wardrobe',{description:'创建虚构成年角色$NAME，住在温和气候地区，时区Asia/Shanghai，性格平和，喜欢实用简洁的日常服装。未声明已拥有衣物，请由独立穿搭准备流程按人格补齐初始搭配。'}));if(!f)return;
 const p=fidPath(f.id);
 const done=await t.step('preparation','Wait for actual Worker and formal Agent, not just admission or task ID',async()=>{
  t.model('Background wardrobe preparation and context selection');const result=await t.until('初始穿搭（睡眠会真实等待；超时不计通过）',()=>t.get(p+'/inventory/preparation'),value=>value.status==='completed');
  equal(result.fluctlight_id,f.id,'Preparation belongs to this character');check(result.task_id&&result.result?.inventory_committed===true,'Completion requires persisted wardrobe results');await t.remember('wardrobePreparation',{id:result.id,taskId:result.task_id});return result;
 });
 await t.step('plans','Read real stock, multi-tag plans and multiple accessory identities',async()=>{
  if(!done)t.blocked('PREPARATION_REQUIRED','Preparation did not complete');const stock=await t.pages(p+'/inventory?kind=wearable&limit=100'),plans=await t.pages(p+'/inventory/outfits'),directory=await t.get(p+'/inventory/tags');
  check(stock.length>0&&plans.length>0,'Must have actual inventory and plans');const ids=new Set(stock.map(i=>i.id));for(const plan of plans){check(Array.isArray(plan.tags)&&plan.tags.length>0,'Initial plans require meaningful scenario tags');check(plan.tags.every(tag=>directory.tags.includes(tag)),'Saved tags must be in the maintained directory');check(plan.items.length&&plan.items.every(item=>ids.has(item.id)),'Every plan uses real owned inventory IDs');}
  for(const item of stock.filter(i=>i.source_ref===`wardrobe-preparation:${done.id}`)){equal(item.source_kind,'initialization','Starter stock has controlled initialization provenance');check(item.source_verified===true,'New inventory needs verified source');check(Array.isArray(item.seasons),'Season values must persist as an array');}
  return {items:stock.map(i=>({id:i.id,type:i.clothing_type,seasons:i.seasons})),plans:plans.map(o=>({id:o.id,tags:o.tags}))};
 });
 await t.step('wear','Actual wardrobe.wear receipt and refreshed current wearing agree',async()=>{
  if(!done)t.blocked('PREPARATION_REQUIRED','No completed preparation receipt');const receipt=done.result.wear_receipt;check(receipt?.operation_id&&receipt.result?.capability_name==='wardrobe.wear','Must retain the actual domain Tool receipt');check(receipt.result.status==='completed'||receipt.result.status==='handled'&&receipt.result.output?.warning_code==='wearing_unchanged','Only actual wear or proof of unchanged wear is completion');
  const actual=await t.get(p+'/wardrobe'),again=await t.get(p+'/inventory/preparation');equal(again.result.wear_receipt.operation_id,receipt.operation_id,'Refresh must retain the exact effect identity');
  const selected=[...done.result.wear_item_ids].sort();equal(actual.worn_items.map(i=>i.id).sort(),selected,'Current wearing must match the actual selected inventory');check(actual.worn_items.every(i=>i.source_verified===true),'Wearing requires verified sources');return {operationId:receipt.operation_id,items:selected};
 });
 await t.step('guards','Isolated CAS, stale context, multiple accessories, archived rename and exact replay regressions',async()=>({go:await t.suite('wardrobe-guards',goArgs(['TestWardrobeTagsSeasonsAndMultipleAccessoriesPersist','TestWardrobeSelectionFenceRejectsStaleAndReplaysCommittedReceipt','TestWardrobeCompleteInventoryRejectsExtraStarterStock','TestWardrobePreparationFormalAgentCommitsInventoryThenRealWearAndReplays','TestWardrobeRemovedProfileRetiresOnlyUncommittedSelection','TestWardrobeRemovedProfileStillReplaysExistingWearEffect','TestWardrobePreparationCannotCommitAfterCharacterPause']),{database:true}),components:await t.suite('wardrobe-ui',nodeArgs(['v3-wardrobe-world.test.mjs']))}));
 t.review('persona-and-style','Review that starter garments and selected scenario fit the actual persona, season and situation. Verify composer/library/clothing tabs after refresh.',{fid:f.id});
});
