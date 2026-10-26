import assert from 'node:assert/strict';
import test from 'node:test';
import {acceptancePath} from '../src/v3/mock-navigation.mjs';
import {approvedRoute} from '../src/approved-ui/routing.mjs';
test('Kev bookmarks resolve to current React configuration and diagnostic pages',()=>{
 const setting=approvedRoute('#'+acceptancePath('/settings','?section=kev'));
 assert.equal(setting.page,'管理中心');assert.equal(setting.query.get('section'),'高级');
 const diagnostic=approvedRoute('#'+acceptancePath('/diagnostics','?section=kev-decisions'));
 assert.equal(diagnostic.page,'运行详情');assert.equal(diagnostic.diagnostic.kind,'kev');
 assert.equal(approvedRoute('#/diag/kev?legacy=1').removed,true);
});
