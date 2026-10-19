// This stores a submitted command identity and its immutable input, never a
// business result. Unknown transport outcomes must replay that exact command
// before the UI can start another proposal, including after a page refresh.
export function persistentCommand({key,storage,execute,preAdmissionConflicts=['revision_conflict'],retainedValidationCodes=[]}){
 let running=false;
 function owns(record,exactAttempt=false){
  const raw=storage().getItem(key);if(!raw)return false;
  try{const current=JSON.parse(raw);return current.idempotencyKey===record.idempotencyKey&&(!exactAttempt||current.attempts===record.attempts);}catch{return false;}
 }
 function pending(){
  const raw=storage().getItem(key);if(!raw)return null;
  let record;try{record=JSON.parse(raw);}catch{throw new Error('原提交记录损坏，请先从请求记录核对结果；当前没有发起新提交。');}
  if(record?.version!==1||typeof record.idempotencyKey!=='string'||!record.idempotencyKey||!record.value||typeof record.value!=='object'||Array.isArray(record.value))throw new Error('原提交记录不可读取，请先核对原请求结果。');
  return structuredClone(record);
 }
 async function run(record){
  if(running)throw new Error('原提交仍在处理中，请等待实际结果');
  if(record.rejection)throw new Error('原提交已明确因版本冲突被拒绝，请先核对最新版本并明确重设基线。');
  running=true;
  try{
   record.attempts=(record.attempts||0)+1;delete record.rejection;storage().setItem(key,JSON.stringify(record));
   const result=await execute(structuredClone(record.value),record.idempotencyKey);
   if(owns(record))storage().removeItem(key);return result;
  }catch(error){
   // Validation rejection is known to precede domain admission. Network/5xx,
   // authentication loss and conflict preserve the original recovery identity.
   if(record.attempts===1&&[400,422].includes(error?.status)&&!retainedValidationCodes.includes(error?.code)&&owns(record,true))storage().removeItem(key);
   // These CAS errors precede the atomic admission transaction. A conflict
   // after an earlier unknown attempt cannot prove the original had no effect.
   if(record.attempts===1&&error?.status===409&&preAdmissionConflicts.includes(error?.code)&&owns(record,true)){
    record.rejection={status:409,code:error.code};storage().setItem(key,JSON.stringify(record));
   }
   throw error;
  }finally{running=false;}
 }
 return {
  pending,
  releaseRejected(){const record=pending();if(running||!record?.rejection||record.attempts!==1)throw new Error('原提交尚未确认无效果，不能更换其内容或幂等身份。请先重试原提交。');storage().removeItem(key);},
  async submit(value){
   const previous=pending();
   if(previous){if(JSON.stringify(previous.value)!==JSON.stringify(value))throw new Error('原提议的受理结果尚未确认。新草稿已保留，请先查询或重试原提交。');return run(previous);}
   const record={version:1,idempotencyKey:crypto.randomUUID(),attempts:0,value:structuredClone(value)};
   storage().setItem(key,JSON.stringify(record));return run(record);
  },
  async retry(){const record=pending();if(!record)throw new Error('没有等待核对的原提交，请重新读取请求列表。');return run(record);},
 };
}
