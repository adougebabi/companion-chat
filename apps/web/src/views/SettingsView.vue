<script setup lang="ts">
import { computed, onMounted, ref } from "vue";

import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { settingsSections, type SettingsSection } from "../app/navigation";
import { useConversationStore } from "../stores/conversations";
import { useControlCenterStore } from "../stores/control-center";

const props = defineProps<{ section?: SettingsSection | null }>();
const emit = defineEmits<{ logout: []; navigateSection: [section: SettingsSection | null] }>();
const store = useConversationStore();
const controlCenter = useControlCenterStore();
const currentSection = computed(() => props.section ?? null);

const providerRoles = [
  { value: "initialization", label: "初始化" }, { value: "cognitive_assessment", label: "认知判断" }, { value: "action_realization", label: "回复生成" }, { value: "reflection", label: "反思" }, { value: "embedding", label: "Embedding" }, { value: "media_prompt", label: "媒体提示词" }, { value: "visual_identity_vision", label: "视觉身份·视觉理解" }, { value: "visual_identity_patch", label: "视觉身份·身份补丁" },
] as const;
const selectedProviderRole = ref<(typeof providerRoles)[number]["value"]>("cognitive_assessment");
const roleEndpointId = ref(""); const roleModelId = ref(""); const roleTokenBudget = ref(2048); const roleTimeoutSeconds = ref(60);
const endpointPickerId = ref(""); const endpointId = ref("primary"); const endpointUrl = ref(""); const endpointSecret = ref(""); const providerKind = ref("openai-compatible");
const comfyUiUrl = ref(""); const comfyUiWorkflow = ref(""); const visualIdentityWorkflow = ref(""); const changedOwnerPassword = ref("");
const roleModelCopied = ref(false);
const manualEndpointValue = "__new_manual_endpoint__";

