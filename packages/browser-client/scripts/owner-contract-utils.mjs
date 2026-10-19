export function ownerPathContracts(paths){
 const text={type:'string'},data={type:'object',additionalProperties:true};
 const errorSchema={type:'object',additionalProperties:false,properties:{code:text,message:text,details:data},required:['code','message']};
 for(const [path,methods]of Object.entries(paths))for(const operation of Object.values(methods)){
  const parameters=[...path.matchAll(/\{([^}]+)\}/g)].map(match=>({name:match[1],in:'path',required:true,schema:{type:'string',minLength:1,maxLength:256}}));
  operation.parameters=[...parameters,...operation.parameters||[]];
  for(const status of [400,401,403,404,409,422,500,502,503])operation.responses[status]={description:status===409?'Version or idempotency conflict; preserve the draft':'Bounded Owner operation error',content:{'application/json':{schema:errorSchema}}};
 }
 return paths;
}
