import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {after} from 'node:test';
const vite=await createServer({root:fileURLToPath(new URL('../../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const {createCreationController}=await vite.ssrLoadModule('/src/state/creation-controller.mjs');
after(()=>vite.close());
import {memoryStorage} from './runtime.mjs';
export const validAnalysis=()=>({analysis_id:'initialization_source_'+'a'.repeat(32),correlation_id:'controlled-analysis',schema_version:2,core_persona:{identity:{name:'',timezone:'Asia/Shanghai'},personality:{},behavioral_policy:{},life_profile:{},personality_system:{mode:'single',profiles:[]}},developing_self:{},extensions:{},initial_goals:[],initial_intentions:[],initial_relationships:[]});
export function creationFixture(analyze=async()=>validAnalysis()){
 const saved=memoryStorage(),calls=[];let world=null;
 const client={world:async()=>({world}),updateRuntimeWorld:async()=>{world={id:'world-created',name:'日常世界'};return world;}};
 const directory={analyze,activate:async body=>{calls.push(body);return {id:'created'};},move:async()=>{}};
 const c=createCreationController({owner:'owner',storage:saved,client,directory,ownerCurrent:()=>true,label:k=>k,analysisToActivation:(a,name,requestId,timezone)=>({requestId,name,corePersona:a.core_persona,initializationTimezone:timezone})});
 return {c,saved,calls,client,directory};
}
