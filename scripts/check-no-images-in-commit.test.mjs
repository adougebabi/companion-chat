import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,writeFile,unlink,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {fileURLToPath} from 'node:url';
import {spawnSync} from 'node:child_process';

const guard=fileURLToPath(new URL('./check-no-images-in-commit.mjs',import.meta.url));
async function repository(run){
  const cwd=await mkdtemp(join(tmpdir(),'fluctlight-image-guard-'));
  const env={...process.env};
  for(const key of ['GIT_DIR','GIT_WORK_TREE','GIT_INDEX_FILE','GIT_COMMON_DIR'])delete env[key];
  const git=(...args)=>{const result=spawnSync('git',args,{cwd,env,encoding:'utf8'});assert.equal(result.status,0,result.stderr);};
  const inspect=()=>spawnSync(process.execPath,[guard],{cwd,env,encoding:'utf8'});
  try{git('init','-q');await run({cwd,git,inspect});}finally{await rm(cwd,{recursive:true,force:true});}
}
test('text-only changes are allowed',async()=>repository(async({cwd,git,inspect})=>{
  await writeFile(join(cwd,'report.md'),'verification results');git('add','report.md');assert.equal(inspect().status,0);
}));
test('new images are rejected regardless of case or filename whitespace',async()=>repository(async({cwd,git,inspect})=>{
  await writeFile(join(cwd,'screen shot\nlatest.PNG'),'image');git('add','screen shot\nlatest.PNG');const result=inspect();assert.equal(result.status,1);assert.match(result.stderr,/禁止提交图片/);assert.match(result.stderr,/latest\.PNG/);
}));
test('tracked image modifications are rejected and deletions are allowed',async()=>repository(async({cwd,git,inspect})=>{
  await writeFile(join(cwd,'existing.svg'),'<svg/>');git('add','existing.svg');git('-c','user.name=Image Guard Test','-c','user.email=image-guard@example.invalid','commit','-qm','fixture');
  await writeFile(join(cwd,'existing.svg'),'<svg>changed</svg>');git('add','existing.svg');assert.equal(inspect().status,1);
  await unlink(join(cwd,'existing.svg'));git('add','-u');assert.equal(inspect().status,0);
}));
test('renaming a tracked file to an image is rejected',async()=>repository(async({cwd,git,inspect})=>{
  await writeFile(join(cwd,'notes.txt'),'unchanged content');git('add','notes.txt');git('-c','user.name=Image Guard Test','-c','user.email=image-guard@example.invalid','commit','-qm','fixture');git('mv','notes.txt','reference.SvG');assert.equal(inspect().status,1);
}));
