<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { RefreshCw } from "@lucide/vue";
import FormField from "./product/FormField.vue";
import FeedbackNotice from "./product/FeedbackNotice.vue";
import AsyncState from "./product/AsyncState.vue";
import { useControlCenterStore } from "../stores/control-center";
import { useFormErrors } from "../composables/use-form-errors";
import { kevDecisionPoints } from "../lib/kev-presentation";
import { displayStatus, userError } from "../lib/user-feedback";
const store = useControlCenterStore();
const pointLabels = kevDecisionPoints;
const config = reactive({enabled:false,media_enabled:true,endpoint:"http://127.0.0.1:8010/v1/systemone",points:Object.fromEntries(Object.keys(pointLabels).map(key=>[key,true])),timeout_ms:1500,budget_ms:3000,concurrency:1,batch_size:8,response_bytes:1048576,request_bytes:262144,strategy:"choice_argmax",min_probability:0,min_margin:0,max_deferrals:3,deferral_seconds:60,retention_days:7,model:"Jakevin/kev-4b-ternary-mlx",model_revision:"unknown"});
const ready = ref(false), loadError = ref(""), error = ref(""), notice = ref(""), secret = ref(""), testing = ref(false);
const connectionMessage = ref(""), connectionDetail = ref(""), connectionOk = ref(false);
const {errors,validate} = useFormErrors();
const version = computed(()=>store.settings?.versions?.kev ?? 0);
function restore() {
  const current = store.settings?.values.kev;
  if(current && typeof current==='object' && !Array.isArray(current)) { const points={...config.points,...(current as {points?:Record<string,boolean>}).points}; Object.assign(config,current); config.points=points; }
}
async function read(refresh=false) {
  ready.value=false;loadError.value='';
  if(refresh || !store.settings) await store.loadSettings();
  if(!store.settings || store.settingsError){loadError.value=store.settingsError || '无法读取决策配置。';return;}
  restore();ready.value=true;
}
async function save() {
  if(store.saving || testing.value)return;
  error.value='';notice.value='';
  if(!await validate([
    {id:'kev-endpoint',label:'服务接口',value:config.endpoint,required:true,kind:'url'},
    {id:'kev-timeout',label:'请求超时',value:config.timeout_ms,required:true,kind:'number',integer:true,min:50,max:60000},
    {id:'kev-budget',label:'阶段总预算',value:config.budget_ms,required:true,kind:'number',integer:true,min:Number(config.timeout_ms),max:120000},
    {id:'kev-deferrals',label:'最大连续延后次数',value:config.max_deferrals,kind:'number',integer:true,min:1,max:100},
    {id:'kev-retention',label:'记录保留天数',value:config.retention_days,kind:'number',integer:true,min:1,max:90},
    ...(config.strategy==='choice_guarded' ? [
      {id:'kev-probability',label:'最低选中概率',value:config.min_probability,kind:'number' as const,min:0,max:1},
      {id:'kev-margin',label:'前两名差距',value:config.min_margin,kind:'number' as const,min:0,max:1},
    ] : [])
  ]))return;
  await store.saveSettings({kev:{...config,points:{...config.points}}},secret.value?{'kev:systemone':secret.value}:{});
  error.value=store.settingsError;
  if(!error.value){secret.value='';restore();notice.value='配置已保存，后续请求按新策略执行。';}
}
async function test() {
  testing.value=true;connectionMessage.value='';connectionDetail.value='';
  try {
    const result=await store.testKevConnection();connectionOk.value=String(result.call_status)==='ok';
    connectionMessage.value=connectionOk.value?'连接成功。此次测试未执行业务动作。':userError(result.error_code,'连接测试未通过，请检查服务地址和运行状态。');
    if(!connectionOk.value)connectionDetail.value=`调用状态：${String(result.call_status)}\n错误码：${String(result.error_code ?? '未知')}`;
  } finally {testing.value=false;}
}
onMounted(()=>void read());
</script>
<template>
  <AsyncState :loading="!ready && !loadError" :error="loadError" retryable loading-label="正在读取已保存的决策配置…" @retry="read(true)">
    <form class="stack-form kev-settings" novalidate @submit.prevent="save">
      <div class="planning-actions"><p class="field-hint">{{version ? `已保存配置版本 ${version}` : '尚未配置，可填写后保存'}} · {{config.enabled ? '已启用' : '已关闭'}}</p><Button variant="ghost" type="button" :disabled="store.saving || testing" @click="read(true)"><RefreshCw :size="16" aria-hidden="true" />重新读取</Button></div>
      <FeedbackNotice :message="error" /><FeedbackNotice :message="notice" tone="success" />
      <fieldset class="kev-form-body" :disabled="store.saving || testing">
        <div class="option-grid"><label class="checkbox-row"><input v-model="config.enabled" type="checkbox" /><span>启用 Kev 判定</span></label><label class="checkbox-row"><input v-model="config.media_enabled" type="checkbox" /><span>启用媒体方案补充</span></label></div>
        <FormField id="kev-endpoint" label="服务接口" required v-model:error="errors['kev-endpoint']" hint="填写应用后台能访问的地址。" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model="config.endpoint" type="url" /></FormField>
        <fieldset class="checkbox-group"><legend>决策点</legend><div class="option-grid"><label v-for="(label,key) in pointLabels" :key="key" class="checkbox-row"><input v-model="config.points[key]" type="checkbox" /><span>{{label}}</span></label></div></fieldset>
        <div class="form-grid">
          <FormField id="kev-timeout" label="请求超时（毫秒）" required v-model:error="errors['kev-timeout']" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model.number="config.timeout_ms" type="number" min="50" max="60000" /></FormField>
          <FormField id="kev-budget" label="阶段总预算（毫秒）" required v-model:error="errors['kev-budget']" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model.number="config.budget_ms" type="number" :min="config.timeout_ms" max="120000" /></FormField>
          <FormField id="kev-strategy" label="决策策略" class="full-width" v-slot="{controlAttrs}"><Select v-model="config.strategy"><SelectTrigger v-bind="controlAttrs"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="choice_argmax">采用模型选择</SelectItem><SelectItem value="choice_guarded">额外置信度与差距门槛</SelectItem></SelectContent></Select></FormField>
          <template v-if="config.strategy==='choice_guarded'"><FormField id="kev-probability" label="最低选中概率" v-model:error="errors['kev-probability']" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model.number="config.min_probability" type="number" min="0" max="1" step="0.01" /></FormField><FormField id="kev-margin" label="最低前两名差距" v-model:error="errors['kev-margin']" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model.number="config.min_margin" type="number" min="0" max="1" step="0.01" /></FormField></template>
          <FormField id="kev-deferrals" label="最大连续延后次数" v-model:error="errors['kev-deferrals']" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model.number="config.max_deferrals" type="number" min="1" max="100" /></FormField>
          <FormField id="kev-retention" label="决策记录保留天数" v-model:error="errors['kev-retention']" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model.number="config.retention_days" type="number" min="1" max="90" /></FormField>
          <FormField id="kev-secret" label="访问凭据（可选）" :hint="store.settings?.configuredSecrets.includes('kev:systemone') ? '凭据已配置；留空保留现有凭据。' : '未配置凭据。'" class="full-width" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model="secret" type="password" autocomplete="new-password" placeholder="留空保留现有凭据" /></FormField>
        </div>
      </fieldset>
      <p class="field-hint">模型版本：{{displayStatus(config.model_revision)}}。连接测试使用已保存配置，修改后请先保存。</p>
      <div class="form-actions"><Button type="submit" :disabled="store.saving || testing">{{store.saving ? '正在保存…' : '保存 Kev 配置'}}</Button><Button variant="outline" type="button" :disabled="testing || store.saving" @click="test">{{testing ? '正在测试…' : '测试连接'}}</Button></div>
      <FeedbackNotice :message="connectionMessage" :tone="connectionOk ? 'success' : 'error'" :detail="connectionDetail" />
    </form>
  </AsyncState>
</template>
