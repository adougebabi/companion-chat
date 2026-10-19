import {spawn} from 'node:child_process';
import {resolve,join} from 'node:path';
import {mkdir,writeFile} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
import {randomUUID} from 'node:crypto';
import {cases} from './cases.mjs';
import {readJSON,redact,repoRoot} from './runtime.mjs';
import {runnerArguments,caseOutcome} from './cli.mjs';

const args=process.argv.slice(2);
if(args.includes('--list')){
 for(const item of cases)console.log(`${item.id}\t${item.title}\t${item.file}`);
}else if(args.includes('--self-test')){
 const child=spawn(process.execPath,['--test',fileURLToPath(new URL('runtime.test.mjs',import.meta.url))],{cwd:repoRoot,stdio:'inherit'});child.on('error',error=>{console.error(error.message);process.exitCode=1;});child.on('exit',code=>{process.exitCode=code??1;});
}else{
 const parsed=runnerArguments(args),id=parsed.id;
 const targets=id?cases.filter(c=>c.id===id):args.includes('--all')?cases:[];
 if(!targets.length){console.error('Usage: node scripts/e2e/v3-regression/run.mjs --list | --self-test | --case L01 | --all [--live] [--run-dir <private-folder>]');process.exitCode=1;}
 else{
  const forwarded=parsed.forwarded,at=forwarded.indexOf('--run-dir');
  const runDir=resolve(at>=0?forwarded[at+1]:process.env.FLUCTLIGHT_E2E_RUN_DIR||join(repoRoot,'.v3-regression',new Date().toISOString().replace(/[:.]/g,'-')));
  if(at<0)forwarded.push('--run-dir',runDir);
  await mkdir(runDir,{recursive:true,mode:0o700});
  const summary={schemaVersion:1,startedAt:new Date().toISOString(),executionMode:forwarded.includes('--preflight')?'preflight':'scenario',runDir,liveRequested:forwarded.includes('--live'),sequential:true,fullDeliveryAccepted:false,cases:[]};
  for(const item of targets){
   console.log(`\n--- ${item.id}: ${item.title} ---`);
   const invocationId=randomUUID();
   const result=await new Promise(resolvePromise=>{const child=spawn(process.execPath,[fileURLToPath(new URL(item.file,import.meta.url)),...forwarded],{cwd:repoRoot,env:{...process.env,FLUCTLIGHT_E2E_INVOCATION_ID:invocationId},stdio:'inherit'});child.on('error',error=>resolvePromise({code:1,error:error.message}));child.on('exit',(code,signal)=>resolvePromise({code:code??1,signal}));});
   const report=await readJSON(join(runDir,item.id+'.json'),null);
   summary.cases.push(caseOutcome(item,result,report,invocationId));
   await writeFile(join(runDir,'summary.json'),JSON.stringify(redact(summary),null,2)+'\n',{mode:0o600});
  }
  summary.finishedAt=new Date().toISOString();summary.counts=Object.fromEntries(['passed','preflight_passed','failed','blocked','review_required','harness_failed'].map(status=>[status,summary.cases.filter(c=>c.status===status).length]));
  await writeFile(join(runDir,'summary.json'),JSON.stringify(redact(summary),null,2)+'\n',{mode:0o600});
  console.log(`\nReports: ${runDir}\n${JSON.stringify(summary.counts)}`);
  process.exitCode=summary.cases.some(c=>c.exitCode===1)?1:summary.cases.some(c=>c.exitCode===2)?2:summary.cases.some(c=>c.exitCode===3)?3:0;
 }
}
