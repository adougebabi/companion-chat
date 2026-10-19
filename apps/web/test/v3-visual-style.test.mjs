import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';

function load(path,names,bindings){
 const source=readFileSync(new URL(path,import.meta.url),'utf8').replace(/^import .+;\s*$/gm,'').replaceAll('export ','');
 return Function(...Object.keys(bindings),source+`\nreturn {${names.join(',')}};`)(...Object.values(bindings));
}
const fields=['appearance','clothing','constraints','references'].map(key=>[key,key,'']);
test('invalid advanced JSON becomes submittable after a normal field repairs its value',()=>{
 const nodes=Object.fromEntries([...fields.map(([key])=>key),'visualStyleJson'].map(key=>[key,{value:'',error:'',setCustomValidity(value){this.error=value;}}]));
 const box={querySelector:selector=>nodes[selector.match(/name=(\w+)/)[1]]},body={schema:'visual-style.v1',appearance:'old'};let changes=0;
 const {bindVisualStyle}=load('../src/v3/proto/components/visual-style.js',['bindVisualStyle'],{visualBriefFields:fields});
 bindVisualStyle(box,body,{changed:()=>changes++});nodes.visualStyleJson.value='{broken';nodes.visualStyleJson.oninput({target:nodes.visualStyleJson});assert.ok(nodes.visualStyleJson.error);
 nodes.appearance.value='repaired guidance';nodes.appearance.oninput({target:nodes.appearance});
 assert.equal(nodes.visualStyleJson.error,'');assert.equal(JSON.parse(nodes.visualStyleJson.value).appearance,'repaired guidance');assert.equal(changes,1);
});
test('saving visual guidance updates its origin projection without remounting or clearing new inputs',()=>{
 const nodes=new Map();const el={innerHTML:'',querySelector(selector){if(!nodes.has(selector))nodes.set(selector,{innerHTML:''});return nodes.get(selector);}};
 let options,mounts=0,disposed=false,reloads=0;const snapshot={revision:1,brief:{source_resource_id:'style',source_version:'1',source_overrides:{appearance:'old'}}};
 const {renderRealVisualEditor}=load('../src/v3/proto/pages/real-visual.js',['renderRealVisualEditor'],{
  asyncPage:(_el,_read,paint)=>paint(snapshot,()=>{},()=>!disposed),visualClient:{read(){}},get:()=>({status:'active'}),visualBriefFields:fields,
  VisualBriefSource:brief=>JSON.stringify(brief?.source_overrides),mountVersionedFields:(_box,value)=>{mounts++;options=value;return()=>{};},
  mountCursorRecords:()=>({reload(){reloads++;},dispose(){}}),
 });
 const cleanup=renderRealVisualEditor(el,{charId:'character'});const form=el.querySelector('[data-editor]');form.newDraft='typed during save';
 options.onSaved({revision:2,brief:{...snapshot.brief,source_overrides:{appearance:'new saved'}}});
 assert.equal(el.querySelector('[data-brief-source]').innerHTML,JSON.stringify({appearance:'new saved'}));assert.equal(form.newDraft,'typed during save');assert.equal(mounts,1);assert.equal(reloads,1);
 disposed=true;cleanup();options.onSaved({brief:{source_overrides:{appearance:'late'}}});assert.match(el.querySelector('[data-brief-source]').innerHTML,/new saved/);
});
