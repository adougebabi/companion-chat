import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {beginDraftSubmission,settleDraftSubmission} from '../src/v3/proto/components/draft-submission.js';

function loadComponent(file,bindings,exported){
 const source=readFileSync(new URL(`../src/v3/proto/components/${file}`,import.meta.url),'utf8').replace(/^import .+;\s*$/gm,'').replaceAll('export function ','function ');
 return Function(...Object.keys(bindings),`${source}\nreturn ${exported};`)(...Object.values(bindings));
}
function fakeDrawer(initial={}){
 const fields=new Map();
 const el={isConnected:true,querySelector(selector){if(!fields.has(selector))fields.set(selector,{value:initial[selector.match(/name=([^\]]+)/)?.[1]]??'',hidden:false,disabled:false,textContent:'',innerHTML:'',reportValidity:()=>true});return fields.get(selector);}};
 return {el,close(){el.isConnected=false;}};
}
test('explicit rebase survives closing and reopening the actual command drawer',async()=>{
 let current,submitted;const initial={};
 const governanceCommand=loadComponent('governance-command.js',{
  drawer:()=>current=fakeDrawer(initial),esc:String,confirm:async()=>true,toast:()=>{},Field:({name,value})=>{initial[name]=value;return '';},JsonDetails:JSON.stringify,
  get:()=>({actorId:'inventory-test-owner'}),subscribe:()=>{},beginDraftSubmission,settleDraftSubmission,
 },'governanceCommand');
 const options={scope:['fid','item','archive'],title:'archive',note:'test',revision:2,target:{expectedItemRevision:3},
  read:async()=>({revision:8,item:{revision:9}}),rebaseTarget:latest=>({expectedItemRevision:latest.item.revision}),
  execute:async(body,target)=>{submitted={body,target};return {saved:true};}};
 governanceCommand(options);
 current.el.querySelector('[name=reason]').value='preserved reason';current.el.querySelector('[name=reason]').oninput();
 await current.el.querySelector('[data-rebase]').onclick();current.close();
 governanceCommand(options);
 await current.el.querySelector('[data-submit]').onclick();
 assert.equal(submitted.body.expectedRevision,8);assert.equal(submitted.body.reason,'preserved reason');
 assert.deepEqual(submitted.target,{expectedItemRevision:9});assert.equal('target' in submitted.body,false);
});
test('late inventory acceptance observes its original page and never opens an unrelated receipt',async()=>{
 let command,receipts=0,alive=true;
 const window={location:{hash:'#/c/original/life/wardrobe'}};
 const request=loadComponent('inventory-actions.js',{
  inventoryClient:{},get:()=>null,subscribe:()=>{},Field:()=>'',governanceCommand:config=>command=config,
  requestReceipt:()=>receipts++,JsonDetails:()=>'',drawer:()=>{},esc:String,
  newWardrobeItemDraft:()=>({}),wardrobeCategoryOptions:()=>[],wardrobeSlotOptions:()=>[],window,
 },'requestInventoryAction');
 request('original','remove',{slots:['outer']},2,()=>{},()=>alive);
 window.location.hash='#/c/other/life/wardrobe';await command.onSaved({request:{id:'accepted'}});assert.equal(receipts,0);
 window.location.hash='#/c/original/life/wardrobe';alive=false;await command.onSaved({request:{id:'accepted'}});assert.equal(receipts,0);
 alive=true;await command.onSaved({request:{id:'accepted'}});assert.equal(receipts,1);
});
