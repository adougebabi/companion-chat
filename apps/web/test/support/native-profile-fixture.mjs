import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {installDOM,mountReact,act} from '../helpers/react-dom.mjs';
export function profileDetail({id='fixture-role',status='active',active='warm',revision=2,name='角色'}={}){return {id,status,current_revision:4,identity:{name,background:'真实背景',timezone:'Asia/Shanghai'},core_persona:{identity:{name,timezone:'Asia/Shanghai'},personality:{},behavioral_policy:{},life_profile:{},personality_system:{profiles:[{id:'warm',name:'日常人格'},{id:'focused',name:'专注人格'}]}},personality_runtime:{active_profile_id:active,revision},current_state:{appearance:null},developing_self:{claims:[]}};}
export function personaReceipt(fid,body,{status='completed',active=body.targetPersonaId,runtimeRevision=body.expectedRuntimeRevision+1,disposition='applied',requestId='persona-fixture-request'}={}){const value={request_id:requestId,status,disposition,revision:body.expectedRevision,runtime_revision:runtimeRevision,active_profile_id:active,previous_profile_id:'warm',audit_id:'audit-fixture',persona_action_audit_id:null};return {...value,request:{id:requestId,kind:'persona_switch',fluctlight_id:fid,status,revision:1,payload:{target_persona_id:body.targetPersonaId,reason:body.reason,expected_revision:body.expectedRevision,expected_runtime_revision:body.expectedRuntimeRevision},result:value,result_refs:[],created_at:'2026-10-27T00:00:00Z'}};}
export async function nativeProfileFixture({id='fixture-role',read=async()=>profileDetail({id}),switchPersona=async(fid,body)=>personaReceipt(fid,body)}={}){
 const environment=installDOM();location.hash='#/c/'+id+'/profile';
 const vite=await createServer({root:fileURLToPath(new URL('../../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
 const services=await vite.ssrLoadModule('/src/v3/proto/services.js');
 const {createWorkspace}=await vite.ssrLoadModule('/src/state/workspace.mjs'),store=await vite.ssrLoadModule('/src/v3/proto/store.js');
 const {createProfileModel}=await vite.ssrLoadModule('/src/react-app/pages/CharacterDetailPage.jsx'),{cell}=await vite.ssrLoadModule('/src/state/view-model.ts');
 const w=createWorkspace();store.replaceSnapshot({...store.get(),session:{actorId:'owner'},characters:{[id]:{id,name:'角色',status:'正常'}},ui:{charId:id}});
 let lastRequest;w.client.detail=read;w.client.switchPersona=async(...args)=>{const result=await switchPersona(...args);lastRequest=result.request;return result;};services.configureDomainService("life",{request:async()=>structuredClone(lastRequest)});w.client.world=async()=>({world:null});
 const disposers=[];
 const c=createProfileModel({workspace:w,cell,observe(){},onStart(){},onDispose:fn=>disposers.push(fn)});
 return {c,w,read:()=>c.read(),draft(){c.target.value='focused';c.reason.value='专注完成当前工作';c.confirm.value=true;},async html(){
  const view=await mountReact(vite,()=>c.render(),{workspace:w});
  try{const persona=[...view.host.querySelectorAll('[role=tab]')].find(node=>node.textContent==='性格与人格');if(persona){await act(async()=>persona.dispatchEvent(new window.MouseEvent('mousedown',{bubbles:true,button:0,ctrlKey:false})));await view.flush();}const switcher=[...view.host.querySelectorAll('button')].find(node=>node.textContent.trim()==='切换人格');if(switcher&&!switcher.disabled)await view.click(switcher);for(const trigger of view.host.querySelectorAll('[data-slot="collapsible-trigger"][aria-expanded="false"]'))await view.click(trigger);return view.host.innerHTML+[...document.querySelectorAll('[role="dialog"]')].map(node=>node.outerHTML).join('');}finally{await view.dispose();}
 },async dispose(){for(const fn of disposers)fn();w.dispose();await vite.close();await new Promise(r=>setTimeout(r,10));environment.restore();}};
}
