<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import FormField from "../product/FormField.vue";
import FeedbackNotice from "../product/FeedbackNotice.vue";
import { useControlCenterStore } from "../../stores/control-center";
import { useFormErrors } from "../../composables/use-form-errors";
import { displayStatus } from "../../lib/user-feedback";

const props = defineProps<{kind:"reply"|"vision"}>();
const emit = defineEmits<{bindModel:[role:"conversation_reply"|"vision_understanding"]}>();
const store=useControlCenterStore();
const reply=reactive({mode:"fact_only",model_ref:"conversation_reply",expression_version:"fact-only-reply.v5",temperature:0.8,top_p:0.9,max_tokens:1024,max_input_tokens:8192,timeout_seconds:300,concurrency:1,retry_limit:1,failure_policy:"error",max_action_handoffs:2});
const vision=reactive({enabled:false,model_ref:"vision_understanding"});
const notice=ref(""),error=ref(""),testingVision=ref(false);
const {errors,validate}=useFormErrors();
const bindingRole=computed(()=>props.kind==="reply"?"conversation_reply":"vision_understanding");
const configKey=computed(()=>props.kind==="reply"?"conversation.reply":"vision.understanding");
const binding=computed(()=>store.providerBindings.find(item=>item.role===bindingRole.value));
const version=computed(()=>store.settings?.versions?.[configKey.value]??0);
const status=computed(()=>{
  const all=store.settings?.values["ai.capability_status"];
  if(!all||typeof all!=="object"||Array.isArray(all))return {} as Record<string,unknown>;
  const current=(all as Record<string,unknown>)[bindingRole.value];
  return current&&typeof current==="object"&&!Array.isArray(current)?current as Record<string,unknown>:{};
});
const stateLabels:Record<string,string>={disabled:"已关闭",unconfigured:"配置不完整",ready:"配置就绪",unavailable:"当前不可用"};
const fields=[
  {key:"temperature",label:"温度",min:0,max:2,integer:false,step:0.05},
  {key:"top_p",label:"Top P",min:0.01,max:1,integer:false,step:0.01},
  {key:"max_tokens",label:"回复输出上限",min:1,max:32768,integer:true,step:1},
  {key:"max_input_tokens",label:"回复上下文预算",min:1024,max:131072,integer:true,step:1024},
  {key:"timeout_seconds",label:"回复总超时（秒）",min:1,max:3600,integer:true,step:1},
  {key:"concurrency",label:"回复生成并发",min:1,max:8,integer:true,step:1},
  {key:"retry_limit",label:"可恢复错误重试次数",min:0,max:2,integer:true,step:1},
  {key:"max_action_handoffs",label:"媒体动作回交上限",min:1,max:2,integer:true,step:1},
] as const;

function restore(){
  const values=store.settings?.values;
  for(const [key,target] of [["conversation.reply",reply],["vision.understanding",vision]] as const){
    const saved=values?.[key];
    if(saved&&typeof saved==="object"&&!Array.isArray(saved))Object.assign(target,saved);
  }
}
async function read(){await store.loadSettings();if(!store.settingsError)restore();error.value=store.settingsError;}
async function save(){
  if(store.saving)return;
  error.value="";notice.value="";
  if(props.kind==="reply"&&!await validate(fields.map(field=>({id:`reply-${field.key}`,label:field.label,value:reply[field.key],kind:"number" as const,required:true,min:field.min,max:field.max,integer:field.integer}))))return;
  await store.saveSettings(props.kind==="reply"?{"conversation.reply":{...reply}}:{"vision.understanding":{...vision}},{});
  error.value=store.settingsError;
  if(!error.value){restore();notice.value="已保存。后续任务使用新配置，正在运行的私聊保持原配置。";}
}
async function testVision(){
  if(testingVision.value)return;
  testingVision.value=true;error.value="";notice.value="";
  try{const result=await store.testVisionUnderstanding();if(result.image_input_verified){notice.value="图片输入测试通过。";}else{error.value=`视觉理解测试未通过：${String(result.error_code??result.state)}`;}await store.loadSettings();}finally{testingVision.value=false;}
}
onMounted(restore);
</script>

