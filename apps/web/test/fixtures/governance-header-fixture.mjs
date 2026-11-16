import {proxy, snapshot, subscribe} from 'valtio/vanilla';
export function headerWorkspace() {
 const calls=[];
 const read=(name,value)=>async()=>{calls.push(name);return value;};
 const empty={items:[],next_cursor:''};
 const state=proxy({session:{actorId:'header-owner'},characters:[{id:'A',name:'林青',status:'active'}],selectedId:'A'});
 return {state,calls,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),currentScope:()=> 'header-scope',
  services:{governanceClient:{current:read('current',{revision:1,identity:{name:'林青'}}),list:read('governance-list',empty),history:read('history',empty)},
   selfClient:{summary:read('summary',{personality:{description:'温和而认真'},foundation_revisions:[]}),list:read('self-list',empty),reflections:read('reflections',empty),expressionLearning:read('expression',{settings:{revision:1,learning_enabled:false,use_enabled:false},items:[]}),expressionSources:read('sources',empty)}},
  navigatePath:path=>calls.push(path)};
}
