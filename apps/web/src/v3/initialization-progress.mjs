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

// The creation wizard reads four independent authorities. It never starts work.
export async function readCreationProgress(client,fid,{localDate,worldId}={}){
 const [visual,detail,lifecycle,world]=await Promise.all([
  readInitializationProgress(client,fid),readPart(()=>client.detail(fid)),
  readPart(()=>client.lifecycleDiagnostics({fluctlightId:fid,limit:100})),readPart(()=>client.world(fid))
 ]);
 const identity=visual.identity.state==='ready'?visual.identity.value:null;
 const sheet=identity?.character_sheet_asset_id?await readPart(()=>client.ownerMediaAsset(identity.character_sheet_asset_id,fid)): {state:'not_requested'};
 const avatar=visual.avatar.state==='ready'?visual.avatar.value:null;
 let schedule;
 if(localDate)schedule=await readPart(()=>client.schedules(fid,{from:localDate,to:localDate}));
 else schedule={state:'not_requested'};
 const accepted=schedule.state==='ready'?schedule.value.days.find(day=>day.local_date===localDate)?.schedule:null;
 const oldSchedule=detail.state==='ready'&&detail.value.schedule?.local_date===localDate?detail.value.schedule:null;
 const scheduleTask=lifecycle.state==='ready'?lifecycle.value.workflowIntents.find(task=>task.intentType==='schedule.current_day'):null;
 const failed=state=>['failed','dead_letter','cancelled','deadletter'].includes(state);
 const stages=[
  {key:'analysis',title:'分析与整理设定',state:'ready',note:'本次创建资料已提交并确认'},
  {key:'sheet',title:'初始化设定图',state:sheet.state==='ready'&&sheet.value.readable?'ready':sheet.state==='failed'||visual.identity.state==='failed'?'unknown':failed(identity?.session?.status)?'failed':'pending',note:identity?.session?.status==='awaiting_review'?'候选等待明确评审':sheet.state==='failed'?sheet.message:'按实际视觉任务继续准备',href:`#/c/${encodeURIComponent(fid)}/media/identity`},
  {key:'avatar',title:'初始化头像',state:avatar?.avatar?.url?'ready':visual.avatar.state==='failed'?'unknown':'pending',note:visual.avatar.state==='failed'?visual.avatar.message:avatar?.avatar?.url?'已独立读回可显示头像':'尚未读到可显示头像',href:`#/c/${encodeURIComponent(fid)}/media/avatar`},
  {key:'schedule',title:'初始化日程',state:accepted||oldSchedule?'ready':failed(scheduleTask?.status)?'failed':schedule.state==='failed'?'unknown':'pending',note:accepted||oldSchedule?'已读回这一天的已接受日程':scheduleTask?'原任务：'+scheduleTask.status:schedule.state==='failed'?schedule.message:'等待已登记的日程任务',href:scheduleTask?`#/diag/task/${encodeURIComponent(scheduleTask.intentId)}`:`#/c/${encodeURIComponent(fid)}/life/schedule`}
 ];
 const worldConfirmed=world.state==='ready'&&worldId&&world.value.world?.id===worldId;
 return {fid,stages,worldConfirmed,worldError:world.state==='failed'?world.message:'',complete:stages.every(stage=>stage.state==='ready')&&!!worldConfirmed,visual,schedule,world};
}
