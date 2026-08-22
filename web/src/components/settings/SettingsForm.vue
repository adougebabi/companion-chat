<script setup lang="ts">
import { computed, reactive, watch } from 'vue';
import type { H3ConfigCheck, H3ConfigSummary, MediaProviderSummary, SettingsSnapshot } from '../types';

const props = withDefaults(defineProps<{ settings?: SettingsSnapshot; saving?: boolean; error?: string | null }>(), { settings: () => ({}), saving: false, error: null });
const emit = defineEmits<{ (event: 'save', settings: SettingsSnapshot): void; (event: 'close'): void }>();
const draft = reactive<SettingsSnapshot>({ ...props.settings });
watch(() => props.settings, value => Object.assign(draft, value || {}), { deep: true });

const providers = computed<MediaProviderSummary[]>(() => {
  const configured = draft.mediaProviders?.filter(provider => provider && provider.id) || [];
  return configured.length ? configured : [{id: 'comfyui', label: 'ComfyUI', capabilities: ['image', 'video']}];
});
const imageProviders = computed(() => providers.value.filter(provider => provider.capabilities?.includes('image')));
const videoProviders = computed(() => providers.value.filter(provider => provider.capabilities?.includes('video')));
const h3Enabled = computed(() => videoProviders.value.some(provider => provider.id === 'h3'));
const h3Summary = computed(() => draft.h3ConfigSummary || {});
const h3SummaryRows: Array<[keyof H3ConfigSummary, string]> = [
  ['executable', '可执行文件'],
  ['modelDir', '模型目录'],
  ['outputDir', '输出目录']
];

function checkFor(key: keyof H3ConfigSummary): H3ConfigCheck | undefined {
  return h3Summary.value[key];
}

function labelFor(provider: MediaProviderSummary): string {
  return provider.label || provider.id;
}

function save() {
  const payload: SettingsSnapshot = { ...draft };
  // Empty secret/path controls mean "leave the server value unchanged". The
  // server owns validation and never returns private path/key values here.
  if (!payload.lmStudioApiKey) delete payload.lmStudioApiKey;
  if (!payload.h3Executable) delete payload.h3Executable;
  if (!payload.h3Profile && !payload.h3ModelDir) delete payload.h3Profile;
  if (!payload.h3OutputDir) delete payload.h3OutputDir;
  payload.h3Defaults = {
    ...(draft.h3Defaults || {}),
    ...(draft.h3Profile ? {profile: draft.h3Profile} : {}),
    ...(draft.h3Width !== undefined ? {width: draft.h3Width} : {}),
    ...(draft.h3Height !== undefined ? {height: draft.h3Height} : {}),
    ...(draft.h3Frames !== undefined ? {frames: draft.h3Frames} : {}),
    ...(draft.h3Steps !== undefined ? {steps: draft.h3Steps} : {}),
    ...(draft.h3Layers !== undefined ? {layers: draft.h3Layers} : {}),
    ...(draft.h3Reuse !== undefined ? {reuse: draft.h3Reuse} : {}),
    ssdStreaming: draft.h3SsdStreaming === true
  };
  emit('save', payload);
}
</script>

