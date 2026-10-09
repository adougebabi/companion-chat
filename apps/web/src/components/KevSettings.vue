<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import Button from "@/components/ui/button/Button.vue";
import Input from "@/components/ui/input/Input.vue";
import { useControlCenterStore } from "../stores/control-center";
const store=useControlCenterStore();
const pointLabels:Record<string,string>={"runtime.wakeup":"自动唤醒","runtime.reflection":"自动反思","goal.completion_check":"目标完成检查","goal.replenish_plan":"目标规划","tools.select":"工具预披露","persona.switch":"人格切换","context.select":"可选上下文"};
const config=reactive({enabled:false,endpoint:"http://127.0.0.1:8010/v1/systemone",points:Object.fromEntries(Object.keys(pointLabels).map(key=>[key,true])),timeout_ms:1500,budget_ms:3000,concurrency:1,batch_size:8,response_bytes:1048576,request_bytes:262144,strategy:"choice_argmax",min_probability:0,min_margin:0,max_deferrals:3,deferral_seconds:60,retention_days:7,model:"Jakevin/kev-4b-ternary-mlx",model_revision:"unknown"});
const secret=ref("");const notice=ref("");const testing=ref(false);
const version=computed(()=>store.settings?.versions?.kev??0);
function restore(){const current=store.settings?.values.kev;if(current&&typeof current==="object"&&!Array.isArray(current)){Object.assign(config,current);config.points={...config.points,...(current as {points?:Record<string,boolean>}).points};}}
async function save(){await store.saveSettings({kev:{...config,points:{...config.points}}},secret.value?{"kev:systemone":secret.value}:{});if(!store.error){secret.value="";restore();notice.value="配置已保存，后续请求将按新开关执行。";}}
async function test(){testing.value=true;notice.value="";try{const result=await store.testKevConnection();notice.value=String(result.call_status)==="ok"?"协议连接成功，未执行业务动作。":`连接未通过：${String(result.call_status)} · ${String(result.error_code??"")}`;}finally{testing.value=false;}}
onMounted(async()=>{if(!store.settings)await store.loadSettings();restore();});
</script>
<template>
<form class="stack-form" @submit.prevent="save">
  <p>当前保存配置版本：{{ version }} · {{ store.settings?.values.kev && (store.settings.values.kev as {enabled?:boolean}).enabled ? '已启用' : '已关闭' }}</p>
  <label><input v-model="config.enabled" type="checkbox" /> 启用 Kev 判定</label>
  <label>服务接口<Input v-model="config.endpoint" required type="url" /></label>
  <p class="field-note">地址必须从 Core 和 Worker 实际可达。关闭后使用原流程；异常和弃权也回原流程。</p>
  <fieldset><legend>决策点</legend><label v-for="(label,key) in pointLabels" :key="key" class="flex items-center gap-2"><input v-model="config.points[key]" type="checkbox" />{{ label }}</label></fieldset>
  <label>请求超时（毫秒）<Input v-model.number="config.timeout_ms" type="number" min="50" max="60000" /></label>
  <label>选择阶段总预算（毫秒）<Input v-model.number="config.budget_ms" type="number" :min="config.timeout_ms" max="120000" /></label>
  <label>策略<select v-model="config.strategy"><option value="choice_argmax">直接采用合法 choice</option><option value="choice_guarded">额外概率与差距门槛</option></select></label>
  <template v-if="config.strategy==='choice_guarded'"><label>最低选中概率<Input v-model.number="config.min_probability" type="number" min="0" max="1" step="0.01" /></label><label>最低前两名差距<Input v-model.number="config.min_margin" type="number" min="0" max="1" step="0.01" /></label></template>
  <label>最大连续延后次数<Input v-model.number="config.max_deferrals" type="number" min="1" max="100" /></label>
  <label>记录保留天数<Input v-model.number="config.retention_days" type="number" min="1" max="90" /></label>
  <label>可选访问凭据<Input v-model="secret" type="password" autocomplete="new-password" placeholder="留空保留现有凭据" /></label>
  <p class="field-note">{{store.settings?.configuredSecrets.includes('kev:systemone')?'凭据已配置':'未配置凭据'}} · 模型版本 {{config.model_revision}}</p>
  <div class="flex flex-wrap gap-2"><Button type="submit" :disabled="store.saving">保存 Kev 配置</Button><Button type="button" variant="outline" :disabled="testing" @click="test">测试连接</Button></div>
  <p v-if="notice" role="status">{{notice}}</p>
</form>
</template>