function selectEndpoint(id: string) { endpointPickerId.value = id; const endpoint = controlCenter.providerEndpoints.find((item) => item.id === id); if (!endpoint) return; endpointId.value = endpoint.id; endpointUrl.value = endpoint.base_url; providerKind.value = endpoint.kind; endpointSecret.value = ""; }
async function selectRole(role: (typeof providerRoles)[number]["value"]) { selectedProviderRole.value = role; const binding = controlCenter.providerBindings.find((item) => item.role === role); roleEndpointId.value = binding?.endpoint_id ?? controlCenter.providerEndpoints.find((item) => item.id === "primary")?.id ?? controlCenter.providerEndpoints[0]?.id ?? ""; roleModelId.value = binding?.model_id ?? ""; roleTokenBudget.value = binding?.token_budget ?? 2048; roleTimeoutSeconds.value = binding?.timeout_seconds ?? 60; await controlCenter.loadProviderModels(roleEndpointId.value); }
function handleRoleChange(value: string | number) { if (typeof value !== "string") return; const role = providerRoles.find((item) => item.value === value)?.value; if (role) void selectRole(role); }
function handleRoleEndpointChange(value: unknown) { if (typeof value !== "string") return; roleEndpointId.value = value; void controlCenter.loadProviderModels(value); }
function handleEndpointPickerChange(value: unknown) { if (typeof value !== "string") return; if (value === manualEndpointValue) { endpointPickerId.value = ""; return; } selectEndpoint(value); }
function updateRoleTokenBudget(value: string | number) { roleTokenBudget.value = typeof value === "number" ? value : Number(value); }
function updateRoleTimeout(value: string | number) { roleTimeoutSeconds.value = typeof value === "number" ? value : Number(value); }
async function load() { await controlCenter.loadSettings(); const primary = controlCenter.providerEndpoints.find((endpoint) => endpoint.id === "primary")?.id ?? controlCenter.providerEndpoints[0]?.id ?? ""; selectEndpoint(primary); await selectRole(selectedProviderRole.value); const comfy = controlCenter.settings?.values["media.comfyui"]; if (comfy && typeof comfy === "object" && !Array.isArray(comfy)) { const config = comfy as Record<string, unknown>; comfyUiUrl.value = String(config.baseUrl ?? ""); const workflow = config.workflow; comfyUiWorkflow.value = workflow && typeof workflow === "object" ? JSON.stringify(workflow, null, 2) : ""; const identityWorkflow = config.visual_identity_workflow; visualIdentityWorkflow.value = identityWorkflow && typeof identityWorkflow === "object" ? JSON.stringify(identityWorkflow, null, 2) : ""; } const autonomy = controlCenter.settings?.values["product.autonomy"]; const wakeUp = controlCenter.settings?.values["product.wakeup"]; const retention = controlCenter.settings?.values["diagnostics.retention"]; controlCenter.autonomySettingsJson = JSON.stringify(autonomy && typeof autonomy === "object" && !Array.isArray(autonomy) ? autonomy : { mode: "active", allowed_actions: ["proactive_message", "memory_candidate", "relationship_candidate", "schedule_proposal", "media_request", "moment"], budget_remaining: 1 }, null, 2); controlCenter.wakeUpSettingsJson = JSON.stringify(wakeUp && typeof wakeUp === "object" && !Array.isArray(wakeUp) ? wakeUp : { enabled: true, interval_seconds: 1800 }, null, 2); controlCenter.diagnosticsRetentionJson = JSON.stringify(retention && typeof retention === "object" && !Array.isArray(retention) ? retention : { retention_days: 30, max_rows: 10000 }, null, 2); }
async function saveRole() { if (!roleEndpointId.value || !roleModelId.value.trim()) { controlCenter.error = "请为该模型角色选择 endpoint 和模型。"; return; } await controlCenter.configureModelRole({ role: selectedProviderRole.value, endpointId: roleEndpointId.value, modelId: roleModelId.value.trim(), tokenBudget: roleTokenBudget.value, timeoutSeconds: roleTimeoutSeconds.value }); if (!controlCenter.error) await selectRole(selectedProviderRole.value); }
async function saveEndpoint() { if (!endpointId.value.trim() || !endpointUrl.value.trim()) { controlCenter.error = "请完整填写 endpoint 标识和服务地址。"; return; } if (endpointSecret.value) { await controlCenter.saveSettings({}, { [`provider:${endpointId.value.trim()}`]: endpointSecret.value }); if (controlCenter.error) return; } await controlCenter.configureProviderEndpoint({ endpointId: endpointId.value.trim(), kind: providerKind.value, baseUrl: endpointUrl.value.trim(), secretPurpose: `provider:${endpointId.value.trim()}` }); if (!controlCenter.error) { endpointSecret.value = ""; selectEndpoint(endpointId.value.trim()); await selectRole(selectedProviderRole.value); } }
async function saveMedia() { if (!comfyUiUrl.value.trim() || !comfyUiWorkflow.value.trim()) { if (comfyUiUrl.value.trim() || comfyUiWorkflow.value.trim()) controlCenter.error = "请同时填写 ComfyUI URL 和图片工作流。"; return; } try { const workflow = JSON.parse(comfyUiWorkflow.value); if (!workflow || typeof workflow !== "object" || Array.isArray(workflow)) throw new Error("workflow_not_object"); const visualWorkflow = visualIdentityWorkflow.value.trim() ? JSON.parse(visualIdentityWorkflow.value) : undefined; if (visualWorkflow !== undefined && (!visualWorkflow || typeof visualWorkflow !== "object" || Array.isArray(visualWorkflow))) throw new Error("visual_identity_workflow_not_object"); await controlCenter.saveSettings({ "media.comfyui": { baseUrl: comfyUiUrl.value.trim(), workflow, ...(visualWorkflow === undefined ? {} : { visual_identity_workflow: visualWorkflow }) } }); } catch { controlCenter.error = "ComfyUI 图片工作流必须是 JSON 对象。"; } }
async function changePassword() { const changed = await store.changePassword(changedOwnerPassword.value); if (changed) changedOwnerPassword.value = ""; }
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
    <p v-if="controlCenter.error" class="error-banner" role="alert">{{ controlCenter.error }}</p>
    <section v-if="!currentSection" class="settings-overview" aria-labelledby="settings-overview-title">
      <p class="eyebrow">CONTROL CENTER</p>
      <h2 id="settings-overview-title">选择一个设置项</h2>
      <p class="page-lede">从左侧列表选择模型、媒体、运行策略或所有者设置。</p>
      <nav class="settings-mobile-section-list" aria-label="设置选项">
        <Button v-for="section in settingsSections" :key="section.id" class="settings-mobile-section-link" variant="outline" type="button" @click="emit('navigateSection', section.id)">
          <span><strong>{{ section.label }}</strong><small>{{ section.description }}</small></span>
          <span aria-hidden="true">›</span>
        </Button>
      </nav>
    </section>

    <template v-else>
      <header class="settings-detail-header">
        <Button class="back-link" variant="ghost" type="button" @click="emit('navigateSection', null)">‹ 返回设置</Button>
        <div><p class="eyebrow">CONTROL CENTER</p><h2>{{ settingsSections.find((item) => item.id === currentSection)?.label }}</h2><p class="field-note">{{ settingsSections.find((item) => item.id === currentSection)?.description }}</p></div>
      </header>
      <Accordion :key="currentSection" type="single" :default-value="currentSection" class="settings-accordion">
      <AccordionItem v-if="currentSection === 'model-role'" value="model-role" class="settings-section settings-drawer">
        <AccordionTrigger class="settings-drawer-summary section-heading w-full py-0 hover:no-underline">
          <div><p class="eyebrow">MODEL ROLES</p><h2 id="model-role-title">模型角色绑定</h2><small>为不同认知角色选择模型和预算</small></div>
        </AccordionTrigger>
        <AccordionContent>
          <div class="settings-drawer-body model-role-config">
            <Tabs v-model="selectedProviderRole" class="role-tabs" @update:model-value="handleRoleChange">
              <TabsList variant="line" class="segmented-control role-switcher" aria-label="模型角色">
                <TabsTrigger v-for="providerRole in providerRoles" :key="providerRole.value" :value="providerRole.value" class="segment-button" :class="{ selected: selectedProviderRole === providerRole.value }">
                  {{ providerRole.label }}
                </TabsTrigger>
              </TabsList>
              <TabsContent v-for="providerRole in providerRoles" :key="providerRole.value" :value="providerRole.value" class="role-tab-content">
                <div class="settings-divider"><span>服务接入</span><Separator class="settings-divider-line" /></div>
                <label for="role-endpoint">使用 Endpoint
                  <Select :model-value="roleEndpointId || undefined" @update:model-value="handleRoleEndpointChange">
                    <SelectTrigger id="role-endpoint" class="w-full"><SelectValue placeholder="请选择 endpoint" /></SelectTrigger>
                    <SelectContent>
                      <SelectItem v-for="endpoint in controlCenter.providerEndpoints" :key="endpoint.id" :value="endpoint.id">
                        {{ endpoint.id }} · {{ endpoint.base_url }}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </label>
                <Badge variant="outline" class="provider-model-status" :class="{ error: controlCenter.providerModelsError }" role="status">
                  <span aria-hidden="true">{{ controlCenter.providerModelsError ? "!" : "✓" }}</span>
                  {{ controlCenter.providerModelsError || (controlCenter.providerModels.length ? "模型列表已从 endpoint 读取。" : "该 endpoint 尚未返回模型列表，可手动填写模型 ID。") }}
                </Badge>
                <div class="settings-divider"><span>模型参数</span><Separator class="settings-divider-line" /></div>
                <form class="stack-form" @submit.prevent="saveRole">
                  <label for="role-model">模型 ID
                    <span class="model-id-control">
                      <Input id="role-model" v-model="roleModelId" type="text" maxlength="256" placeholder="模型标识" />
                      <Button variant="outline" size="icon" class="copy-button" type="button" aria-label="复制模型 ID" title="复制模型 ID" :disabled="!roleModelId" @click="copyRoleModel">{{ roleModelCopied ? "✓" : "⧉" }}</Button>
                    </span>
                  </label>
                  <div class="form-grid">
                    <label for="role-budget">Token 预算<Input id="role-budget" :model-value="roleTokenBudget" type="number" min="1" step="1" @update:model-value="updateRoleTokenBudget" /></label>
                    <label for="role-timeout">超时（秒）<Input id="role-timeout" :model-value="roleTimeoutSeconds" type="number" min="1" step="1" @update:model-value="updateRoleTimeout" /></label>
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
            <form class="stack-form" @submit.prevent="saveEndpoint">
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
                <label for="endpoint-id">Endpoint 标识<Input id="endpoint-id" v-model="endpointId" maxlength="128" /></label>
                <label for="provider-kind">协议类型<Input id="provider-kind" v-model="providerKind" maxlength="64" /></label>
                <label for="endpoint-url">服务地址<Input id="endpoint-url" v-model="endpointUrl" type="url" placeholder="http://host:port/v1" /></label>
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
                <small>{{ binding.endpoint_id }} · {{ binding.endpoint_status }} · {{ binding.token_budget }} Token / {{ binding.timeout_seconds }} 秒</small>
              </div>
            </div>
          </div>
        </AccordionContent>
      </AccordionItem>

      <AccordionItem v-if="currentSection === 'media'" value="media" class="settings-section settings-drawer">
        <AccordionTrigger class="settings-drawer-summary section-heading w-full py-0 hover:no-underline">
          <div><p class="eyebrow">MEDIA PROVIDER</p><h2 id="media-title">ComfyUI</h2><small>图片生成服务与工作流</small></div>
        </AccordionTrigger>
        <AccordionContent>
          <div class="settings-drawer-body">
            <form class="stack-form" @submit.prevent="saveMedia">
              <label for="comfy-url">ComfyUI URL<Input id="comfy-url" v-model="comfyUiUrl" type="url" placeholder="http://comfyui:8188" /></label>
              <label for="comfy-workflow">图片工作流<Textarea id="comfy-workflow" v-model="comfyUiWorkflow" rows="7" spellcheck="false" placeholder='{"node":{"inputs":{"text":"{{prompt}}"}}}' /></label>
              <label for="visual-identity-workflow">Visual Identity 工作流（可选）<Textarea id="visual-identity-workflow" v-model="visualIdentityWorkflow" rows="7" spellcheck="false" placeholder='{"seed":{"node":{"inputs":{"text":"{{prompt}}"}}},"character_sheet":{"node":{"inputs":{"text":"{{prompt}}"}}}}' /></label>
              <Button class="primary-button" type="submit" :disabled="controlCenter.saving">保存媒体设置</Button>
            </form>
          </div>
        </AccordionContent>
      </AccordionItem>

      <AccordionItem v-if="currentSection === 'operations'" value="operations" class="settings-section settings-drawer">
        <AccordionTrigger class="settings-drawer-summary section-heading w-full py-0 hover:no-underline">
          <div><p class="eyebrow">AUTONOMY / DIAGNOSTICS</p><h2 id="operations-title">运行策略</h2><small>自治行为与诊断保留</small></div>
        </AccordionTrigger>
        <AccordionContent>
          <div class="settings-drawer-body">
            <form class="stack-form" @submit.prevent="controlCenter.saveOperationalSettings">
              <label for="autonomy-json">自治策略<Textarea id="autonomy-json" v-model="controlCenter.autonomySettingsJson" rows="5" spellcheck="false" /></label>
              <label for="wake-up-json">定期唤醒<Textarea id="wake-up-json" v-model="controlCenter.wakeUpSettingsJson" rows="3" spellcheck="false" /></label>
              <label for="retention-json">诊断保留策略<Textarea id="retention-json" v-model="controlCenter.diagnosticsRetentionJson" rows="4" spellcheck="false" /></label>
              <Button class="primary-button" type="submit" :disabled="controlCenter.saving">保存运行策略</Button>
            </form>
          </div>
        </AccordionContent>
      </AccordionItem>

      <AccordionItem v-if="currentSection === 'owner'" value="owner" class="settings-section settings-drawer">
        <AccordionTrigger class="settings-drawer-summary section-heading w-full py-0 hover:no-underline">
          <div><p class="eyebrow">OWNER</p><h2 id="owner-title">所有者</h2><small>密码和本地会话</small></div>
        </AccordionTrigger>
        <AccordionContent>
          <div class="settings-drawer-body">
            <form class="stack-form" @submit.prevent="changePassword">
              <label for="owner-password">新密码<Input id="owner-password" v-model="changedOwnerPassword" type="password" autocomplete="new-password" minlength="6" required aria-describedby="owner-password-requirements" /></label>
              <p id="owner-password-requirements" class="field-note">新密码至少 6 个字符。修改后会撤销当前所有会话。</p>
              <p v-if="store.authError" class="error-banner" role="alert">{{ store.authError }}</p>
              <Button class="primary-button" type="submit" :disabled="store.authLoading || !changedOwnerPassword">{{ store.authLoading ? "正在修改..." : "修改密码" }}</Button>
              <Button variant="secondary" class="secondary-button" type="button" @click="emit('logout')">退出登录</Button>
            </form>
          </div>
        </AccordionContent>
      </AccordionItem>
      </Accordion>
    </template>
  </section>
