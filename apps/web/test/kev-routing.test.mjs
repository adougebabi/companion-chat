import assert from 'node:assert/strict';
import test from 'node:test';
import {approvedRoute} from '../src/approved-ui/routing.mjs';
test('Kev bookmarks resolve inside the single settings and diagnostic workspace',()=>{
 const settings=approvedRoute('#/settings/kev');assert.equal(settings.page,'管理中心');assert.equal(settings.query.get('section'),'高级');
 const diagnostics=approvedRoute('#/diag/kev');assert.equal(diagnostics.page,'管理中心');assert.equal(diagnostics.feature.params.section,'kev');assert.equal(diagnostics.legacy,undefined);
});
