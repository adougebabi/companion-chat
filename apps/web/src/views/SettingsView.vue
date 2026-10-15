<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import KevSettings from "../components/KevSettings.vue";
import ReplyVisionSettings from "../components/settings/ReplyVisionSettings.vue";
import OperationalSettings from "../components/settings/OperationalSettings.vue";
import MediaSettings from "../components/settings/MediaSettings.vue";
import AsyncState from "../components/product/AsyncState.vue";
import FeedbackNotice from "../components/product/FeedbackNotice.vue";
import FormField from "../components/product/FormField.vue";
import { useFormErrors } from "../composables/use-form-errors";
import { displayStatus } from "../lib/user-feedback";

import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { ChevronLeft, ChevronRight, Copy, Check, CircleAlert } from "@lucide/vue";
import { settingsSections, type SettingsSection } from "../app/navigation";
import { useConversationStore } from "../stores/conversations";
import { useControlCenterStore } from "../stores/control-center";

const props = defineProps<{ section?: SettingsSection | null }>();
const emit = defineEmits<{ logout: []; navigateSection: [section: SettingsSection | null] }>();
const store = useConversationStore();
const controlCenter = useControlCenterStore();
const currentSection = computed(() => props.section ?? null);
const ready = ref(false), loadError = ref(''), localError = ref(''), localNotice = ref('');
const { errors, validate, clear } = useFormErrors();
let sectionEpoch = 0;
function resetFeedback() { sectionEpoch++; localError.value=''; localNotice.value=''; clear(); }
watch(currentSection, resetFeedback, {flush:'sync'});

const providerRoles = [
  { value: "generic_llm", label: "通用 LLM" }, { value: "embedding", label: "Embedding" },
  { value: "conversation_reply", label: "私聊回复" }, { value: "vision_understanding", label: "视觉理解" },
] as const;
const selectedProviderRole = ref<(typeof providerRoles)[number]["value"]>("generic_llm");
const roleEndpointId = ref(""); const roleModelId = ref(""); const roleTokenBudget = ref(4096); const roleTimeoutSeconds = ref(60);
const endpointPickerId = ref(""); const endpointId = ref("primary"); const endpointUrl = ref(""); const endpointSecret = ref(""); const providerKind = ref("openai-compatible");
const changedOwnerPassword = ref("");
const roleModelCopied = ref(false);
watch(selectedProviderRole, () => { resetFeedback(); roleModelCopied.value=false; }, {flush:'sync'});
const manualEndpointValue = "__new_manual_endpoint__";

