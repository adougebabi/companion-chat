import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import {approvedRoute,approvedHref,lifePages} from '../src/approved-ui/routing.mjs';
const read=path=>readFileSync(new URL(path,import.meta.url),'utf8');
test('all active life routes use the same workspace and retain the actual character',()=>{
 for(const page of lifePages){const route=approvedRoute(approvedHref(page,'actual-role'));assert.equal(route.page,page);assert.equal(route.fid,'actual-role');assert.equal(route.legacy,undefined);}
 assert.equal(approvedRoute('#/c/actual-role/life?legacy=1').page,'此刻');
});
test('object deep links preserve selectors for the sole domain implementation',()=>{
 for(const [url,name,id] of [['#/c/actual-role/growth/memory/mem%20one','memId','mem one'],['#/c/actual-role/life/goal/goal-one','goalId','goal-one'],['#/c/actual-role/life/wardrobe/item/item-one','itemId','item-one'],['#/c/actual-role/life/diary/day-one','entryId','day-one']]){
  const route=approvedRoute(url);assert.equal(route.fid,'actual-role');assert.equal(route.feature.params[name],id);assert.equal(route.legacy,undefined);
 }
});
test('current life commands retain original request and revision boundaries',()=>{
 const life=read('../src/approved-ui/pages/LifePage.mjs'),wardrobe=read('../src/approved-ui/pages/WardrobePage.mjs'),memory=read('../src/v3-domain/memory.js');
 assert.match(life,/w\.command/);assert.match(life,/expectedLifeContextRevision/);assert.match(wardrobe,/requestUnresolved/);assert.match(memory,/memoryClient\.revise/);assert.match(memory,/rollback|restore/);
});
