import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
const source=readFileSync(new URL('../src/v3/proto/components/studio-foundation.js',import.meta.url),'utf8').replace(/^import .+;\s*$/gm,'').replace('export function','function');
const render=Function('esc',source+'\nreturn FoundationSourceLinks;')(v=>String(v).replaceAll('<','&lt;').replaceAll('"','&quot;'));
test('a persisted Foundation proposal links to governed acceptance and never claims it is effective',()=>{
 const view=render({referenced_by:[{target_kind:'foundation',fluctlight_id:'character',runtime_id:'proposal/one',running_version:'7',instance_revision:4,status:'proposed',currently_effective:false}]});
 assert.match(view,/提案待审核 · 尚未生效/);assert.match(view,/#\/c\/character\/governance\/foundation\/proposal%2Fone/);assert.doesNotMatch(view,/当前生效/);
});
test('current and historical applications use server state and exclude unrelated domain references',()=>{
 const view=render({referenced_by:[{target_kind:'world',status:'world-hidden'},{target_kind:'foundation',fluctlight_id:'character',runtime_id:'current',running_version:'3',instance_revision:9,status:'accepted',currently_effective:true},{target_kind:'foundation',fluctlight_id:'character',runtime_id:'historical',running_version:'2',instance_revision:8,status:'accepted',currently_effective:false}]});
 assert.match(view,/当前生效/);assert.match(view,/已接受历史/);assert.doesNotMatch(view,/world-hidden/);assert.equal(render(null),'');
});
