import assert from 'node:assert/strict';
import test from 'node:test';
import {planDefaultGroupMembership} from '../src/lib/group-membership.ts';
import {normalizeActorGroups} from '../src/lib/actor-groups.ts';
test('default group planning assigns only actors that are not already grouped',()=>{
 const groups=[{id:'default',name:'默认',actor_ids:['A']},{id:'other',name:'朋友',actor_ids:['B']}];
 assert.deepEqual(planDefaultGroupMembership(groups,['A','B','C']),{defaultGroup:groups[0],ungroupedActorIds:['C']});
});
test('group normalization accepts historical members and drops malformed identities',()=>{
 assert.deepEqual(normalizeActorGroups([{id:'G',name:'朋友',members:['A',false,'B']},null]),[{id:'G',name:'朋友',actor_ids:['A','B']}]);
 assert.deepEqual(normalizeActorGroups(null),[]);
});
