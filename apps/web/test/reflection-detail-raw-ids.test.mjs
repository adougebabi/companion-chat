import assert from 'node:assert/strict';
import test from 'node:test';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {installDOM,mountReact} from './helpers/react-dom.mjs';
const env=installDOM();
import {createServer} from './support/module-server.mjs';

const vite=await createServer({
  root:fileURLToPath(new URL('../',import.meta.url)),
  envFile:false,
  appType:'custom',
  logLevel:'silent',
  server:{middlewareMode:true,ws:false},
});
test.after(async()=>{await vite.close();env.restore();});
const {featureRoute}=await vite.ssrLoadModule('/src/approved-ui/feature-routes.mjs');
const {default:DomainPage}=await vite.ssrLoadModule('/src/react-app/pages/DomainPage.tsx');

const ref='memory:ctx_hidden';
const at='2026-11-20T02:15:00Z';
const reflection={
  id:'reflection_proposal_hidden',fluctlight_id:'fluctlight_hidden',title:'反思窗口 317—329',
  status:'applied',at,from_sequence:317,to_sequence:329,base_state_revision:5791,
  summary:'她发现交流时先确认对方的想法更有帮助。',body:'随后留意自己急于给建议的习惯。',
  schema_version:'fluctlight.reflection-proposal.v2',policy_version:'reflection.policy.v2',
  source_refs:['sequence:317'],idempotency_key:'private-replay',correlation_id:'private-trace',
  source_window_status:'available',
  source_window:{
    source_window:'sequence:317-329',
    evidence:[
      {kind:'conversation.turn',occurred_at:'2026-11-19T22:00:00Z',ref:'sequence:317',summary:'{"source_ref":"res_hidden"}'},
      {kind:'life.owner_request.submitted',occurred_at:'2026-11-20T01:30:00Z',ref:'sequence:329'},
      {kind:'private.event_key'},
    ],
    outcomes:[{id:'outcome_hidden',revision:6791}],
  },
  references:[{ref,kind:'memory',entity_id:'memory_hidden',revision:8791}],
  dispositions:[
    {domain:'memory',candidate_index:0,candidate_id:'evolution_candidate_hidden',disposition:'accepted',changed_ref:ref,reason_code:'policy_accepted'},
    {domain:'affect_profile',candidate_index:1,disposition:'deferred',reason_code:'cross_window_evidence_required'},
  ],
  proposal:{
    schema_version:'fluctlight.reflection-proposal.v2',
    memory_candidates:[{operation:'revise',content:'记下她偏好「先听完」<&\'"。',semantic_reason:'她愿意先倾听。',target_ref:ref}],
    goal_candidates:[{operation:'create',desired_outcome:'更耐心地倾听',success_criteria:['先确认对方的意思','req_hidden','']}],
    emotional_summary:{dominant_patterns:['被理解时更放松']},
    affect_recalibration_candidates:[{target:'baseline.pleasure',direction:'increase'}],
  },
  result:{status:'applied',revisions:{memory:9791},metrics:{internal_count:6791},future_field:{sample:[false,0,null,"<&'\"中文"]}},
};