<template>
  <form class="settings-sheet" @submit.prevent="save">
    <header><div><small>SETTINGS</small><h2 id="settings-dialog-title">系统设置</h2></div><button class="close-dialog" type="button" aria-label="关闭设置" @click="emit('close')">×</button></header>
    <div class="settings-sheet-body">
      <p v-if="error" class="wizard-error" role="alert">{{ error }}</p>
      <section class="settings-section">
        <h3>语言模型</h3>
        <label>模型服务地址<input v-model="draft.lmStudioUrl" type="url" autocomplete="off" /></label>
        <label>API Key<input v-model="draft.lmStudioApiKey" type="password" autocomplete="new-password" :placeholder="draft.hasLmStudioApiKey ? '已配置，留空保持不变' : '可选'" /></label>
        <label>模型 ID<input v-model="draft.model" autocomplete="off" /></label>
        <p class="settings-help">密钥只会提交给服务端保存，服务端响应不会回传密钥。</p>
      </section>
      <section class="settings-section">
        <h3>媒体提供方</h3>
        <p class="settings-help">媒体任务由服务端执行，浏览器不会直接连接提供方。</p>
        <label>图片提供方<select v-model="draft.imageProvider"><option v-for="provider in imageProviders" :key="`image-${provider.id}`" :value="provider.id">{{ labelFor(provider) }}</option></select></label>
        <label>视频提供方<select v-model="draft.videoProvider"><option v-for="provider in videoProviders" :key="`video-${provider.id}`" :value="provider.id">{{ labelFor(provider) }}</option></select></label>
        <div class="provider-list"><span v-for="provider in providers" :key="provider.id" class="provider-chip"><b>{{ labelFor(provider) }}</b><small>{{ provider.id }} · {{ provider.capabilities?.join(' / ') || '未声明能力' }}</small></span></div>
      </section>
      <section class="settings-section">
        <h3>调试与加载</h3>
        <label class="settings-toggle"><input v-model="draft.simplifiedMediaMode" type="checkbox" /><span><b>简化媒体模式</b><small>聊天窗口不加载图片或视频，但仍保留媒体状态。</small></span></label>
        <label class="settings-toggle"><input v-model="draft.debugInspector" type="checkbox" /><span><b>显示检查器入口</b><small>允许从聊天工具菜单打开安全的调试摘要。</small></span></label>
      </section>
      <section class="settings-section">
        <h3>ComfyUI（兼容模式）</h3>
        <label>ComfyUI 地址<input v-model="draft.comfyUrl" type="url" autocomplete="off" /></label>
        <label>图片工作流 JSON<textarea v-model="draft.imageWorkflow" rows="5" spellcheck="false" /></label>
        <label>视频工作流 JSON<textarea v-model="draft.videoWorkflow" rows="5" spellcheck="false" /></label>
      </section>
      <section class="settings-section h3-settings">
        <h3>h3.c 视频配置</h3>
        <p class="settings-help">仅在视频提供方选择 h3 时生效。路径和数值由服务端校验，命令通过参数数组启动。</p>
        <p class="settings-help">当前服务配置只安全回显末段名称，完整路径不会发送到浏览器。</p>
        <ul class="h3-config-summary">
          <li v-for="([key, label]) in h3SummaryRows" :key="key"><b>{{ label }}</b><span>{{ checkFor(key)?.configured ? (checkFor(key)?.displayName || '已配置（路径不回显）') : '未配置' }}</span><small>{{ checkFor(key)?.valid ? '可用' : checkFor(key)?.error || '未验证' }}</small></li>
        </ul>
        <p v-if="!h3Enabled" class="settings-help">当前服务端未注册 h3 提供方。</p>
        <div class="settings-grid">
          <label>可执行文件<input v-model="draft.h3Executable" autocomplete="off" placeholder="留空保持已保存配置" /></label>
          <label>模型/Profile 目录<input v-model="draft.h3Profile" autocomplete="off" placeholder="留空保持已保存配置" /></label>
          <label>输出目录<input v-model="draft.h3OutputDir" autocomplete="off" placeholder="留空保持已保存配置" /></label>
          <label>超时（毫秒）<input v-model.number="draft.h3TimeoutMs" type="number" min="1000" max="86400000" step="1000" /></label>
          <label>宽度<input v-model.number="draft.h3Width" type="number" min="1" max="8192" step="1" /></label>
          <label>高度<input v-model.number="draft.h3Height" type="number" min="1" max="8192" step="1" /></label>
          <label>帧数<input v-model.number="draft.h3Frames" type="number" min="1" max="100000" step="1" /></label>
          <label>步数<input v-model.number="draft.h3Steps" type="number" min="1" max="10000" step="1" /></label>
          <label>Layers<input v-model.number="draft.h3Layers" type="number" min="1" max="1000" step="1" /></label>
          <label>Reuse<input v-model.number="draft.h3Reuse" type="number" min="0" max="100000" step="1" /></label>
          <label class="settings-check"><input v-model="draft.h3SsdStreaming" type="checkbox" /> SSD streaming</label>
        </div>
      </section>
    </div>
    <footer class="wizard-footer"><button class="quiet" type="button" @click="emit('close')">取消</button><button class="primary" type="submit" :disabled="saving">{{ saving ? '保存中…' : '保存设置' }}</button></footer>
  </form>
</template>
