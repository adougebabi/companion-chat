<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { BrowserClient, BrowserApiError, type BrowserGoalSet, type BrowserGoalSetCommand } from '@fluctlight/browser-client';
import Button from '@/components/ui/button/Button.vue';
import { randomId } from '../../random-id';
import { apiOrigin } from '../../runtime-config';
const props=defineProps<{fluctlightId:string;readOnly?:boolean}>();
const client=new BrowserClient(apiOrigin);
const state=ref<BrowserGoalSet|null>(null),runs=ref<Record<string,unknown>[]>([]),error=ref(''),busy=ref(false),notice=ref('');
const order=ref<string[]>([]),dependencyGoal=ref(''),prerequisites=ref<string[]>([]);
const sourceReviews=computed(()=>{const values=state.value?.source_reviews;return Array.isArray(values)?values.filter((v):v is Record<string,unknown>=>Boolean(v)&&typeof v==='object'):[];});
const mergeSeconds=ref(2),retrySeconds=ref(60),attemptLimit=ref(5),leaseSeconds=ref(300),candidateLimit=ref(20);
const sourceOutcome=ref(''),sourceCriteria=ref(''),sourceMotivation=ref(''),sourceGoalId=ref('');
function resolveSource(review:Record<string,unknown>,operation:string){return apply({sourceReviews:[{source_id:review.source_id,expected_revision:review.revision,operation,goal_id:sourceGoalId.value,desired_outcome:sourceOutcome.value,success_criteria:sourceCriteria.value.split('\n').filter(Boolean),motivation:sourceMotivation.value}]},'Owner语义复核历史来源，保留原文并记录决定');}
let epoch=0;
let pending:{fingerprint:string;key:string}|null=null;
async function load(){const e=epoch;busy.value=true;try{const [s,r]=await Promise.all([client.goalSet(props.fluctlightId),client.goalPlanningHistory(props.fluctlightId)]);if(e!==epoch)return;state.value=s;mergeSeconds.value=Number(s.merge_window_seconds??2);retrySeconds.value=Number(s.retry_backoff_seconds??60);attemptLimit.value=Number(s.max_attempts??5);leaseSeconds.value=Number(s.lease_seconds??300);candidateLimit.value=Number(s.candidate_limit??20);runs.value=r;order.value=s.goals.filter(g=>g.status==='active').map(g=>g.id);error.value='';}catch(c){if(e===epoch)error.value=String(c);}finally{if(e===epoch)busy.value=false;}}
async function apply(patch:Partial<BrowserGoalSetCommand>,reason:string){if(!state.value||busy.value)return;const e=epoch;busy.value=true;const fingerprint=JSON.stringify(patch);if(pending?.fingerprint!==fingerprint)pending={fingerprint,key:randomId()};try{const value=await client.updateGoalSet(props.fluctlightId,{...patch,expectedVersion:state.value.revision,expectedFactsRevision:state.value.facts_revision,idempotencyKey:pending!.key,reason});if(e!==epoch)return;state.value=value;pending=null;notice.value='已保存到服务端，后续规划与执行使用新版本。';error.value='';}catch(c){if(e!==epoch)return;error.value=c instanceof BrowserApiError&&c.status===409?'版本、容量或依赖发生冲突；保留草稿，请刷新后重新提交。':String(c);}finally{if(e===epoch)busy.value=false;}}
function move(index:number,delta:number){const next=index+delta;if(next<0||next>=order.value.length)return;const copy=[...order.value];[copy[index],copy[next]]=[copy[next]!,copy[index]!];order.value=copy;}
async function plan(){const e=epoch;busy.value=true;try{await client.requestGoalPlanning(props.fluctlightId,randomId());if(e===epoch)notice.value=state.value?.auto_planning_enabled?'已提交持久规划请求；刷新查看结果。':'已请求建议；自动规划仍保持关闭，不会自动激活。';}catch(c){if(e===epoch)error.value=String(c);}finally{if(e===epoch)busy.value=false;}}
function title(id:string){return state.value?.goals.find(g=>g.id===id)?.desired_outcome??id;}
watch(()=>props.fluctlightId,()=>{epoch++;state.value=null;runs.value=[];error.value='';notice.value='';pending=null;void load();},{immediate:true});
</script>
<template>
<section class="planning-panel" aria-label="独立目标规划">
 <div class="planning-actions"><h3>目标集合与自动规划</h3><Button variant="outline" :disabled="busy" @click="load">刷新规划状态</Button></div>
 <p v-if="error" role="alert">{{error}}</p><p v-if="notice" role="status">{{notice}}</p>
 <template v-if="state">
 <p>活动目标 {{state.active_count}} / {{state.max_active_goals}} · {{state.ordering_mode==='manual'?'人工顺序':'自动顺序'}} · 集合版本 {{state.revision}}</p>
 <p v-if="state.capacity_violation" role="alert">存量目标已超额；请明确暂停不再推进的目标，新增和恢复将被拒绝。</p>
 <p>暂停自动规划只停止补充、激活和重新排序；既有目标和日程继续按原授权运行。</p>
 <div v-if="!readOnly" class="planning-actions"><Button :disabled="busy" @click="apply({autoPlanningEnabled:!state.auto_planning_enabled},'Owner更改自动规划开关')">{{state.auto_planning_enabled?'暂停自动规划':'启用自动规划'}}</Button><Button variant="outline" :disabled="busy" @click="plan">立即规划{{state.auto_planning_enabled?'':'建议'}}</Button><Button variant="outline" :disabled="busy" @click="apply({orderingMode:'automatic'},'Owner恢复自动排序')">恢复自动排序</Button></div>
 <ol><li v-for="(id,index) in order" :key="id"><span>{{title(id)}}</span><template v-if="!readOnly"><button :disabled="busy||index===0" :aria-label="`上移${title(id)}`" @click="move(index,-1)">上移</button><button :disabled="busy||index===order.length-1" :aria-label="`下移${title(id)}`" @click="move(index,1)">下移</button></template></li></ol>
 <Button v-if="!readOnly" :disabled="busy" @click="apply({orderingMode:'manual',order},'Owner确认人工目标顺序')">保存人工顺序</Button>
 <form v-if="!readOnly" @submit.prevent="apply({dependencies:{[dependencyGoal]:prerequisites}},'Owner编辑前置目标')"><h4>行动前置依赖</h4><label>下游目标<select v-model="dependencyGoal" required><option value="">选择目标</option><option v-for="g in state.goals" :key="g.id" :value="g.id">{{g.desired_outcome}}</option></select></label><label>前置目标（需全部完成）<select v-model="prerequisites" multiple><option v-for="g in state.goals.filter(g=>g.id!==dependencyGoal)" :key="g.id" :value="g.id">{{g.desired_outcome}}</option></select></label><p>生成来源只用于追溯，与行动前置依赖分别保存。</p><Button :disabled="busy||!dependencyGoal">保存依赖</Button></form>
 <div v-if="!readOnly"><p v-for="g in state.goals.filter(g=>g.context_review_required===true)" :key="g.id">{{g.desired_outcome}}：背景变更后待复核。<Button variant="outline" :disabled="busy" @click="apply({reviewedGoalIds:[g.id]},'Owner已核对当前Actor背景与关系，允许复核后行动')">已核对当前背景</Button></p></div>
 <ul><li v-for="(d,index) in state.dependencies" :key="index">{{title(String(d.goal_id))}} ← {{title(String(d.prerequisite_id))}}：{{d.satisfied?'已满足':'等待前置完成'}}</li></ul>
 <details v-if="!readOnly"><summary>规划恢复与候选容量策略</summary><form @submit.prevent="apply({recoveryPolicy:{merge_window_seconds:mergeSeconds,retry_backoff_seconds:retrySeconds,max_attempts:attemptLimit,lease_seconds:leaseSeconds,candidate_limit:candidateLimit}},'Owner调整规划恢复与候选上限')"><label>事件合并窗口（秒）<input v-model.number="mergeSeconds" type="number" min="1" max="60" /></label><label>失败退避（秒）<input v-model.number="retrySeconds" type="number" min="5" max="3600" /></label><label>最大自动尝试次数<input v-model.number="attemptLimit" type="number" min="1" max="10" /></label><label>任务租约（秒）<input v-model.number="leaseSeconds" type="number" min="60" max="600" /></label><label>候选保留上限<input v-model.number="candidateLimit" type="number" min="1" max="20" /></label><p>未改变信息的快照不会周期性调用模型；耗尽尝试后保留失败记录，等待新的事实或人工请求。</p><Button :disabled="busy">保存恢复策略</Button></form></details>
 <details v-if="sourceReviews.length"><summary>历史人格愿望待复核（{{sourceReviews.length}}）</summary><p>原始设定仍保留。请确认其中的稳定特征、关系配置与一次性愿望；稳定特征和关系请使用对应编辑器，不从旧愿望推断当前事实。</p><article v-for="review in sourceReviews" :key="String(review.source_id)"><pre>{{JSON.stringify(review.raw_source,null,2)}}</pre><p>来源 {{review.source_id}} · 版本 {{review.revision}}</p><template v-if="!readOnly"><label>已存在目标ID（包括终态）<input v-model="sourceGoalId" /></label><Button :disabled="busy||!sourceGoalId" @click="resolveSource(review,'link_goal')">关联既有目标，保留原状态</Button><label>独立期望结果<input v-model="sourceOutcome" maxlength="2000" /></label><label>可观察标准（每行一项）<textarea v-model="sourceCriteria" /></label><label>当前动机<input v-model="sourceMotivation" /></label><Button :disabled="busy||!sourceOutcome||!sourceCriteria||!sourceMotivation" @click="resolveSource(review,'candidate')">确认一次性愿望，保存非活动候选</Button><Button variant="outline" :disabled="busy" @click="resolveSource(review,'dismiss')">确认不是当前行动愿望，保留历史来源</Button></template></article></details>
 <details open><summary>最近规划与未补满原因</summary><p v-if="!runs.length">尚无规划记录。</p><article v-for="run in runs" :key="String(run.id)"><p>{{run.created_at}} · {{run.status}} · {{run.mode}}</p><p>{{(run.result as Record<string,unknown>)?.decision}} · {{(run.result as Record<string,unknown>)?.reason || run.error_code}}</p><p>后续复核：{{(run.result as Record<string,unknown>)?.review_condition || run.available_at}}</p></article></details>
 </template>
</section>
</template>
<style scoped>
.planning-panel{border:1px solid var(--surface-border);background:var(--surface-glass);border-radius:16px;padding:16px;display:grid;gap:12px;min-width:0}.planning-actions{display:flex;gap:8px;flex-wrap:wrap;align-items:center}h3{margin-right:auto}li{overflow-wrap:anywhere;margin:8px 0}li button{margin-left:8px}form{display:grid;gap:10px}label{display:grid;gap:6px}select{width:100%;max-width:100%;padding:8px;border:1px solid var(--surface-border);border-radius:8px}pre{white-space:pre-wrap;overflow-wrap:anywhere}article{border-top:1px solid var(--surface-border);overflow-wrap:anywhere}p{overflow-wrap:anywhere}
.planning-panel :deep(.bg-primary) { color: var(--primary-foreground); }
</style>
