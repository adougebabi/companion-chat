import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
const source=readFileSync(new URL('../src/v3/proto/components/place-definition.js',import.meta.url),'utf8').replace(/^import .+;\s*$/gm,'').replaceAll('export ','');
const {bindPlaceDefinition,newPlaceDefinition,PlaceDefinitionFields}=Function('Field',source+'\nreturn {bindPlaceDefinition,newPlaceDefinition,PlaceDefinitionFields};')(f=>`<label>${f.label}<input name="${f.name}" value="${f.value}"></label>`);
test('ordinary Chinese place text needs no JSON and preserves unknown floor plus advanced fields',()=>{
 const body={...newPlaceDefinition(),custom:{kept:true}},nodes=new Map();let changes=0;
 const box={querySelector(selector){if(!nodes.has(selector))nodes.set(selector,{value:'',error:'',setCustomValidity(v){this.error=v;}});return nodes.get(selector);}};
 bindPlaceDefinition(box,body,{changed(){changes++;},repaint(){}});
 const description=box.querySelector('[name=description]');description.value='西湖边的二楼书店';description.oninput({target:description});
 assert.equal(body.description,description.value);assert.equal(body.floor,'');assert.deepEqual(body.custom,{kept:true});assert.equal(changes,1);
 assert.deepEqual(JSON.parse(box.querySelector('[name=placeDefinitionJson]').value),body);
 assert.match(PlaceDefinitionFields(body),/楼层未填写时保持未知/);
 const raw=box.querySelector('[name=placeDefinitionJson]');raw.value='{broken';raw.onchange({target:raw});assert.ok(raw.error);
 description.oninput({target:description});assert.equal(raw.error,'');
});