function selectEndpoint(id: string) { endpointPickerId.value = id; const endpoint = controlCenter.providerEndpoints.find((item) => item.id === id); if (!endpoint) return; endpointId.value = endpoint.id; endpointUrl.value = endpoint.base_url; providerKind.value = endpoint.kind; endpointSecret.value = ""; }
async function selectRole(role: (typeof providerRoles)[number]["value"]) { selectedProviderRole.value = role; const binding = controlCenter.providerBindings.find((item) => item.role === role); roleEndpointId.value = binding?.endpoint_id ?? controlCenter.providerEndpoints.find((item) => item.id === "primary")?.id ?? controlCenter.providerEndpoints[0]?.id ?? ""; roleModelId.value = binding?.model_id ?? ""; roleTokenBudget.value = binding?.token_budget ?? 4096; roleTimeoutSeconds.value = binding?.timeout_seconds ?? 60; await controlCenter.loadProviderModels(roleEndpointId.value); }
function handleRoleChange(value: string | number) { if (typeof value !== "string") return; const role = providerRoles.find((item) => item.value === value)?.value; if (role) void selectRole(role); }
function handleRoleEndpointChange(value: unknown) { if (typeof value !== "string") return; roleEndpointId.value = value; void controlCenter.loadProviderModels(value); }
function handleEndpointPickerChange(value: unknown) { if (typeof value !== "string") return; if (value === manualEndpointValue) { endpointPickerId.value = ""; endpointId.value=""; endpointUrl.value=""; endpointSecret.value=""; providerKind.value="openai-compatible"; return; } selectEndpoint(value); }
function updateRoleTokenBudget(value: string | number) { roleTokenBudget.value = typeof value === "number" ? value : Number(value); }
function updateRoleTimeout(value: string | number) { roleTimeoutSeconds.value = typeof value === "number" ? value : Number(value); }
async function load() {
  ready.value=false; loadError.value='';
  await controlCenter.loadSettings();
  if (controlCenter.settingsError || !controlCenter.settings) { loadError.value=controlCenter.settingsError || '无法读取设置。'; return; }
  const primary=controlCenter.providerEndpoints.find(endpoint=>endpoint.id==='primary')?.id ?? controlCenter.providerEndpoints[0]?.id ?? '';
  selectEndpoint(primary); ready.value=true; void selectRole(selectedProviderRole.value);
}
async function settingsAction(action: () => Promise<unknown>, message: string) {
  const epoch=sectionEpoch; localError.value=''; localNotice.value='';
  await action();
  if (epoch!==sectionEpoch)return;
  localError.value=controlCenter.settingsError;
  if (!localError.value)localNotice.value=message;
}
async function saveRole() {
  if(controlCenter.saving)return;
  if (!await validate([
    {id:'role-endpoint',label:'Endpoint',value:roleEndpointId.value,required:true},
    {id:'role-model',label:'模型ID',value:roleModelId.value,required:true,maxLength:256},
    {id:'role-budget',label:'Token预算',value:roleTokenBudget.value,required:true,kind:'number',integer:true,min:1},
    {id:'role-timeout',label:'超时',value:roleTimeoutSeconds.value,required:true,kind:'number',integer:true,min:1}
  ]))return;
  await settingsAction(()=>controlCenter.configureModelRole({role:selectedProviderRole.value,endpointId:roleEndpointId.value,modelId:roleModelId.value.trim(),tokenBudget:roleTokenBudget.value,timeoutSeconds:roleTimeoutSeconds.value}),'模型角色绑定已保存。');
}
async function saveEndpoint() {
  if(controlCenter.saving)return;
  if (!await validate([
    {id:'endpoint-id',label:'Endpoint标识',value:endpointId.value,required:true,maxLength:128},
    {id:'provider-kind',label:'协议类型',value:providerKind.value,required:true,maxLength:64},
    {id:'endpoint-url',label:'服务地址',value:endpointUrl.value,required:true,kind:'url'}
  ]))return;
  await settingsAction(async()=>{
    if(endpointSecret.value){await controlCenter.saveSettings({}, {[`provider:${endpointId.value.trim()}`]:endpointSecret.value});if(controlCenter.settingsError)return;}
    await controlCenter.configureProviderEndpoint({endpointId:endpointId.value.trim(),kind:providerKind.value,baseUrl:endpointUrl.value.trim(),secretPurpose:`provider:${endpointId.value.trim()}`});
    if(!controlCenter.settingsError){endpointSecret.value='';selectEndpoint(endpointId.value.trim());}
  },'Endpoint已保存。请重新核对模型角色绑定。');
}
async function changePassword() {
  if (!await validate([{id:'owner-password',label:'新密码',value:changedOwnerPassword.value,required:true,trim:false,minLength:6}]))return;
  const changed=await store.changePassword(changedOwnerPassword.value);if(changed)changedOwnerPassword.value='';
}
async function copyRoleModel() {
  if (!roleModelId.value || !navigator.clipboard) return;
  try {
    await navigator.clipboard.writeText(roleModelId.value);
    roleModelCopied.value = true;
    window.setTimeout(() => { roleModelCopied.value = false; }, 1800);
  } catch {
    roleModelCopied.value = false;
  }
}
onMounted(() => void load());
</script>

