import assert from 'node:assert/strict';
import {createSSRApp,h,nextTick} from 'vue';
import {renderToString} from 'vue/server-renderer';
const nodes=node=>!node?[]:Array.isArray(node)?node.flatMap(nodes):typeof node==='object'?[node,...nodes(node.children)]:[];
const text=node=>!node?'':Array.isArray(node)?node.map(text).join(''):typeof node==='object'?text(node.children):String(node);
async function capture(component){let tree,render;const app=createSSRApp({setup(props,context){render=component.setup(props,context);return (...args)=>{tree=render(...args);return h('div');};}});await renderToString(app);return {tree,async snapshot(){let result;await renderToString(createSSRApp({render(...args){result=render(...args);return h('div');}}));tree=result;return result;}};}
export async function assertApprovedFixtureWardrobe(){
 const previous=Object.fromEntries(['document','window','location','fetch','setTimeout','clearTimeout'].map(k=>[k,globalThis[k]]));let network=0;
 globalThis.document={addEventListener(){},querySelectorAll(){return[];},getElementById(){return null;}};globalThis.window={addEventListener(){},removeEventListener(){}};globalThis.location={hash:'#'+encodeURIComponent('衣柜')};globalThis.fetch=()=>{network++;throw Error('fixture must not use real transport');};
 try{
  const {default:Reference,approvedReferenceData}=await import('../../src/approved-ui/reference-fixture.mjs');
  const root=await capture(Reference),all=nodes(root.tree),simulate=all.find(n=>n.props?.['data-testid']==='simulate-failure');assert.ok(simulate);simulate.props['onUpdate:modelValue'](false);
  const switcher=all.find(n=>n.type==='button'&&n.props?.class==='character-switch'),name=text(switcher).trim();
  const character=approvedReferenceData.characters.find(c=>c.name===name);assert.ok(character,name);
  const view=all.find(n=>n.type?.__name==='WardrobePage');assert.ok(view);const wardrobe=await capture(view.type);
  const library=nodes(wardrobe.tree).find(n=>n.type==='button'&&text(n).trim()==='方案库');assert.ok(library);library.props.onClick();await nextTick();
  const ids=items=>Object.values(items).flat().sort().join('|');
  // Prepare a plan using only existing fixture-owned IDs. Wear itself still
  // runs through the actual demo button callback and simulated task lifecycle.
  const subset=JSON.parse(JSON.stringify(character.wearing)),slot=Object.keys(subset).find(k=>subset[k]?.length);assert.ok(slot);subset[slot]=subset[slot].slice(1);character.outfits.push({id:'r10-controlled-plan',name:'测试：已有服饰组合',tag:character.outfitTags[0]||'日常',items:subset});
  async function wearDifferent(){const target=character.outfits.find(o=>ids(o.items)!==ids(character.wearing));assert.ok(target);const tree=await wardrobe.snapshot();const card=nodes(tree).find(n=>n.type==='article'&&text(n).includes(target.name));assert.ok(card);const button=nodes(card).find(n=>n.type==='button'&&text(n).includes('换上'));assert.ok(button);button.props.onClick();}
  const queue=[];globalThis.setTimeout=(fn,ms)=>{const item={fn,ms};queue.push(item);return item;};globalThis.clearTimeout=item=>{const at=queue.indexOf(item);if(at>=0)queue.splice(at,1);};
  async function settle(){for(let i=0;i<30;i++){const task=queue.shift();if(task)task.fn();await Promise.resolve();await nextTick();if(!queue.length&&i>5)break;}}
  const before=ids(character.wearing);await wearDifferent();await settle();assert.notEqual(ids(character.wearing),before);
  const confirmed=JSON.parse(JSON.stringify(character.wearing));simulate.props['onUpdate:modelValue'](true);await wearDifferent();await settle();assert.deepEqual(character.wearing,confirmed);assert.equal(network,0);
 }finally{for(const [key,value]of Object.entries(previous))if(value===undefined)delete globalThis[key];else globalThis[key]=value;}
}
