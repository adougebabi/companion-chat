import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
const source=readFileSync(new URL('../src/v3/proto/components/item-definition.js',import.meta.url),'utf8').replace(/^import .+;\s*$/gm,'').replaceAll('export ','');
const bind=Function('wardrobeCategoryOptions','wardrobeSlotOptions','esc',source+'\nreturn bindItemDefinition;')(()=>[{value:'book',label:'Book'}],kind=>kind==='wearable'?[{value:'top',label:'Top'}]:[{value:'',label:'No slot'}],String);
function fixture(){const nodes=Object.fromEntries(['item_kind','category','slot','description','itemDefinitionJson'].map(key=>[key,{value:'',error:'',disabled:true,required:false,setCustomValidity(value){this.error=value;}}]));const body={schema:'item-definition.v1',item_kind:'object',category:'book',slot:'',description:'old'};bind({querySelector:selector=>nodes[selector.match(/name=(\w+)/)[1]]},body,{changed(){},repaint(){}});return {nodes,body};}
test('repairing invalid advanced item JSON through an ordinary field clears the original form error',()=>{
 const {nodes,body}=fixture();nodes.itemDefinitionJson.value='{broken';nodes.itemDefinitionJson.oninput({target:nodes.itemDefinitionJson});assert.ok(nodes.itemDefinitionJson.error);nodes.description.value='saved draft';nodes.description.oninput({target:nodes.description});assert.equal(nodes.itemDefinitionJson.error,'');assert.equal(JSON.parse(nodes.itemDefinitionJson.value).description,'saved draft');assert.equal(body.description,'saved draft');
});
test('advanced JSON changing an object to clothing makes the slot editable and preserves custom categories',()=>{
 const {nodes,body}=fixture();nodes.itemDefinitionJson.value=JSON.stringify({schema:'item-definition.v1',item_kind:'wearable',category:'custom-category',slot:'custom-slot',description:'explicit clothing'});nodes.itemDefinitionJson.oninput({target:nodes.itemDefinitionJson});assert.equal(nodes.itemDefinitionJson.error,'');assert.equal(nodes.slot.disabled,false);assert.equal(nodes.slot.required,true);assert.equal(nodes.slot.value,'custom-slot');assert.match(nodes.category.innerHTML,/custom-category/);assert.equal(body.item_kind,'wearable');
});

test('binding a saved definition restores its actual category instead of the placeholder',()=>{
 const {nodes,body}=fixture();
 assert.equal(nodes.item_kind.value,body.item_kind);
 assert.equal(nodes.category.value,'book');
 assert.equal(nodes.description.value,'old');
});
