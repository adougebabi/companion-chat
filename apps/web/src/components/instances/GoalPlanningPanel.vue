<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { BrowserClient, BrowserApiError, type BrowserGoalSet, type BrowserGoalSetCommand } from '@fluctlight/browser-client';
import Button from '@/components/ui/button/Button.vue';
import Input from '@/components/ui/input/Input.vue';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ArrowUp, ArrowDown, ChevronDown, RefreshCw } from '@lucide/vue';
import FeedbackNotice from '../product/FeedbackNotice.vue';
import AsyncState from '../product/AsyncState.vue';
import JsonViewer from '../product/JsonViewer.vue';
import FormField from '../product/FormField.vue';
import { displayStatus, userError } from '../../lib/user-feedback';
import { formatInstantInZone } from '../../lib/instant';
import { validateFields } from '../../lib/form-validation';
import { randomId } from '../../random-id';
import { apiOrigin } from '../../runtime-config';
const props=defineProps<{fluctlightId:string;readOnly?:boolean;timezone?:string}>();
const client=new BrowserClient(apiOrigin);
const state=ref<BrowserGoalSet|null>(null),runs=ref<Record<string,unknown>[]>([]),error=ref(''),busy=ref(false),notice=ref('');
const order=ref<string[]>([]),dependencyGoal=ref(''),prerequisites=ref<string[]>([]);
const sourceReviews=computed(()=>{const values=state.value?.source_reviews;return Array.isArray(values)?values.filter((v):v is Record<string,unknown>=>Boolean(v)&&typeof v==='object'):[];});
const mergeSeconds=ref(2),retrySeconds=ref(60),attemptLimit=ref(5),leaseSeconds=ref(300),candidateLimit=ref(20);
const fieldErrors=ref<Record<string,string>>({});
const sourceOutcome=ref(''),sourceCriteria=ref(''),sourceMotivation=ref(''),sourceGoalId=ref('');
function resolveSource(review:Record<string,unknown>,operation:string){return apply({sourceReviews:[{source_id:review.source_id,expected_revision:review.revision,operation,goal_id:sourceGoalId.value,desired_outcome:sourceOutcome.value,success_criteria:sourceCriteria.value.split('\n').filter(Boolean),motivation:sourceMotivation.value}]},'Owner语义复核历史来源，保留原文并记录决定');}
let epoch=0;
let pending:{fingerprint:string;key:string}|null=null;
async function load(){const e=epoch;busy.value=true;try{const [s,r]=await Promise.all([client.goalSet(props.fluctlightId),client.goalPlanningHistory(props.fluctlightId)]);if(e!==epoch)return;state.value=s;mergeSeconds.value=Number(s.merge_window_seconds??2);retrySeconds.value=Number(s.retry_backoff_seconds??60);attemptLimit.value=Number(s.max_attempts??5);leaseSeconds.value=Number(s.lease_seconds??300);candidateLimit.value=Number(s.candidate_limit??20);runs.value=r;order.value=s.goals.filter(g=>g.status==='active').map(g=>g.id);error.value='';}catch(c){if(e===epoch)error.value=String(c);}finally{if(e===epoch)busy.value=false;}}
async function apply(patch:Partial<BrowserGoalSetCommand>,reason:string){if(!state.value||busy.value)return;const e=epoch;busy.value=true;const fingerprint=JSON.stringify(patch);if(pending?.fingerprint!==fingerprint)pending={fingerprint,key:randomId()};try{const value=await client.updateGoalSet(props.fluctlightId,{...patch,expectedVersion:state.value.revision,expectedFactsRevision:state.value.facts_revision,idempotencyKey:pending!.key,reason});if(e!==epoch)return;state.value=value;pending=null;notice.value='已保存到服务端，后续规划与执行使用新版本。';error.value='';}catch(c){if(e!==epoch)return;error.value=c instanceof BrowserApiError&&c.status===409?'版本、容量或依赖发生冲突；保留草稿，请刷新后重新提交。':String(c);}finally{if(e===epoch)busy.value=false;}}
function move(index:number,delta:number){const next=index+delta;if(next<0||next>=order.value.length)return;const copy=[...order.value];[copy[index],copy[next]]=[copy[next]!,copy[index]!];order.value=copy;}
async function plan(){const e=epoch;busy.value=true;try{await client.requestGoalPlanning(props.fluctlightId,randomId());if(e===epoch)notice.value=state.value?.auto_planning_enabled?'已提交持久规划请求；刷新查看结果。':'已请求建议；自动规划仍保持关闭，不会自动激活。';}catch(c){if(e===epoch)error.value=String(c);}finally{if(e===epoch)busy.value=false;}}
function runTime(value:unknown){return value ? formatInstantInZone(String(value), props.timezone || Intl.DateTimeFormat().resolvedOptions().timeZone) : '未安排';}
function saveRecovery(){fieldErrors.value=validateFields([
 {id:'planning-merge',label:'合并窗口',value:mergeSeconds.value,kind:'number',integer:true,min:1,max:60},
 {id:'planning-retry',label:'失败退避',value:retrySeconds.value,kind:'number',integer:true,min:5,max:3600},
 {id:'planning-attempts',label:'尝试次数',value:attemptLimit.value,kind:'number',integer:true,min:1,max:10},
 {id:'planning-lease',label:'任务租约',value:leaseSeconds.value,kind:'number',integer:true,min:60,max:600},
 {id:'planning-candidates',label:'候选上限',value:candidateLimit.value,kind:'number',integer:true,min:1,max:20}
]);if(Object.keys(fieldErrors.value).length)return;
return apply({recoveryPolicy:{merge_window_seconds:mergeSeconds.value,retry_backoff_seconds:retrySeconds.value,max_attempts:attemptLimit.value,lease_seconds:leaseSeconds.value,candidate_limit:candidateLimit.value}},'Owner调整规划恢复与候选上限');}
watch(dependencyGoal,()=>{prerequisites.value=state.value?.dependencies.filter(d=>d.goal_id===dependencyGoal.value).map(d=>String(d.prerequisite_id))??[];});
function title(id:string){return state.value?.goals.find(g=>g.id===id)?.desired_outcome??id;}
watch(()=>props.fluctlightId,()=>{epoch++;state.value=null;runs.value=[];error.value='';notice.value='';pending=null;void load();},{immediate:true});
</script>
<template>
  <section class="product-section planning-panel" aria-label="独立目标规划">
    <div class="planning-actions"><h3>目标集合与自动规划</h3><Button variant="outline" type="button" :disabled="busy" @click="load"><RefreshCw :size="16" aria-hidden="true" />刷新规划状态</Button></div>
    <FeedbackNotice :message="error ? userError(error, '无法读取或保存规划，请重试。') : ''" :detail="error" />
    <FeedbackNotice :message="notice" tone="success" />
    <AsyncState :loading="busy && !state" loading-label="正在读取目标规划…">
    <template v-if="state">
      <p class="field-hint">活动目标 {{state.active_count}} / {{state.max_active_goals}} · {{state.ordering_mode==='manual'?'人工顺序':'自动顺序'}} · 集合版本 {{state.revision}}</p>
      <FeedbackNotice v-if="state.capacity_violation" message="存量目标已超额；请先暂停不再推进的目标，新增和恢复暂不可用。" />
      <p class="section-intro">暂停自动规划只停止补充、激活和重新排序；既有目标和日程继续按原授权运行。</p>
      <div v-if="!readOnly" class="planning-actions"><Button variant="outline" type="button" :disabled="busy" @click="apply({autoPlanningEnabled:!state.auto_planning_enabled},'Owner更改自动规划开关')">{{state.auto_planning_enabled?'暂停自动规划':'启用自动规划'}}</Button><Button variant="outline" type="button" :disabled="busy" @click="plan">立即规划{{state.auto_planning_enabled?'':'建议'}}</Button><Button variant="ghost" type="button" :disabled="busy" @click="apply({orderingMode:'automatic'},'Owner恢复自动排序')">恢复自动排序</Button></div>
      <ol class="planning-order"><li v-for="(id,index) in order" :key="id"><span class="planning-position">{{index+1}}</span><span class="planning-goal-title">{{title(id)}}</span><div v-if="!readOnly" class="planning-order-actions"><Button variant="outline" size="icon-sm" type="button" :disabled="busy||index===0" :aria-label="`上移${title(id)}`" @click="move(index,-1)"><ArrowUp :size="16" aria-hidden="true" /></Button><Button variant="outline" size="icon-sm" type="button" :disabled="busy||index===order.length-1" :aria-label="`下移${title(id)}`" @click="move(index,1)"><ArrowDown :size="16" aria-hidden="true" /></Button></div></li></ol>
      <div v-if="!readOnly" class="form-actions"><Button type="button" :disabled="busy" @click="apply({orderingMode:'manual',order},'Owner确认人工目标顺序')">保存人工顺序</Button></div>
      <details v-if="!readOnly" class="product-disclosure planning-subsection"><summary><span>行动前置依赖</span><ChevronDown :size="18" aria-hidden="true" /></summary>
        <form class="stack-form" novalidate @submit.prevent="dependencyGoal && apply({dependencies:{[dependencyGoal]:prerequisites}},'Owner编辑前置目标')">
          <FormField id="planning-downstream" label="下游目标" required v-slot="{controlAttrs}"><Select v-model="dependencyGoal"><SelectTrigger v-bind="controlAttrs"><SelectValue placeholder="选择目标" /></SelectTrigger><SelectContent><SelectItem v-for="g in state.goals" :key="g.id" :value="g.id">{{g.desired_outcome}}</SelectItem></SelectContent></Select></FormField>
          <fieldset v-if="dependencyGoal" class="checkbox-group"><legend>前置目标（需全部完成）</legend><label v-for="g in state.goals.filter(g=>g.id!==dependencyGoal)" :key="g.id" class="checkbox-row"><input v-model="prerequisites" type="checkbox" :value="g.id" :disabled="busy" /><span>{{g.desired_outcome}}</span></label><p v-if="state.goals.length < 2" class="field-hint">没有可选的其他目标。</p></fieldset>
          <p class="field-hint">生成来源用于追溯，与行动前置依赖分别保存。</p><div class="form-actions"><Button :disabled="busy||!dependencyGoal">保存依赖</Button></div>
        </form>
      </details>
      <div v-if="!readOnly" class="planning-reviews"><div v-for="g in state.goals.filter(g=>g.context_review_required===true)" :key="g.id"><p>{{g.desired_outcome}}：背景变更后待复核。</p><Button variant="outline" type="button" :disabled="busy" @click="apply({reviewedGoalIds:[g.id]},'Owner已核对当前Actor背景与关系，允许复核后行动')">已核对当前背景</Button></div></div>
      <ul v-if="state.dependencies.length" class="dependency-summary"><li v-for="(d,index) in state.dependencies" :key="index">{{title(String(d.goal_id))}} ← {{title(String(d.prerequisite_id))}}：{{d.satisfied?'已满足':'等待前置完成'}}</li></ul>
      <details v-if="!readOnly" class="product-disclosure planning-subsection"><summary><span>规划恢复与候选容量策略</span><ChevronDown :size="18" aria-hidden="true" /></summary><form class="form-grid" novalidate @submit.prevent="saveRecovery">
        <FormField id="planning-merge" label="事件合并窗口（秒）" v-model:error="fieldErrors['planning-merge']" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model.number="mergeSeconds" type="number" min="1" max="60" /></FormField>
        <FormField id="planning-retry" label="失败退避（秒）" v-model:error="fieldErrors['planning-retry']" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model.number="retrySeconds" type="number" min="5" max="3600" /></FormField>
        <FormField id="planning-attempts" label="最大自动尝试次数" v-model:error="fieldErrors['planning-attempts']" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model.number="attemptLimit" type="number" min="1" max="10" /></FormField>
        <FormField id="planning-lease" label="任务租约（秒）" v-model:error="fieldErrors['planning-lease']" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model.number="leaseSeconds" type="number" min="60" max="600" /></FormField>
        <FormField id="planning-candidates" label="候选保留上限" v-model:error="fieldErrors['planning-candidates']" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model.number="candidateLimit" type="number" min="1" max="20" /></FormField>
        <p class="field-hint full-width">耗尽尝试后保留失败记录，等待新的事实或人工请求。</p><div class="form-actions full-width"><Button :disabled="busy">保存恢复策略</Button></div>
      </form></details>
      <details v-if="sourceReviews.length" class="product-disclosure planning-subsection"><summary><span>历史人格愿望待复核（{{sourceReviews.length}}）</span><ChevronDown :size="18" aria-hidden="true" /></summary><p class="section-intro">原始设定仍保留。稳定特征和关系请使用对应编辑器，不从旧愿望推断当前事实。</p><article v-for="review in sourceReviews" :key="String(review.source_id)"><JsonViewer :value="review.raw_source" label="查看历史来源" /><p class="field-hint">来源 {{review.source_id}} · 版本 {{review.revision}}</p><div v-if="!readOnly" class="stack-form"><label>已存在目标ID（包括终态）<Input v-model="sourceGoalId" /></label><Button variant="outline" type="button" :disabled="busy||!sourceGoalId" @click="resolveSource(review,'link_goal')">关联既有目标，保留原状态</Button><label>独立期望结果<Input v-model="sourceOutcome" maxlength="2000" /></label><label>可观察标准（每行一项）<textarea v-model="sourceCriteria" /></label><label>当前动机<Input v-model="sourceMotivation" /></label><Button type="button" :disabled="busy||!sourceOutcome||!sourceCriteria||!sourceMotivation" @click="resolveSource(review,'candidate')">确认愿望并保存候选</Button><Button variant="outline" type="button" :disabled="busy" @click="resolveSource(review,'dismiss')">不是当前行动愿望，保留历史</Button></div></article></details>
      <details class="product-disclosure planning-subsection"><summary><span>最近规划与未补满原因</span><ChevronDown :size="18" aria-hidden="true" /></summary><p v-if="!runs.length" class="field-hint">尚无规划记录。</p><article v-for="run in runs" :key="String(run.id)" class="planning-run"><p class="field-hint"><time :datetime="String(run.created_at)">{{runTime(run.created_at)}}</time> · {{displayStatus(run.status)}} · {{displayStatus(run.mode)}}</p><p>{{displayStatus((run.result as Record<string,unknown>)?.decision)}} · {{(run.result as Record<string,unknown>)?.reason || (run.error_code ? userError(run.error_code) : '暂无补充说明')}}</p><p class="field-hint">后续复核：{{(run.result as Record<string,unknown>)?.review_condition || runTime(run.available_at)}}</p><JsonViewer :value="run" label="规划技术详情" /></article></details>
    </template>
    </AsyncState>
  </section>
</template>
