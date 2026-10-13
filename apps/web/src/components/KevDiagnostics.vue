<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import FilterPanel from "./product/FilterPanel.vue";
import FormField from "./product/FormField.vue";
import AsyncState from "./product/AsyncState.vue";
import FeedbackNotice from "./product/FeedbackNotice.vue";
import JsonViewer from "./product/JsonViewer.vue";
import { useControlCenterStore } from "../stores/control-center";
import { useConversationStore } from "../stores/conversations";
import { formatInstantInZone } from "../lib/instant";
import { zonedLocalInputToISO } from "../lib/zoned-local-input";
import { displayStatus, shortId, userError } from "../lib/user-feedback";
import { kevDecisionPoints } from "../lib/kev-presentation";
import { focusField } from "../composables/use-form-errors";
const store=useControlCenterStore(), conversation=useConversationStore();
const rows=computed(()=>store.kevDecisionRows), cursor=computed(()=>store.kevDecisionCursor), busy=computed(()=>store.kevDecisionLoading);
const filter=reactive({actor_self:"",actor_user:"",agent:"",decision_point:"",policy_outcome:"",call_status:"",application_status:"",from:"",to:""});
const fromLocal=ref(''),toLocal=ref(''),dateErrors=ref<Record<string,string>>({}),applied=ref<Record<string,string>>({});
const appliedCount=computed(()=>Object.values(applied.value).filter(Boolean).length);
function time(value:unknown){return formatInstantInZone(String(value??""),store.diagnosticDisplayTimezone);}
async function load(reset=true){
  if(reset){
    dateErrors.value={};
    try{filter.from=fromLocal.value ? zonedLocalInputToISO(fromLocal.value,store.diagnosticDisplayTimezone) : '';}catch{dateErrors.value['kev-from']='该时区内的开始时间无效，请重新选择。';}
    try{filter.to=toLocal.value ? zonedLocalInputToISO(toLocal.value,store.diagnosticDisplayTimezone) : '';}catch{dateErrors.value['kev-to']='该时区内的结束时间无效，请重新选择。';}
    if(filter.from&&filter.to&&Date.parse(filter.from)>Date.parse(filter.to))dateErrors.value['kev-to']='结束时间不能早于开始时间。';
    if(Object.keys(dateErrors.value).length){await focusField(Object.keys(dateErrors.value)[0]);return;}
  }
  const captured=reset ? Object.fromEntries(Object.entries(filter).map(([key,value])=>[key,value.trim()])) : {...applied.value};
  await store.loadKevDecisions(captured,reset);
  if(reset && !store.kevDecisionError && store.kevDecisionFilter===JSON.stringify(captured))applied.value=captured;
}
function clear(){Object.keys(filter).forEach(key=>filter[key as keyof typeof filter]='');fromLocal.value='';toLocal.value='';dateErrors.value={};void load();}
async function exportRows(){
  store.kevDecisionLoading=true;store.kevDecisionError='';
  try{
    const records:Array<Record<string,unknown>>=[];let next='';const captured={...applied.value};
    do{const page=await store.readKevDecisions({...captured,cursor:next,limit:'500'});records.push(...page.items);next=page.next_cursor;}while(next&&records.length<10000);
    const bundle={filters:captured,items:records,complete:!next,next_cursor:next};const link=document.createElement('a');const url=URL.createObjectURL(new Blob([JSON.stringify(bundle,null,2)],{type:'application/json'}));link.href=url;link.download='kev-decisions.json';link.click();URL.revokeObjectURL(url);
  }catch{store.kevDecisionError='导出失败，请重试。';}finally{store.kevDecisionLoading=false;}
}
onMounted(()=>void load());
</script>
<template>
  <section class="kev-diagnostics-card" aria-labelledby="kev-diagnostics-title">
    <h3 id="kev-diagnostics-title" class="sr-only">Kev 决策记录</h3>
    <p class="section-intro">模型结论与实际动作分别记录，采用率不表示判断正确率。时间显示：{{store.diagnosticDisplayTimezone}}</p>
    <FilterPanel :busy="busy" :applied-count="appliedCount" :advanced-open="Object.values(dateErrors).some(Boolean)" @apply="load()" @clear="clear">
      <FormField id="kev-filter-instance" label="摇光实例" v-slot="{controlAttrs}"><Select :model-value="filter.actor_self || '__all__'" @update:model-value="value=>filter.actor_self=value==='__all__'?'':String(value)"><SelectTrigger v-bind="controlAttrs"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="__all__">全部实例</SelectItem><SelectItem v-for="instance in conversation.fluctlights" :key="instance.id" :value="instance.id">{{String(instance.identity.name ?? shortId(instance.id))}}</SelectItem></SelectContent></Select></FormField>
      <FormField id="kev-filter-point" label="决策点" v-slot="{controlAttrs}"><Select :model-value="filter.decision_point || '__all__'" @update:model-value="value=>filter.decision_point=value==='__all__'?'':String(value)"><SelectTrigger v-bind="controlAttrs"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="__all__">全部决策点</SelectItem><SelectItem v-for="(label,key) in kevDecisionPoints" :key="key" :value="key">{{label}}</SelectItem></SelectContent></Select></FormField>
      <template #advanced>
        <FormField id="kev-filter-actor" label="对象 ID" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model="filter.actor_user" /></FormField>
        <FormField id="kev-filter-agent" label="Agent" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model="filter.agent" /></FormField>
        <FormField id="kev-filter-policy" label="策略结论" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model="filter.policy_outcome" placeholder="例如 use_original" /></FormField>
        <FormField id="kev-filter-call" label="调用状态" v-slot="{controlAttrs}"><Select :model-value="filter.call_status || '__all__'" @update:model-value="value=>filter.call_status=value==='__all__'?'':String(value)"><SelectTrigger v-bind="controlAttrs"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="__all__">全部状态</SelectItem><SelectItem value="ok">成功</SelectItem><SelectItem value="unavailable">不可用</SelectItem><SelectItem value="timeout">超时</SelectItem></SelectContent></Select></FormField>
        <FormField id="kev-filter-application" label="应用状态" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model="filter.application_status" placeholder="例如 original_used" /></FormField>
        <FormField id="kev-from" label="开始时间" v-model:error="dateErrors['kev-from']" :hint="`按 ${store.diagnosticDisplayTimezone} 输入`" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model="fromLocal" type="datetime-local" /></FormField>
        <FormField id="kev-to" label="结束时间" v-model:error="dateErrors['kev-to']" :hint="`按 ${store.diagnosticDisplayTimezone} 输入`" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model="toLocal" type="datetime-local" /></FormField>
      </template>
      <template #actions><Button type="button" variant="ghost" :disabled="busy" @click="exportRows">导出当前结果</Button></template>
    </FilterPanel>
    <FeedbackNotice :message="store.kevDecisionError" />
    <AsyncState :loading="busy" :empty="!rows.length && !store.kevDecisionError" loading-label="正在读取决策记录…" empty-label="暂无匹配记录">
      <div class="diagnostic-drawer-body">
        <article v-for="row in rows" :key="String(row.id)" class="diagnostic-row">
          <div class="diagnostic-meta"><strong>{{kevDecisionPoints[String(row.decision_point)] ?? row.decision_point}}</strong><span class="status-pill">{{displayStatus(row.call_status)}}</span><small><time :datetime="String(row.started_at)">{{time(row.started_at)}}</time></small></div>
          <p>模型选择：{{(row.answer as {choice?:string}|undefined)?.choice ?? '未知'}} · 策略：{{displayStatus(row.policy_outcome)}} · 实际应用：{{displayStatus(row.application_status)}}</p>
          <p v-if="row.fallback_reason" class="field-hint">{{userError(row.fallback_reason,'当前采用回退方案，详情见原始记录。')}}</p>
          <JsonViewer :value="row" label="输入、原始响应、概率及行动关联" />
        </article>
      </div>
      <div v-if="cursor" class="load-more-container"><Button class="load-more-pill" type="button" variant="outline" :disabled="busy" @click="load(false)">加载更多</Button></div>
    </AsyncState>
  </section>
</template>
