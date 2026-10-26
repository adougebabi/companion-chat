import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync,existsSync,statSync} from 'node:fs';
import {dirname,resolve} from 'node:path';
import {fileURLToPath} from 'node:url';
import {approvedRoute} from '../src/approved-ui/routing.mjs';
import {featureRouteInventory} from '../src/approved-ui/feature-routes.mjs';
const base=fileURLToPath(new URL('../src/',import.meta.url));
test('the actual production dependency graph cannot mount any retired page or shell',()=>{
 const pending=[resolve(base,'main.tsx')],seen=new Set();
 while(pending.length){const path=pending.pop();if(seen.has(path))continue;seen.add(path);
  assert.doesNotMatch(path,/\/proto\/(?:pages|modules)\/|\/src\/views\/|LegacySurface|\/proto\/app\./);
  const source=readFileSync(path,'utf8');
  for(const match of source.matchAll(/(?:from\s*|import\(\s*)['"]([^'"]+)['"]/g)){
   if(!match[1].startsWith('.')&&!match[1].startsWith('@/'))continue;
   const target=match[1].startsWith('@/')?resolve(base,match[1].slice(2)):resolve(dirname(path),match[1]);
   const next=[target,...['.ts','.tsx','.js','.jsx','.mjs'].map(ext=>target+ext),resolve(target,'index.ts'),resolve(target,'index.tsx')].find(p=>existsSync(p)&&statSync(p).isFile());
   assert.ok(next,'missing production dependency '+match[1]+' from '+path);pending.push(next);
  }
 }
 assert.ok(seen.size>100);
});
test('every registered domain route preserves its selector and has an existing exported consumer',()=>{
 for(const definition of featureRouteInventory){
  const route=approvedRoute('#'+definition.path.replace(/:([a-zA-Z]+)/g,(_m,key)=>key==='charId'?'role':key==='section'?'turns':'selected%20'+key));
  assert.equal(route.page,definition.page);assert.ok(route.feature,definition.path);assert.equal(route.legacy,undefined);
  const spec=/import\(['"]([^'"]+)['"]\)/.exec(route.feature.load.toString());assert.ok(spec);
  const file=resolve(base,'approved-ui',spec[1]);assert.ok(existsSync(file));assert.match(readFileSync(file,'utf8'),new RegExp('export (?:(?:async )?function |const )'+route.feature.render+'\\b'));
 }
});
test('legacy flags are rejected and unknown routes cannot activate another implementation',()=>{
 for(const path of ['/settings','/studio','/c/role/media','/c/role/life/now']) {
  const flagged=approvedRoute('#'+path+'?legacy=1');assert.equal(flagged.removed,true);assert.equal(flagged.feature,undefined);
 }
 for(const path of ['/chat','/life','/worlds/nothing','/admin','/does-not-exist','/c/%/chat','/c/%E0%A4/profile','/c/%E0%A4/settings/autonomy'])assert.equal(approvedRoute('#'+path).notFound,true);
});
test('object selectors preserve decoded IDs and character boundaries',()=>{
 for(const [path,key,value] of [['growth/memory/mem%20one','memId','mem one'],['life/goal/goal-one','goalId','goal-one'],['life/wardrobe/item/item-one','itemId','item-one'],['life/diary/day-one','entryId','day-one']]){
  const route=approvedRoute('#/c/role/'+path);assert.equal(route.fid,'role');assert.equal(route.feature.params[key],value);
 }
});
