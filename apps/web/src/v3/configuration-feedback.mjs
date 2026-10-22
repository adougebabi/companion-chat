// Test receipts report actual call outcomes independently of saved configuration.
export function configurationTestSummary(value) {
 const failures=[],successes=[];
 const visit=(node,path='',depth=0)=>{
  if(!node||typeof node!=='object'||depth>8)return;
  for(const [key,item] of Object.entries(node)){
   if(['error','last_error','last_error_code','error_code','failure_reason'].includes(key)&&item)failures.push({path:[path,key].filter(Boolean).join('.'),value:typeof item==='string'?item:JSON.stringify(item)});
   if(['state','status','call_status','connection_status'].includes(key)&&typeof item==='string'){
    if(['failed','error','unavailable','timeout','request_failed','rejected','unconfigured','disabled','not_configured'].includes(item))failures.push({path:[path,key].filter(Boolean).join('.'),value:item});
    if(['ok','success','succeeded','available','passed','reachable'].includes(item))successes.push(item);
   }
   if((key==='ok'||key==='success'||key==='connected')&&item===false)failures.push({path:[path,key].filter(Boolean).join('.'),value:'false'});
   if((key==='ok'||key==='success'||key==='connected'||key==='image_input_verified')&&item===true)successes.push(key);
   if(item&&typeof item==='object')visit(item,[path,key].filter(Boolean).join('.'),depth+1);
  }
 };
 visit(value);
 return failures.length?{state:'failed',text:'测试未通过，请检查连接、凭据与服务状态。',failures}:successes.length?{state:'success',text:'测试已通过，已保存连接返回成功回执。实际生成与业务结果仍以各自任务为准。',failures:[]}:{state:'unknown',text:'已收到测试回执，但没有明确的成功结论。请展开实际结果核对。',failures:[]};
}

export function beijingFilterInput(value) {
 if(!value)return '';
 const date=new Date(value);
 if(!Number.isFinite(date.getTime()))return value;
 return new Date(date.getTime()+8*3600000).toISOString().slice(0,16).replace('T',' ');
}
export function diagnosticTimeFilter(value) {
 if(!value?.trim())return '';
 const match=/^(\d{4})-(\d{2})-(\d{2})[ T](\d{2}):(\d{2})(?::(\d{2}))?$/.exec(value.trim());
 if(!match)throw new Error('请按 YYYY-MM-DD HH:mm 填写北京时间，例如 2026-10-22 10:00。');
 const [,y,m,d,h,min,s='00']=match,result=`${y}-${m}-${d}T${h}:${min}:${s}+08:00`,date=new Date(result);
 if(!Number.isFinite(date.getTime())||beijingFilterInput(result)!==`${y}-${m}-${d} ${h}:${min}`||Number(s)>59)throw new Error('日期或时间无效，请检查年月日、小时和分钟。');
 return result;
}
