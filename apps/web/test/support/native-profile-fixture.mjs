import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {installDOM,mountReact} from '../helpers/react-dom.mjs';
export function profileDetail({id='fixture-role',status='active',active='warm',revision=2,name='角色'}={}){return {id,status,current_revision:4,identity:{name,background:'真实背景',timezone:'Asia/Shanghai'},core_persona:{identity:{name,timezone:'Asia/Shanghai'},personality:{},behavioral_policy:{},life_profile:{},personality_system:{profiles:[{id:'warm',name:'日常人格'},{id:'focused',name:'专注人格'}]}},personality_runtime:{active_profile_id:active,revision},current_state:{appearance:null},developing_self:{claims:[]}};}
export async function nativeProfileFixture({id='fixture-role',read=async()=>profileDetail({id}),switchPersona=async()=>({status:'completed'})}={}){
 const environment=installDOM();location.hash='#/c/'+id+'/profile';
 const vite=await createServer({root:fileURLToPath(new URL('../../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
 const {createWorkspace}=await vite.ssrLoadModule('/src/state/workspace.mjs'),store=await vite.ssrLoadModule('/src/v3/proto/store.js');
 const {createProfileModel}=await vite.ssrLoadModule('/src/react-app/pages/CharacterDetailPage.jsx'),{cell}=await vite.ssrLoadModule('/src/state/view-model.ts');
 const w=createWorkspace();store.replaceSnapshot({...store.get(),session:{actorId:'owner'},characters:{[id]:{id,name:'角色',status:'正常'}},ui:{charId:id}});
 w.client.detail=read;w.client.switchPersona=switchPersona;w.client.world=async()=>({world:null});
 const disposers=[];
 const c=createProfileModel({workspace:w,cell,onStart(){},onDispose:fn=>disposers.push(fn)});
 return {c,w,read:()=>c.read(),draft(){c.target.value='focused';c.reason.value='专注完成当前工作';c.confirm.value=true;},async html(){
  const view=await mountReact(vite,()=>c.render(),{workspace:w});
  try{for(const trigger of view.host.querySelectorAll('[data-slot="collapsible-trigger"][aria-expanded="false"]'))await view.click(trigger);return view.host.innerHTML;}finally{await view.dispose();}
 },async dispose(){for(const fn of disposers)fn();w.dispose();await vite.close();environment.restore();}};
}
