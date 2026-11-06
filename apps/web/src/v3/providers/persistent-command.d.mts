export function persistentCommand<T extends object,R>(options:{key:string;storage:()=>Storage;preAdmissionConflicts?:string[];execute:(value:T,idempotencyKey:string)=>Promise<R>}):{
 pending():{version:1;idempotencyKey:string;value:T;attempts?:number;rejection?:{status:409;code:string}}|null;
 releaseRejected():void;
 submit(value:T):Promise<R>;
 retry():Promise<R>;
};
