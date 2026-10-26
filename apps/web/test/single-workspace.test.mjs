import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync,existsSync} from 'node:fs';
import {dirname,resolve} from 'node:path';
import {fileURLToPath} from 'node:url';
import {approvedRoute} from '../src/approved-ui/routing.mjs';
import {featureRouteInventory} from '../src/approved-ui/feature-routes.mjs';
const base=fileURLToPath(new URL('../src/',import.meta.url));
test('the actual production dependency graph cannot mount any retired page or shell',()=>{
 const pending=[resolve(base,'approved-ui/ApprovedApp.mjs')],seen=new Set();
 while(pending.length){const path=pending.pop();if(seen.has(path))continue;seen.add(path);
  assert.doesNotMatch(path,/\/proto\/(?:pages|modules)\/|\/views\/|LegacySurface|\/proto\/app\./);
  const source=readFileSync(path,'utf8');
  for(const match of source.matchAll(/(?:from\s*|import\(\s*)['"]([^'"]+)['"]/g)){
   if(!match[1].startsWith('.')&&!match[1].startsWith('@/'))continue;
   const target=match[1].startsWith('@/')?resolve(base,match[1].slice(2)):resolve(dirname(path),match[1]);
   const next=[target,...['.ts','.js','.mjs','.vue'].map(ext=>target+ext),resolve(target,'index.ts')].find(p=>existsSync(p)&&!p.endsWith('/'));
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
test('legacy query flags, unknown and missing routes cannot activate another implementation',()=>{
 for(const path of ['/settings','/studio','/c/role/media','/c/role/life/now']){
  const current=approvedRoute('#'+path),flagged=approvedRoute('#'+path+'?legacy=1');assert.equal(flagged.page,current.page);assert.equal(flagged.feature?.render,current.feature?.render);assert.equal(flagged.query.has('legacy'),false);
 }
 for(const path of ['/chat','/life','/worlds/nothing','/admin','/does-not-exist','/c/%/chat','/c/%E0%A4/profile','/c/%E0%A4/settings/autonomy'])assert.equal(approvedRoute('#'+path+'?legacy=1').notFound,true);
});
