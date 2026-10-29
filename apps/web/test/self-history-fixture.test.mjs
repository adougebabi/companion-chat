import test from 'node:test';
import assert from 'node:assert/strict';
import {growthFixture} from './fixtures/growth-fixture.mjs';
const owner='self-owner',fid='self-role';
function fixture(scenario='normal') {
  const handle=growthFixture({owner,scenario});
  return async (path,method='GET',body={})=>{
    let result;
    assert.equal(await handle({path,method,body,url:new URL(path,'http://127.0.0.1'),res:{},json:(_r,value,status=200)=>{result={value,status};}}),true);
    return result;
  };
}
const path='/api/fluctlights/'+fid+'/self-claims/'+fid+'-claim-one';
test('self fixture preserves full snapshots, separate review ledger and append-only governance history',async()=>{
  const call=fixture(), initial=(await call(path)).value, original=(await call(path+'/revisions')).value.items;
  assert.deepEqual(original.map(r=>r.revision),[2,1]);
  for(const r of original){assert.equal(r.after_value.revision,r.revision);assert.equal(r.candidate.revision,r.revision);assert.equal(r.claim_id,initial.id);assert.equal(r.fluctlight_id,fid);for(const key of ['category','claim','value','confidence','evidence_refs','provenance','status','expires_at','superseded_by'])assert.ok(Object.hasOwn(r.after_value,key),key);}
  const review=(await call(path+'/review','POST',{expectedRevision:2,reason:'已核对来源',idempotencyKey:'review-one'})).value;
  assert.ok(review.review_id);assert.equal(review.revision,2);
  const reviewed=(await call(path)).value;
  assert.deepEqual({...reviewed,owner_review:null},initial);
  assert.deepEqual((await call(path+'/revisions')).value.items,original);
  const forgetBody={expectedRevision:2,reason:'停止采用，保留来源',idempotencyKey:'forget-one'};
  const forgotten=(await call(path+'/forget','POST',forgetBody)).value;
  assert.equal(forgotten.status,'forgotten');assert.equal(forgotten.revision,3);
  assert.deepEqual((await call(path+'/forget','POST',forgetBody)).value,forgotten);
  assert.equal((await call(path+'/forget','POST',{...forgetBody,idempotencyKey:'new-stale'})).status,409);
  const history=(await call(path+'/revisions')).value.items;
  assert.deepEqual(history.slice(1),original);assert.deepEqual(history[0].before_value,original[0].after_value);
  assert.equal(history[0].candidate.reason,forgetBody.reason);assert.equal(history[0].candidate.owner_actor_id,owner);
  assert.equal(history[0].source_window,'owner:'+owner);assert.equal(history[0].reason_code,'owner_forget');assert.equal(history[0].base_revision,2);
  const current=(await call(path)).value;
  assert.equal(current.owner_review,null);assert.notEqual(current.updated_at,initial.updated_at);
  for(const key of ['evidence_refs','provenance','value','confidence','expires_at','superseded_by'])assert.deepEqual(current[key],initial[key]);
  const restored=(await call(path+'/restore','POST',{expectedRevision:3,targetRevision:1,reason:'恢复原始可追溯内容',idempotencyKey:'restore-one'})).value;
  assert.equal(restored.revision,4);assert.equal(restored.target_revision,1);
  const after=(await call(path)).value, all=(await call(path+'/revisions')).value.items;
  assert.equal(after.claim,original[1].after_value.claim);assert.equal(after.status,'active');
  assert.deepEqual(all.map(r=>r.revision),[4,3,2,1]);assert.deepEqual(all.slice(1),history);
  assert.equal(all[0].before_value.revision,3);assert.equal(all[0].after_value.revision,4);assert.equal(all[0].reason_code,'owner_restore');
  assert.equal((await call(path+'/reviews')).value.items[0].id,review.review_id);
  const other=path.replaceAll(fid,'other-role');assert.equal((await call(other)).value.revision,2);
});
test('self unknown response replays original receipt before CAS and never appends twice',async()=>{
  const call=fixture('unknown'),body={expectedRevision:2,reason:'原请求结果核对',idempotencyKey:'unknown-one'};
  assert.equal((await call(path+'/forget','POST',body)).status,503);
  const receipt=await call(path+'/forget','POST',body);
  assert.equal(receipt.status,200);assert.equal(receipt.value.revision,3);
  assert.deepEqual((await call(path+'/revisions')).value.items.map(r=>r.revision),[3,2,1]);
  assert.equal((await call(path+'/restore','POST',{expectedRevision:3,targetRevision:3,reason:'不能恢复遗忘快照',idempotencyKey:'invalid-target'})).status,422);
  assert.equal((await call(path)).value.revision,3);
});