async function mountReflection(t,record){
 const fid='fluctlight_hidden',query=new URLSearchParams(),path='/c/'+fid+'/growth/reflection/'+record.id;
 const feature=featureRoute(path,query);assert.equal(feature.native,'reflection');
 const state=proxy({session:{actorId:'owner'},selectedId:fid,characters:[{id:fid,status:'active'}]});
 const reads=[];
 const workspace={state,current:{get value(){return state.characters[0];}},subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),isCurrent:(owner,id)=>owner==='owner'&&id===fid,navigatePath(){},client:{
  managedMemories:async()=>({items:[]}),listReflections:async()=>({items:[]}),selfClaims:async()=>({items:[]}),
  reflection:async(...args)=>{reads.push(args);return record;},reflectionFeedback:async()=>({items:[]}),
 }};
 const view=await mountReact(vite,DomainPage,{workspace,props:{route:{fid,path,query,feature}}});
 t.after(()=>view.dispose());await view.flush();
 assert.deepEqual(reads,[[fid,record.id]]);
 const host=document.querySelector('[role=dialog]');
 const changes=[...host.querySelectorAll('button')].find(n=>n.textContent==='提议与实际改变');await view.click(changes);
 return {host,view};
}
async function checkDownload(t,view,host,record){
 const button=text=>[...host.querySelectorAll('button')].find(n=>n.textContent===text);
 await view.click(button('原始反思记录（诊断）'));
 const blobs=[],links=[];
 const create=t.mock.method(URL,'createObjectURL',blob=>{blobs.push(blob);return 'blob:reflection-fixture';});
 const click=t.mock.method(window.HTMLAnchorElement.prototype,'click',function(){links.push(this.download);});
 try{await view.click(button('下载原始记录'));assert.deepEqual(links,['fluctlight-record.json']);assert.deepEqual(JSON.parse(await blobs[0].text()),record);}
 finally{create.mock.restore();click.mock.restore();}
 assert.equal(host.querySelector('pre,code'),null);
}

const hidden=/reflection-proposal|\.v2\b|reflection_proposal_hidden|fluctlight_hidden|res_hidden|evolution_candidate_hidden|req_hidden|memory_hidden|ctx_hidden|outcome_hidden|private-(?:replay|trace)|private\.event_key|\b(?:317|329|5791|6791|8791|9791|schema_version|policy_version|candidate_index|revision|revisions|source_refs?|source_window|idempotency_key|metrics|correlation_id|success_criteria|policy_accepted|cross_window_evidence_required)\b|尚未记录|未记录|版本|完整记录与来源|2026-11-20T02:/i;

test('独立反思详情展示中文业务摘要并仅下载完整原始记录',async t=>{
  const {host,view}=await mountReflection(t,reflection);
  const visible=host.textContent;
  assert.doesNotMatch(visible,hidden);
  for(const expected of [
    reflection.summary,reflection.body,reflection.proposal.memory_candidates[0].content,
    '已应用','反思时间','2026-11-20 10:15 (北京时间)','反思来源',
    '对话交流 1 条','用户提交的请求 1 条','其他反思素材 1 条',
    '2026-11-20 06:00 (北京时间) 至 2026-11-20 09:30 (北京时间)','关联行动结果1 项',
    '提议与实际改变','已采纳','提议操作修订','提议原因她愿意先倾听。',
    '期望结果更耐心地倾听','完成标准先确认对方的意思',
    '情绪总结','主要情绪规律被理解时更放松','调整对象愉悦程度','暂缓采纳','还需要其他时期的依据',
  ])assert.ok(visible.includes(expected),expected);
  const memoryLink=host.querySelector('a[href$="/growth/memory/memory_hidden"]');
  assert.equal(memoryLink?.textContent,'查看记忆');
  assert.ok([...host.querySelectorAll('button')].some(n=>n.textContent==='赞同这次观察'));
  assert.ok([...host.querySelectorAll('button')].some(n=>n.textContent==='说说不同看法'));
  await checkDownload(t,view,host,reflection);
});

test('独立反思详情省略空字段并保留旧记录原样下载',async t=>{
  const legacy={
    id:'reflection_proposal_hidden',fluctlight_id:'fluctlight_hidden',title:'reflection_proposal_hidden',state:'no_change',
    body:' ',at:'',created_at:null,source_window:null,source_window_status:'legacy_snapshot_unavailable',
    dispositions:null,references:null,proposal:{schema_version:'fluctlight.reflection-proposal.v2'},
  };
  const {host,view}=await mountReflection(t,legacy);
  assert.equal(host.querySelector('h2').textContent,'反思记录');
  assert.match(host.textContent,/未产生变更/);
  assert.doesNotMatch(host.textContent,hidden);
  assert.doesNotMatch(host.textContent,/时间未知|反思来源|不存在的提议标题|旧记录没有|没有目标引用/);
  assert.equal(host.querySelector('.memory-content, .record-facts'),null);
  await checkDownload(t,view,host,legacy);
});