</template>

<style scoped>
.settings-overview {
  display: grid;
  max-width: 680px;
  align-content: center;
  min-height: 100%;
  gap: 8px;
  padding: 20px 0;
}

.settings-overview h2,
.settings-detail-header h2 {
  margin: 4px 0 0;
  color: var(--ink);
  font-size: clamp(1.35rem, 2vw, 1.8rem);
  letter-spacing: -.03em;
}

.settings-mobile-section-list {
  display: grid;
  gap: 8px;
  margin-top: 20px;
}

.settings-mobile-section-link {
  display: flex;
  min-height: 58px;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 10px 14px;
  text-align: left;
}

.settings-mobile-section-link span:first-child {
  display: grid;
  min-width: 0;
  gap: 3px;
}

.settings-mobile-section-link strong,
.settings-mobile-section-link small {
  overflow-wrap: anywhere;
}

.settings-mobile-section-link small {
  color: var(--muted-ink);
  font-size: .78rem;
  line-height: 1.35;
}

.settings-detail-header {
  display: grid;
  gap: 10px;
  margin-bottom: 18px;
}

.settings-detail-header .back-link {
  justify-self: start;
}

.settings-accordion {
  min-width: 0;
  gap: 16px;
}

.settings-detail-header + .settings-accordion .settings-drawer-summary {
  display: none;
}

