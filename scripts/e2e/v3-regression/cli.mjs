import {RegressionError} from './runtime.mjs';
export function runnerArguments(args){
 const at=args.indexOf('--case');if(at>=0&&(!args[at+1]||args[at+1].startsWith('--')))throw new RegressionError('ARGUMENT_INVALID','--case requires L01 through L17');
 return {id:at>=0?args[at+1].toUpperCase():null,forwarded:args.filter((value,index)=>value!=='--all'&&(at<0||index!==at&&index!==at+1))};
}
export function caseOutcome(item,result,report,invocationId){
 const expected={0:report?.executionMode==='preflight'?'preflight_passed':'passed',1:'failed',2:'blocked',3:'review_required'},fresh=report?.invocationId===invocationId&&report.case===item.id&&!!report.finishedAt;
 if(!fresh||!(result.code in expected)||report.status!==expected[result.code])return {id:item.id,title:item.title,status:'harness_failed',exitCode:1,actualExitCode:result.code,error:result.error||'Child exited unexpectedly or did not produce a fresh matching final report',report:null};
 return {id:item.id,title:item.title,status:report.status,exitCode:result.code,failedSteps:report.steps.filter(s=>s.status==='failed'||s.status==='blocked').map(s=>({id:s.id,status:s.status,code:s.error?.code,message:s.error?.message})),report:item.id+'.json'};
}
