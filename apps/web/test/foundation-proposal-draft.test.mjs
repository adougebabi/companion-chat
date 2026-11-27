import test from 'node:test';
import assert from 'node:assert/strict';
import {foundationProposalDraft,foundationProposalChanges,foundationProposalMode,rebaseFoundationProposal,applyProfileChanges} from '../src/approved-ui/profile-fields.mjs';
const current={revision:4,identity:{name:'林青',age:25,private_id:'retain',extra:{future:false}},behavioral_policy:{directness:.5},life_profile:{life_habits:'晚上读书',future:{sample:[null,false,0]}}};
const update=(draft,path,value)=>{draft.fields=draft.fields.map(field=>field.path.join('.')===path?{...field,value}:field);};
test('normal proposal fields only submit actual edits and retain full advanced changes across mode switches',()=>{
 const draft=foundationProposalDraft(current);update(draft,'identity.name','青岚');
 assert.deepEqual(foundationProposalChanges(draft),{identity:{name:'青岚'}});
 Object.assign(draft,foundationProposalMode(draft,'json',current));
 const extra={personality:{future_trait:0},provenance:{future:{sample:[null,false,0]}},identity:{name:'青岚'}};
 draft.changes=JSON.stringify(extra);
 Object.assign(draft,foundationProposalMode(draft,'fields',current));
 update(draft,'identity.age',0);
 assert.deepEqual(foundationProposalChanges(draft),{...extra,identity:{name:'青岚',age:0}});
 Object.assign(draft,foundationProposalMode(draft,'json',current));
 assert.deepEqual(JSON.parse(draft.changes),{...extra,identity:{name:'青岚',age:0}});
 assert.equal(current.identity.name,'林青');assert.equal(current.life_profile.future.sample[1],false);
});
test('rebase keeps intended changes and reason while adopting only the reviewed baseline',()=>{
 const draft=foundationProposalDraft(current);draft.reason='补充称呼';update(draft,'identity.name','青岚');
 const fresh={...current,revision:5,identity:{...current.identity,age:26},life_profile:{...current.life_profile,life_habits:'晚上散步'}};
 const rebased=rebaseFoundationProposal(draft,fresh);
 assert.equal(rebased.reason,'补充称呼');assert.equal(rebased.revision,5);
 assert.deepEqual(foundationProposalChanges(rebased),{identity:{name:'青岚'}});
 assert.equal(rebased.fields.find(field=>field.path.join('.')==='identity.age').value,26);
});
test('malformed JSON and invalid normal values retain the draft, explicit clear and zero retain their meaning',()=>{
 const draft=foundationProposalDraft(current);update(draft,'identity.age','');update(draft,'behavioral_policy.directness',0);
 assert.deepEqual(foundationProposalChanges(draft),{identity:{age:null},behavioral_policy:{directness:0}});
 draft.mode='json';draft.changes='{ unfinished';const before=JSON.stringify(draft);
 assert.throws(()=>foundationProposalMode(draft,'fields',current),/JSON/);assert.equal(JSON.stringify(draft),before);
 draft.changes='[false,0,null]';assert.throws(()=>foundationProposalChanges(draft),/JSON 对象/);
 const original={changes:'{"personality":{"openness":0}}',reason:'旧草稿',revision:3};
 assert.deepEqual(foundationProposalChanges(original),{personality:{openness:0}});
});
test('patch keys remain literal data and never become object prototypes',()=>{
 const patch=JSON.parse('{"identity":{"__proto__":{"future":true},"constructor":{"prototype":{"sample":0}}}}');
 const result=applyProfileChanges(current,patch);
 assert.equal(Object.getPrototypeOf(result.identity),Object.prototype);
 assert.deepEqual(JSON.parse(JSON.stringify(result)).identity.__proto__,{future:true});
 assert.equal({}.future,undefined);
});
