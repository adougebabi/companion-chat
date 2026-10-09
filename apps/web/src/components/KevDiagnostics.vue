<script setup lang="ts">
import { computed, onMounted, reactive } from "vue";
import Button from "@/components/ui/button/Button.vue";
import Input from "@/components/ui/input/Input.vue";
import { useControlCenterStore } from "../stores/control-center";
import { formatInstantInZone } from "../lib/instant";
const store=useControlCenterStore();const rows=computed(()=>store.kevDecisionRows);const cursor=computed(()=>store.kevDecisionCursor);const busy=computed(()=>store.kevDecisionLoading);const error=computed(()=>store.kevDecisionError);
const filter=reactive({actor_self:"",actor_user:"",agent:"",decision_point:"",policy_outcome:"",call_status:"",application_status:"",from:"",to:""});
const labels:Record<string,string>={actor_self:"摇光 ID",actor_user:"Actor ID",agent:"Agent",decision_point:"决策点",policy_outcome:"策略结论",call_status:"调用状态",application_status:"应用状态",from:"开始时间（RFC3339）",to:"结束时间（RFC3339）"};
function pretty(value:unknown){return JSON.stringify(value,null,2);}
function time(value:unknown){return formatInstantInZone(String(value??""),store.diagnosticDisplayTimezone);}
async function load(reset=true){await store.loadKevDecisions({...filter},reset);}

async function exportRows(){store.kevDecisionLoading=true;store.kevDecisionError="";try{const records:Array<Record<string,unknown>>=[];let next="";const captured={...filter};do{const page=await store.readKevDecisions({...captured,cursor:next,limit:"500"});records.push(...page.items);next=page.next_cursor;}while(next&&records.length<10000);const bundle={filters:captured,items:records,complete:!next,next_cursor:next};const link=document.createElement("a");link.href=URL.createObjectURL(new Blob([JSON.stringify(bundle,null,2)],{type:"application/json"}));link.download="kev-decisions.json";link.click();URL.revokeObjectURL(link.href);}catch{store.kevDecisionError="导出失败，请重试。";}finally{store.kevDecisionLoading=false;}}
onMounted(()=>void load());
</script>
<template>
<section aria-labelledby="kev-diagnostics-title"><h2 id="kev-diagnostics-title">Kev 决策记录</h2><p>模型结论与实际动作分别记录。采用率不表示判断正确率。时间显示：{{store.diagnosticDisplayTimezone}}</p>
<form class="lifecycle-filters" @submit.prevent="load()"><label v-for="(_,key) in filter" :key="key">{{labels[key]}}<Input v-model="filter[key]" /></label><Button type="submit" :disabled="busy">应用过滤</Button><Button type="button" variant="outline" :disabled="busy" @click="exportRows">导出当前过滤</Button></form>
<p v-if="error" role="alert">{{error}}</p><p v-if="!busy&&!rows.length">暂无匹配记录。</p>
<article v-for="row in rows" :key="String(row.id)" class="diagnostic-row"><strong>{{row.decision_point}} · {{row.candidate_id}}</strong><p><time :datetime="String(row.started_at)">{{time(row.started_at)}}</time> · request {{row.request_id}} / question {{row.question_id}}</p><p>调用 {{row.call_status}} · 模型 choice {{(row.answer as {choice?:string}|undefined)?.choice??'未知'}} · 策略 {{row.policy_outcome}} · 实际应用 {{row.application_status}}</p><p v-if="row.fallback_reason||row.rule_reason">{{row.fallback_reason||row.rule_reason}}</p><details><summary>输入、原始响应、概率及行动关联</summary><pre class="overflow-auto whitespace-pre-wrap break-words">{{pretty(row)}}</pre></details></article>
<Button v-if="cursor" type="button" variant="outline" :disabled="busy" @click="load(false)">加载更多</Button>
</section>
</template>
