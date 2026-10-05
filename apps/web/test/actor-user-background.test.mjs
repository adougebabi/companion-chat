import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";
import { parseActorUserSettings } from "../src/lib/actor-user-background.ts";

test("explicit user background preserves false, unknown and omitted attributes",()=>{
 const input={background:{name:"Vinson",timezone:null,meeting_confirmed:false}};
 assert.deepEqual(parseActorUserSettings(input),input);
 assert.equal(parseActorUserSettings({background:{meeting_confirmed:"false"}}),null);
 assert.equal(parseActorUserSettings({background:{actor_id:"foreign"}}),null);
 assert.equal(parseActorUserSettings({background:{timezone:""}}),null);
 assert.equal(parseActorUserSettings(null),null);
 assert.deepEqual(parseActorUserSettings({background:{}}),{background:{}});
});

test("creation retains actor_user through edited preview and activation",async()=>{
 const source=await readFile(new URL("../src/views/InstancesView.vue",import.meta.url),"utf8");
 assert.match(source,/parseActorUserSettings\(candidate\.actor_user\)/);
 assert.match(source,/actor_user: actorUser/);
 assert.match(source,/actorUser: foundation\.actor_user/);
 assert.match(source,/设置用户背景（actor_user）/);
});

test("user background is viewed separately and edited only through owner governance",async()=>{
 const [detail,governance,store]=await Promise.all([
  readFile(new URL("../src/components/instances/InstanceDetailsDialog.vue",import.meta.url),"utf8"),
  readFile(new URL("../src/views/GovernanceView.vue",import.meta.url),"utf8"),
  readFile(new URL("../src/stores/control-center.ts",import.meta.url),"utf8"),
 ]);
 assert.match(detail,/detail\.value\.actor_user/);
 assert.match(detail,/<h3>用户背景<\/h3>/);
 assert.doesNotMatch(detail,/saveActorUserBackground/);
 assert.match(governance,/saveActorUserBackground/);
 assert.match(governance,/纠正原信息/);assert.match(governance,/情况发生变化/);
 assert.match(store,/idChanged \|\| !this\.actorUserBackgroundDirty/);
 assert.match(store,/expectedCurrentFactsRevision:this\.actorUserBackgroundRevision/);
 assert.match(store,/this\.fluctlightDetailFluctlightId !== fluctlightId/);
});
