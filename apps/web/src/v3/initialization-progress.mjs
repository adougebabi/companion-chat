async function readPart(read){try{return {state:'ready',value:await read()};}catch(error){return {state:'failed',message:error?.message||'读取失败'};}}
// Each optional source retains its own failure. No page read starts a task.
export async function readInitializationProgress(client,fid){
 const [identity,avatar]=await Promise.all([
  readPart(async()=>{const value=await client.visualIdentityManagement(fid);if(value.fluctlight_id!==fid)throw new Error('视觉进度的角色范围不匹配');return value;}),
  readPart(async()=>{const value=await client.displayAvatar(fid);if(value.fluctlight_id!==fid)throw new Error('头像的角色范围不匹配');return value;})
 ]);
 const session=identity.state==='ready'?identity.value.session:null;
 const [timeline,task]=await Promise.all([
  session?readPart(()=>client.visualIdentityTimeline(fid)):Promise.resolve({state:'not_requested'}),
  session?.task_id?readPart(async()=>{const value=await client.diagnosticTask(session.task_id);if(value.fluctlight_id&&value.fluctlight_id!==fid)throw new Error('初始化任务的角色范围不匹配');return value;}):Promise.resolve({state:'not_requested'})
 ]);
 return {fid,identity,avatar,timeline,task};
}
export const initializationRunning=value=>value?.identity?.state==='ready'&&['queued','running','character_sheet_pending'].includes(value.identity.value.session?.status);