.settings-accordion > .settings-section {
  margin-top: 0;
}

.settings-drawer-summary {
  width: 100%;
  min-width: 0;
}

.settings-drawer[data-state="open"] .settings-drawer-summary {
  padding-bottom: 14px;
  border-bottom: 1px solid var(--surface-border);
}

.settings-drawer-summary > div {
  min-width: 0;
}

.settings-drawer-summary small {
  display: block;
  margin-top: 5px;
  color: var(--muted-ink);
  font-size: .78rem;
  font-weight: 500;
  line-height: 1.4;
}

.role-tabs {
  min-width: 0;
  gap: 18px;
}

.role-tab-content {
  min-width: 0;
}

.settings-divider::after {
  display: none;
}

.settings-divider-line {
  flex: 1;
  min-width: 0;
}

.provider-model-status {
  white-space: normal;
}

@media (min-width: 761px) {
  .settings-overview {
    min-height: 70%;
  }

  .settings-overview .settings-mobile-section-list {
    display: none;
  }

  .settings-detail-header .back-link {
    display: none;
  }
}

@media (max-width: 760px) {
  .settings-overview {
    min-height: auto;
    padding: 18px 14px 32px;
  }

  .settings-detail-header {
    padding: 12px 14px 0;
  }

  .settings-detail-header h2 {
    font-size: 1.2rem;
  }
}
</style>