<template>
  <form class="stack-form" novalidate @submit.prevent="save">
    <FeedbackNotice :message="error"/><FeedbackNotice :message="notice" tone="success"/>
    <p class="field-hint">{{stateLabels[String(status.state)]??'尚未读取状态'}} · 配置版本 {{version}}</p>
    <p class="field-hint">绑定模型：{{binding?.model_id??'未绑定'}}。{{status.config_complete?'配置完整。':'启用前请在模型角色绑定中配置独立模型。'}}</p>
    <div class="form-actions"><Button type="button" variant="outline" :disabled="store.saving" @click="emit('bindModel',bindingRole)">编辑模型绑定</Button><Button type="button" variant="ghost" :disabled="store.saving" @click="read">重新读取</Button></div>
    <fieldset class="stack-form" :disabled="store.saving">
      <template v-if="kind==='reply'">
        <FormField id="reply-mode" label="私聊回复模式" v-slot="{controlAttrs}"><Select v-model="reply.mode"><SelectTrigger v-bind="controlAttrs"><SelectValue/></SelectTrigger><SelectContent><SelectItem value="legacy">旧回复模式</SelectItem><SelectItem value="fact_only">Fact-only · 独立回复模型</SelectItem></SelectContent></Select></FormField>
        <p class="field-hint">认知模型负责事实和工具，回复模型自行接话与推进当前目标。采样参数独立于认知模型；总超时包含排队等待。</p>
        <FormField id="reply-expression-version" label="私聊表达版本" v-slot="{controlAttrs}"><Select v-model="reply.expression_version"><SelectTrigger v-bind="controlAttrs"><SelectValue/></SelectTrigger><SelectContent><SelectItem value="fact-only-reply.v5">v5 · 人物亲自发送消息</SelectItem><SelectItem value="fact-only-reply.v4">v4 · 回退此前表达协议</SelectItem></SelectContent></Select></FormField>
        <p class="field-hint">表达版本不更换模型或采样；新私聊使用所选版本，等待媒体的草稿保留原版本。</p>
        <div class="form-grid"><FormField v-for="field in fields" :id="`reply-${field.key}`" :key="field.key" :label="field.label" v-model:error="errors[`reply-${field.key}`]" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" :model-value="reply[field.key]" type="number" :min="field.min" :max="field.max" :step="field.step" @update:model-value="reply[field.key]=Number($event)"/></FormField></div>
        <FormField id="reply-failure-policy" label="回复模型失败时" v-slot="{controlAttrs}"><Select v-model="reply.failure_policy"><SelectTrigger v-bind="controlAttrs"><SelectValue/></SelectTrigger><SelectContent><SelectItem value="error">返回明确失败</SelectItem><SelectItem value="legacy_fallback">显式使用认知模型独立生成回复</SelectItem></SelectContent></Select></FormField>
      </template>
      <template v-else>
        <label class="checkbox-row"><input v-model="vision.enabled" type="checkbox"/><span>启用视觉理解</span></label>
        <p class="field-hint">用于看图、视觉身份及设定卡分析与校验，不控制图片或视频生成。关闭时保留已有有效参考图，新图片未检验不会自动标为验证通过。</p>
        <Button type="button" variant="outline" :disabled="store.saving||testingVision||!vision.enabled" @click="testVision">{{testingVision?'正在测试图片输入…':'测试视觉理解'}}</Button>
        <p class="field-hint">测试使用已保存配置。只调用此处绑定的视觉模型，不会自动加载第三个模型。配置就绪与模型确实支持图片输入分别记录。</p>
      </template>
    </fieldset>
    <p class="field-hint">最近调用：{{displayStatus(String(status.last_call_status??'not_called'))}}<span v-if="status.last_error"> · 最近失败：{{String(status.last_error)}}</span></p>
    <div class="form-actions"><Button type="submit" :disabled="store.saving">{{store.saving?'正在保存…':'保存配置'}}</Button></div>
  </form>
</template>
