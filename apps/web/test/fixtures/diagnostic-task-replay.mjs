// Read-only replay of actual Owner HTTP responses from the isolated Core test.
// No synthesized lifecycle/model results, upstream calls or mutation handlers.
export function diagnosticTaskReplay(evidence){
 if(evidence.source!=='isolated_real_core_temporal_controlled_http'||!Array.isArray(evidence.cases))throw Error('Actual isolated diagnostic evidence required');
 const reads=[];
 return {reads,handler({path,method,url,json,res}){
  if((path.startsWith('/api/')||path.startsWith('/auth/'))&&!['GET','HEAD'].includes(method)){json(res,{code:'fixture_read_only'},405);return true;}
  if(method!=='GET')return false;
  const send=value=>{reads.push({path,query:Object.fromEntries(url.searchParams)});json(res,value);return true;};
  if(path==='/fixture/diagnostic-evidence')return send({source:evidence.source,provider_requests:evidence.provider_requests,renderer_requests:evidence.renderer_requests,tasks:evidence.cases.map(c=>({id:c.task.task_id,workflow:c.task.workflow_id,status:c.task.status,correlation:c.task.correlation_id,model_records:c.models.items.length})),reads});
  if(path==='/auth/session')return send({authenticated:true,actorId:evidence.owner});
  if(path==='/api/fluctlights')return send([{id:evidence.fluctlight_id,status:'active',identity:{name:'隔离媒体诊断角色',timezone:'Asia/Shanghai'},unread_count:0}]);
  if(path==='/api/actor-groups')return send([]);
  if(path==='/api/pending')return send(evidence.pending);
  if(path==='/api/pending/count')return send(evidence.pending_count);
  if(path.startsWith('/api/diagnostics/tasks/')){
   const id=decodeURIComponent(path.slice('/api/diagnostics/tasks/'.length)),entry=evidence.cases.find(c=>[c.task.task_id,c.task.workflow_id,'go:'+c.task.workflow_id].includes(id));
   return entry?send(entry.task):(json(res,{code:'task_not_found'},404),true);
  }
  if(path.startsWith('/api/workflow-controls/')){
   const id=decodeURIComponent(path.slice('/api/workflow-controls/'.length)),entry=evidence.cases.find(c=>[c.task.workflow_id,'go:'+c.task.workflow_id].includes(id));
   return entry?send(entry.workflow):(json(res,{code:'not_replayed'},404),true);
  }
  if(path==='/api/diagnostic-records/workflow-intents')return send(evidence.workflows);
  if(path.startsWith('/api/diagnostic-records/')){
   const [kind,ref]=path.slice('/api/diagnostic-records/'.length).split('/'),correlation=url.searchParams.get('correlationId');
   const entries=evidence.cases.filter(c=>!correlation||c.task.correlation_id===correlation);
   const key=kind==='model-runs'?'models':kind==='media-prompts'?'media':null;
   const items=key?entries.flatMap(c=>c[key].items):[];
   if(ref){const record=items.find(row=>row.ref===decodeURIComponent(ref));return record?send(record):(json(res,{code:'not_found'},404),true);}
   return send({items,next_cursor:''});
  }
  return false;
 }};
}
