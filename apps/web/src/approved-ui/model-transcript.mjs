const roles=new Set(['system','developer','user','assistant','tool','function']);
function messages(value){let parsed=value;if(typeof value==='string'){try{parsed=JSON.parse(value);}catch{return null;}}const rows=Array.isArray(parsed)?parsed:Array.isArray(parsed?.messages)?parsed.messages:null;return rows&&rows.every(row=>row&&roles.has(row.role))?rows:null;}
function failed(message){if(message?.is_error===true||['failed','error','rejected'].includes(message?.status))return true;if(message?.role==='tool'&&typeof message.content==='string'){try{const value=JSON.parse(message.content);return ['failed','error','rejected','unavailable'].includes(value?.status)||value?.is_error===true;}catch{}}return false;}
export function modelTranscriptGroups(record){
 const data=record?._record?.data||{},input=messages(data.prompt),groups=[];
 if(input){let group={id:'input',title:'输入与历史',messages:[],failed:false},round=0;for(const message of input){if(message.role==='assistant'&&message.tool_calls?.length){if(group.messages.length)groups.push(group);group={id:'tool-'+(++round),title:'工具请求组 '+round,messages:[],failed:false};}group.messages.push(message);group.failed ||= failed(message);}if(group.messages.length)groups.push(group);}
 else groups.push({id:'input',title:'已记录的请求内容',messages:[{role:'user',content:record?.input||''}],failed:false});
 const output=data.response??record?.response,transport=output&&typeof output==='object'&&Array.isArray(output.choices)?output.choices.map(choice=>choice.message).filter(Boolean):[{role:'assistant',content:typeof output==='string'?output:JSON.stringify(output??'',null,2)}];
 groups.push({id:'response',title:'本次调用响应',messages:transport,failed:record?.status==='error'||transport.some(failed)});
 return groups;
}
