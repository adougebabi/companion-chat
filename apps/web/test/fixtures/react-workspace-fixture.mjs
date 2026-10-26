// Canonical read projections for the React production UI. Fixture-only, no upstream calls.
export function reactWorkspaceFixture({settings,endpoints,bindings,schedule,owner,date}) {
  return ({path,method,url,json,res}) => {
    if(method!=='GET')return false;
    const send=value=>{json(res,value);return true;};
    if(path==='/api/configuration-catalog'){
      const current=settings(),hours=current.values['product.autonomy']?.quiet_hours;
      return send({effective_values:{...current.values,'product.autonomy':{...current.values['product.autonomy'],quiet_hours:{start:hours?.start||hours?.from||'',end:hours?.end||hours?.to||'',days:[1,2,3,4,5]}}},endpoints:endpoints(),bindings:bindings(),versions:{...Object.fromEntries(Object.keys(current.values).map(key=>['setting:'+key,1])),...Object.fromEntries(endpoints().map(item=>['endpoint:'+item.id,1])),...Object.fromEntries(bindings().map(item=>['binding:'+item.role,1]))},configured_secrets:[]});
    }
    if(path==='/api/runtime-catalog')return send({agents:[],tools:[]});
    if(path.startsWith('/api/diagnostic-records/'))return send({items:[],next_cursor:''});
    if(path==='/api/notifications')return send({items:[],next_cursor:''});
    if(path==='/api/notifications/unread-count')return send({count:0});
    if(path==='/api/pending')return send({groups:[]});
    if(path==='/api/pending/count')return send({count:0});
    if(path==='/api/studio/resources')return send({items:[],next_cursor:''});
    if(path==='/api/moment-feed')return send({items:[],next_cursor:''});
    if(path==='/api/media-library')return send({items:[],next_cursor:''});
    const role=/^\/api\/fluctlights\/([^/]+)\/(.*)$/.exec(path);
    if(!role)return false;
    const [,_fid,part]=role;
    if(part==='schedules'){const value=schedule();return send({today:value.local_date,timezone:value.timezone,days:[{local_date:value.local_date,schedule:value}]});}
    if(part==='memories')return send({items:[{id:'memory-react-fixture',owner_fluctlight_id:_fid,content:'这是一条隔离验收记忆，用于检查展开、编辑与取消。',created_at:date,type:'semantic',status:'active',importance:.6,tags:['验收样本'],revision:1}],next_cursor:''});
    if(part==='diary')return send({items:[],next_cursor:'',timezone:'Asia/Shanghai'});
    if(part==='promises')return send({items:[],next_cursor:''});
    if(part==='npcs')return send({items:[],next_cursor:''});
    if(part==='world/catalog')return send({items:[],active_binding:null,available:true});
    return false;
  };
}