<template>
  <section class="page settings-page" aria-labelledby="settings-title">
    <h1 id="settings-title" class="sr-only">设置</h1>

    <section v-if="!currentSection" class="settings-overview" aria-labelledby="settings-overview-title">
      <p class="eyebrow">系统设置</p>
      <h2 id="settings-overview-title">选择一个设置项</h2>
      <p class="page-lede">从列表选择模型、媒体、运行策略或所有者设置。这里的保存会写入当前服务，不是只记在浏览器里的偏好。</p>
      <div class="banner info">模型角色、Endpoint、媒体和运行策略都会改变真实配置。不会把人格切换直接写成运行状态。</div>
      <nav class="settings-mobile-section-list" aria-label="设置选项">
        <Button v-for="section in settingsSections" :key="section.id" class="settings-mobile-section-link" variant="outline" type="button" @click="emit('navigateSection', section.id)">
          <span><strong>{{ section.label }}</strong><small>{{ section.description }}</small></span>
          <ChevronRight :size="18" aria-hidden="true" />
        </Button>
      </nav>
    </section>

    <template v-else>
      <header class="settings-detail-header">
        <Button class="back-link" variant="ghost" type="button" @click="emit('navigateSection', null)"><ChevronLeft :size="16" aria-hidden="true" /><span>返回设置</span></Button>
        <div><p class="eyebrow">CONTROL CENTER</p><h2>{{ settingsSections.find((item) => item.id === currentSection)?.label }}</h2><p class="field-note">{{ settingsSections.find((item) => item.id === currentSection)?.description }}</p></div>
      </header>
      <FeedbackNotice :message="localError" /><FeedbackNotice :message="localNotice" tone="success" />
      <AsyncState :loading="!ready && !loadError" :error="loadError" retryable loading-label="正在读取已保存配置…" @retry="load">
      <Accordion :key="currentSection" type="single" :default-value="currentSection" class="settings-accordion">
      <AccordionItem v-if="currentSection === 'model-role'" value="model-role" class="settings-section settings-drawer">
        <AccordionTrigger class="settings-drawer-summary section-heading w-full py-0 hover:no-underline">
          <div><p class="eyebrow">MODEL ROLES</p><h2 id="model-role-title">模型角色绑定</h2><small>为认知、私聊回复、视觉理解或 Embedding 选择模型和预算</small></div>
        </AccordionTrigger>
        <AccordionContent>
          <div class="settings-drawer-body model-role-config">
            <Tabs v-model="selectedProviderRole" class="role-tabs" @update:model-value="handleRoleChange">
              <TabsList variant="line" class="segmented-control role-switcher" aria-label="模型角色">
                <TabsTrigger v-for="providerRole in providerRoles" :key="providerRole.value" :value="providerRole.value" :disabled="controlCenter.saving" class="segment-button" :class="{ selected: selectedProviderRole === providerRole.value }">
                  {{ providerRole.label }}
                </TabsTrigger>
              </TabsList>
              <TabsContent v-for="providerRole in providerRoles" :key="providerRole.value" :value="providerRole.value" class="role-tab-content">
                <div class="settings-divider"><span>服务接入</span><Separator class="settings-divider-line" /></div>
                <FormField id="role-endpoint" label="使用 Endpoint" required v-model:error="errors['role-endpoint']" v-slot="{controlAttrs}">
                  <Select :model-value="roleEndpointId || undefined" @update:model-value="handleRoleEndpointChange">
                    <SelectTrigger v-bind="controlAttrs" class="w-full"><SelectValue placeholder="请选择 endpoint" /></SelectTrigger>
                    <SelectContent>
                      <SelectItem v-for="endpoint in controlCenter.providerEndpoints" :key="endpoint.id" :value="endpoint.id">
                        {{ endpoint.id }} · {{ endpoint.base_url }}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </FormField>
                <Badge variant="outline" class="provider-model-status" :class="{ error: controlCenter.providerModelsError }" role="status">
                  <component :is="controlCenter.providerModelsError ? CircleAlert : Check" :size="15" aria-hidden="true" />
                  {{ controlCenter.providerModelsLoading ? "正在读取可用模型…" : controlCenter.providerModelsError || (controlCenter.providerModels.length ? "模型列表已从 endpoint 读取。" : "该 endpoint 尚未返回模型列表，可手动填写模型 ID。") }}
                </Badge>
                <div class="settings-divider"><span>模型参数</span><Separator class="settings-divider-line" /></div>
                <form class="stack-form" novalidate @submit.prevent="saveRole">
                  <FormField id="role-model" label="模型 ID" required v-model:error="errors['role-model']" v-slot="{controlAttrs}">
                    <span class="model-id-control">
                      <Input v-bind="controlAttrs" v-model="roleModelId" type="text" maxlength="256" placeholder="模型标识" />
                      <Button variant="outline" size="icon" class="copy-button" type="button" aria-label="复制模型 ID" title="复制模型 ID" :disabled="!roleModelId" @click="copyRoleModel"><component :is="roleModelCopied ? Check : Copy" :size="16" aria-hidden="true" /></Button>
                    </span>
                  </FormField>
                  <div class="form-grid">
                    <FormField id="role-budget" label="Token 预算" v-model:error="errors['role-budget']" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" :model-value="roleTokenBudget" type="number" min="1" step="1" @update:model-value="updateRoleTokenBudget" /></FormField>
                    <FormField id="role-timeout" label="超时（秒）" v-model:error="errors['role-timeout']" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" :model-value="roleTimeoutSeconds" type="number" min="1" step="1" @update:model-value="updateRoleTimeout" /></FormField>
                  </div>
                  <Button class="primary-button" type="submit" :disabled="controlCenter.saving">{{ controlCenter.saving ? "正在保存..." : "保存当前角色绑定" }}</Button>
                </form>
              </TabsContent>
            </Tabs>
          </div>
        </AccordionContent>
      </AccordionItem>

      <AccordionItem v-if="currentSection === 'endpoint'" value="endpoint" class="settings-section settings-drawer">
        <AccordionTrigger class="settings-drawer-summary section-heading w-full py-0 hover:no-underline">
          <div><p class="eyebrow">ENDPOINTS</p><h2 id="endpoint-title">模型 Endpoint</h2><small>管理模型服务地址和协议</small></div>
        </AccordionTrigger>
        <AccordionContent>
          <div class="settings-drawer-body">
            <form class="stack-form" novalidate @submit.prevent="saveEndpoint">
              <label for="endpoint-picker">已保存 Endpoint
                <Select :model-value="endpointPickerId || manualEndpointValue" @update:model-value="handleEndpointPickerChange">
                  <SelectTrigger id="endpoint-picker" class="w-full"><SelectValue placeholder="新建或手动编辑" /></SelectTrigger>
                  <SelectContent>
                    <SelectItem :value="manualEndpointValue">新建或手动编辑</SelectItem>
                    <SelectItem v-for="endpoint in controlCenter.providerEndpoints" :key="endpoint.id" :value="endpoint.id">
                      {{ endpoint.id }} · {{ endpoint.base_url }}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </label>
              <div class="form-grid">
                <FormField id="endpoint-id" label="Endpoint 标识" required v-model:error="errors['endpoint-id']" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model="endpointId" maxlength="128" /></FormField>
                <FormField id="provider-kind" label="协议类型" required v-model:error="errors['provider-kind']" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model="providerKind" maxlength="64" /></FormField>
                <FormField id="endpoint-url" label="服务地址" required v-model:error="errors['endpoint-url']" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model="endpointUrl" type="url" placeholder="http://host:port/v1" /></FormField>
                <label for="endpoint-secret">访问密钥<Input id="endpoint-secret" v-model="endpointSecret" type="password" autocomplete="new-password" placeholder="仅写入，不会再次显示" /></label>
              </div>
              <p class="field-note">保存 endpoint 会解除其模型角色绑定；API Key 仅写入，不会返回浏览器。</p>
              <Button class="primary-button" type="submit" :disabled="controlCenter.saving">{{ controlCenter.saving ? "正在保存..." : "保存 Endpoint" }}</Button>
            </form>
          </div>
        </AccordionContent>
      </AccordionItem>

      <AccordionItem v-if="currentSection === 'binding'" value="binding" class="settings-section settings-drawer">
        <AccordionTrigger class="settings-drawer-summary section-heading w-full py-0 hover:no-underline">
          <div><p class="eyebrow">ACTIVE BINDINGS</p><h2 id="binding-title">当前角色绑定</h2><small>查看每个角色当前使用的模型</small></div>
        </AccordionTrigger>
        <AccordionContent>
          <div class="settings-drawer-body">
            <div class="binding-list">
              <p v-if="!controlCenter.providerBindings.length" class="field-note">尚未启用任何模型角色。</p>
              <div v-for="binding in controlCenter.providerBindings" :key="binding.role" class="binding-row">
                <strong>{{ providerRoles.find((role) => role.value === binding.role)?.label ?? binding.role }}</strong>
                <span>{{ binding.model_id }}</span>
                <small>{{ binding.endpoint_id }} · {{ displayStatus(binding.endpoint_status) }} · {{ binding.token_budget }} Token / {{ binding.timeout_seconds }} 秒</small>
              </div>
            </div>
          </div>
        </AccordionContent>
      </AccordionItem>

      <AccordionItem v-if="currentSection === 'media'" value="media" class="settings-section settings-drawer">
        <AccordionContent><div class="settings-drawer-body"><MediaSettings /></div></AccordionContent>
      </AccordionItem>

      <AccordionItem v-if="currentSection === 'kev'" value="kev" class="settings-section settings-drawer">
        <AccordionTrigger class="settings-drawer-summary section-heading"><h2>Kev 决策</h2></AccordionTrigger>
        <AccordionContent><div class="settings-drawer-body"><KevSettings /></div></AccordionContent>
      </AccordionItem>
      <AccordionItem v-if="currentSection === 'reply' || currentSection === 'vision'" :value="currentSection" class="settings-section settings-drawer">
        <AccordionContent><div class="settings-drawer-body"><ReplyVisionSettings :key="currentSection" :kind="currentSection==='reply'?'reply':'vision'" @bind-model="selectRole($event);emit('navigateSection','model-role')"/></div></AccordionContent>
      </AccordionItem>
      <AccordionItem v-if="currentSection === 'operations'" value="operations" class="settings-section settings-drawer">
        <AccordionContent><div class="settings-drawer-body"><OperationalSettings /></div></AccordionContent>
      </AccordionItem>

      <AccordionItem v-if="currentSection === 'owner'" value="owner" class="settings-section settings-drawer">
        <AccordionTrigger class="settings-drawer-summary section-heading w-full py-0 hover:no-underline">
          <div><p class="eyebrow">OWNER</p><h2 id="owner-title">所有者</h2><small>密码和本地会话</small></div>
        </AccordionTrigger>
        <AccordionContent>
          <div class="settings-drawer-body">
            <form class="stack-form" novalidate @submit.prevent="changePassword">
              <FormField id="owner-password" label="新密码" required v-model:error="errors['owner-password']" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model="changedOwnerPassword" type="password" autocomplete="new-password" minlength="6" /></FormField>
              <p id="owner-password-requirements" class="field-note">新密码至少 6 个字符。修改后会撤销当前所有会话。</p>
              <p v-if="store.authError" class="error-banner" role="alert">{{ store.authError }}</p>
              <Button class="primary-button" type="submit" :disabled="store.authLoading">{{ store.authLoading ? "正在修改..." : "修改密码" }}</Button>
              <Button variant="secondary" class="secondary-button" type="button" @click="emit('logout')">退出登录</Button>
            </form>
          </div>
        </AccordionContent>
      </AccordionItem>
      </Accordion>
      </AsyncState>
    </template>
  </section>
</template>
