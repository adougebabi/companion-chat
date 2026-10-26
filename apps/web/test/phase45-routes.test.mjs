import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import {approvedRoute,bookmarkedPath} from '../src/approved-ui/routing.mjs';
const read=path=>readFileSync(new URL(path,import.meta.url),'utf8');
test('Studio, avatar, governance and original diagnostics are workspace subroutes',()=>{
 for(const [path,page] of [['/studio/edit/new?kind=place','创作中心'],['/studio/preview?res=source','创作中心'],['/c/role/media/avatar','媒体'],['/c/role/governance?tab=foundation','角色详情'],['/diag/task/original','管理中心'],['/diag/request/original','管理中心'],['/diag/record/model-runs/original','管理中心']]){
  const route=approvedRoute('#'+path);assert.equal(route.page,page);assert.ok(route.feature);assert.equal(route.legacy,undefined);
 }
});
test('bookmarked URLs have no simulated character or second router',()=>{
 assert.equal(bookmarkedPath('/instances'),'/instances');assert.equal(approvedRoute('#'+bookmarkedPath('/instances')).page,'角色库');
 assert.equal(approvedRoute('#'+bookmarkedPath('/settings','?section=模型用途')).query.get('section'),'模型用途');
 assert.equal(bookmarkedPath('/chat'),'/library');
 assert.equal(approvedRoute('#/unknown?legacy=1').notFound,true);
 assert.equal(approvedRoute('#/studio?legacy=1').page,'创作中心');
 const app=read('../src/approved-ui/ApprovedApp.mjs');assert.doesNotMatch(app,/LegacySurface|mountProduct/);
});
test('preview, upload and workflow recovery keep their real services',()=>{
 const studio=read('../src/v3-domain/studio.js'),media=read('../src/approved-ui/pages/MediaPage.vue'),workflow=read('../src/v3-domain/workflow.js');
 assert.match(studio,/persistentCommand/);assert.match(studio,/expectedInstanceRevision/);assert.match(studio,/create_required/);
 assert.match(media,/mountMediaUpload/);assert.match(media,/全部角色/);assert.match(workflow,/sourceRunId|source_run_id/);
});
