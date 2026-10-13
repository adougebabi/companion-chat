<script setup lang="ts">
import { computed, ref } from "vue";
import { FileJson, ChevronDown } from "@lucide/vue";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import FormField from "../product/FormField.vue";
import FeedbackNotice from "../product/FeedbackNotice.vue";
import JsonViewer from "../product/JsonViewer.vue";
import { useControlCenterStore } from "../../stores/control-center";
import { useFormErrors } from "../../composables/use-form-errors";
import { configObject, mediaSettingsPatch, parseWorkflowTemplate, workflowSummary } from "../../lib/configuration-forms";
const store = useControlCenterStore();
const current = ref(configObject(store.settings?.values['media.comfyui']));
const baseUrl = ref(String(current.value.baseUrl ?? current.value.base_url ?? ''));
const workflow = ref(current.value.workflow ? JSON.stringify(current.value.workflow, null, 2) : '');
const visualWorkflow = ref(current.value.visual_identity_workflow ? JSON.stringify(current.value.visual_identity_workflow, null, 2) : '');
const visualDefault = ref(false), error = ref(''), notice = ref(''), filename = ref('');
const importTarget = ref<'image' | 'identity'>('image');
const imageEditorOpen=ref(false), identityEditorOpen=ref(false);
const importing=ref(false);
const { errors, validate, focus } = useFormErrors();
const preview = computed(() => { try { return parseWorkflowTemplate(importTarget.value === 'image' ? workflow.value : visualWorkflow.value); } catch { return null; } });
const summary = computed(() => preview.value ? workflowSummary(preview.value) : null);
async function importFile(event: Event) {
  const input = event.target as HTMLInputElement, file = input.files?.[0];
  if (!file || importing.value) return;
  const target = importTarget.value;
  importing.value=true;
  error.value = ''; notice.value = '';
  try {
    if (file.size > 1_048_576) throw new Error('文件不能超过 1 MiB。');
    const text = await file.text(); parseWorkflowTemplate(text);
    if (target === 'image') workflow.value = text; else { visualWorkflow.value = text; visualDefault.value = false; }
    filename.value = file.name; notice.value = `文件已导入${target === 'image' ? '图片' : '视觉身份'}工作流草稿，请核对后保存。`;
  } catch (cause) { error.value = cause instanceof Error && /[\u3400-\u9fff]/.test(cause.message) ? cause.message : '文件不是有效的工作流 JSON，请检查语法与导出格式。'; }
  finally { input.value = ''; importing.value=false; }
}
async function save() {
  if(store.saving || importing.value)return;
  error.value = ''; notice.value = '';
  if (!await validate([{id:'comfy-url',label:'ComfyUI地址',value:baseUrl.value,required:true,kind:'url'},{id:'comfy-workflow',label:'图片工作流',value:workflow.value,required:true}])) {
    if(errors.value['comfy-workflow']){imageEditorOpen.value=true;await focus('comfy-workflow');}
    return;
  }
  let image: Record<string,unknown>, identity: Record<string,unknown> | undefined;
  try { image = parseWorkflowTemplate(workflow.value); }
  catch { errors.value['comfy-workflow'] = '请填写非空 JSON 对象；完整占位符也可不加引号。'; imageEditorOpen.value=true;await focus('comfy-workflow');return; }
  try { if (!visualDefault.value && visualWorkflow.value.trim()) identity = parseWorkflowTemplate(visualWorkflow.value); }
  catch { errors.value['visual-identity-workflow'] = '视觉身份工作流格式无效，请检查 JSON。';identityEditorOpen.value=true;await focus('visual-identity-workflow');return; }
  await store.saveSettings({'media.comfyui':mediaSettingsPatch(current.value,baseUrl.value,image,identity,visualDefault.value)});
  error.value = store.settingsError;
  if (!error.value) {
    current.value = configObject(store.settings?.values['media.comfyui']);
    baseUrl.value=String(current.value.baseUrl ?? current.value.base_url ?? '');
    workflow.value=current.value.workflow ? JSON.stringify(current.value.workflow,null,2) : '';
    visualWorkflow.value=current.value.visual_identity_workflow ? JSON.stringify(current.value.visual_identity_workflow,null,2) : '';
    visualDefault.value=false;
    notice.value = '媒体配置已保存，已有工作流分支和其他配置均保留。';
  }
}
</script>
<template>
  <form class="stack-form media-settings" novalidate @submit.prevent="save">
    <FeedbackNotice :message="error" /><FeedbackNotice :message="notice" tone="success" />
    <FormField id="comfy-url" label="ComfyUI 服务地址" required v-model:error="errors['comfy-url']" hint="填写应用后台能访问的地址。" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model="baseUrl" type="url" placeholder="http://localhost:8188" :disabled="store.saving" /></FormField>
    <section class="workflow-import"><div class="planning-actions"><FileJson :size="20" aria-hidden="true" /><h3>导入工作流</h3></div><p class="field-hint">使用 ComfyUI 导出的 API 格式 JSON。导入只更新草稿，结构通过不代表生成服务已运行成功。</p><div class="segmented-control"><Button type="button" :variant="importTarget === 'image' ? 'default' : 'outline'" :aria-pressed="importTarget === 'image'" :disabled="importing || store.saving" @click="importTarget='image'">图片工作流</Button><Button type="button" :variant="importTarget === 'identity' ? 'default' : 'outline'" :aria-pressed="importTarget === 'identity'" :disabled="importing || store.saving" @click="importTarget='identity'">视觉身份工作流</Button></div><FormField id="workflow-file" label="选择 JSON 文件" hint="最大 1 MiB；也可在下方粘贴文本。" v-slot="{controlAttrs}"><input v-bind="controlAttrs" type="file" accept=".json,application/json" :disabled="store.saving || importing" @change="importFile" /></FormField><p v-if="filename" class="field-hint">已导入：{{filename}}</p><div v-if="summary" class="workflow-summary"><span>JSON 结构有效</span><span>{{summary.nodes}} 个节点</span><span v-if="summary.placeholders.length">{{summary.placeholders.join(' · ')}}</span></div><JsonViewer v-if="preview" :value="preview" label="预览当前工作流" /></section>
    <details class="product-disclosure" :open="imageEditorOpen" @toggle="imageEditorOpen=($event.target as HTMLDetailsElement).open"><summary><span>粘贴或编辑图片工作流 JSON</span><ChevronDown :size="18" aria-hidden="true" /></summary><FormField id="comfy-workflow" label="图片工作流" required v-model:error="errors['comfy-workflow']" hint="提示词使用 {{prompt}}，随机种子使用 {{seed}}。" v-slot="{controlAttrs}"><Textarea v-bind="controlAttrs" v-model="workflow" rows="8" spellcheck="false" :disabled="store.saving" /></FormField></details>
    <details class="product-disclosure" :open="identityEditorOpen" @toggle="identityEditorOpen=($event.target as HTMLDetailsElement).open"><summary><span>视觉身份工作流（高级）</span><ChevronDown :size="18" aria-hidden="true" /></summary><label class="checkbox-row"><input v-model="visualDefault" type="checkbox" :disabled="store.saving" /><span>清除当前专用工作流（其它已保存分支保留）</span></label><FormField id="visual-identity-workflow" label="视觉身份工作流 JSON" v-model:error="errors['visual-identity-workflow']" hint="支持已保存的多阶段结构。留空保留原配置。" v-slot="{controlAttrs}"><Textarea v-bind="controlAttrs" v-model="visualWorkflow" rows="8" spellcheck="false" :disabled="store.saving || visualDefault" /></FormField></details>
    <JsonViewer :value="{media_workflows:current.media_workflows,visual_identity_workflows:current.visual_identity_workflows}" label="查看已保存的工作流分支（保存时保留）" />
    <div class="form-actions"><Button type="submit" :disabled="store.saving || importing">{{store.saving ? '正在保存…' : '保存媒体设置'}}</Button></div>
  </form>
</template>
