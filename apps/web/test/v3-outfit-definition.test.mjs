import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
const source=readFileSync(new URL('../src/v3/proto/components/outfit-definition.js',import.meta.url),'utf8').replace(/^import .+;\s*$/gm,'').replaceAll('export ','');
function fixture(body){let config;const nodes=new Map(),box={isConnected:true,querySelector(selector){if(!nodes.has(selector))nodes.set(selector,{value:'',error:'',innerHTML:'',setCustomValidity(value){this.error=value;}});return nodes.get(selector);},querySelectorAll:()=>[]};const api=Function('esc','mountCursorRecords',source+'\nreturn {mountOutfitDefinition,OutfitApplicationMembers};')(String,(_box,options)=>{config=options;return {dispose(){}};});const panel=api.mountOutfitDefinition(box,body,{alive:()=>true,changed(){}});return {body,box,panel,api,picker:()=>config};}
test('ordinary outfit description repairs a previous JSON error while preserving exact member references',()=>{
 const f=fixture({schema:'outfit-definition.v1',description:'old',members:[{resource_id:'garment',resource_version:'2'}]}),raw=f.box.querySelector('[name=outfitDefinitionJson]');raw.value='{bad';raw.oninput();assert.ok(raw.error);const description=f.box.querySelector('[name=outfitDescription]');description.value='explicit edit';description.oninput();assert.equal(raw.error,'');assert.deepEqual(JSON.parse(raw.value).members,[{resource_id:'garment',resource_version:'2'}]);f.panel.dispose();
});
test('picker captures actual published version, rejects duplicates and ignores a disposed parent',()=>{
 const f=fixture({schema:'outfit-definition.v1',description:'',members:[]}),button={dataset:{addDefinition:'garment'}},list={querySelectorAll:()=>[button]};f.picker().bind(list,{items:[{id:'garment',published_version:'7'}]});button.onclick();button.onclick();assert.deepEqual(f.body.members,[{resource_id:'garment',resource_version:'7'}]);f.panel.dispose();f.picker().bind(list,{items:[{id:'garment',published_version:'8'}]});button.onclick();assert.equal(f.body.members.length,1);
});
test('a malformed old member remains repairable when another verified reference is explicitly added',()=>{
 const f=fixture({schema:'outfit-definition.v1',description:'',members:[null]}),button={dataset:{addDefinition:'garment'}};f.picker().bind({querySelectorAll:()=>[button]},{items:[{id:'garment',published_version:'2'}]});assert.doesNotThrow(()=>button.onclick());assert.equal(f.body.members[0],null);assert.equal(f.body.members[1].resource_version,'2');f.panel.dispose();
});
