<script setup lang="ts">
import { reactive, watch } from 'vue';
import type { SettingsSnapshot } from '../types';

const props = withDefaults(defineProps<{ settings?: SettingsSnapshot; saving?: boolean; error?: string | null }>(), { settings: () => ({}), saving: false, error: null });
const emit = defineEmits<{ (event: 'save', settings: SettingsSnapshot): void; (event: 'close'): void }>();
const draft = reactive<SettingsSnapshot>({ ...props.settings });
watch(() => props.settings, value => Object.assign(draft, value || {}), { deep: true });

function save() {
  emit('save', { ...draft });
}
</script>

<template>
  <form class="settings-sheet" @submit.prevent="save">
    <header><div><small>SETTINGS</small><h2 id="settings-dialog-title">系统设置</h2></div><button class="close-dialog" type="button" aria-label="关闭设置" @click="emit('close')">×</button></header>
    <div class="settings-sheet-body">
      <p v-if="error" class="wizard-error" role="alert">{{ error }}</p>
      <section class="settings-section"><h3>媒体</h3><label class="settings-toggle"><span><b>简化媒体模式</b><small>保留媒体状态，但不在聊天和动态中自动加载附件</small></span><input v-model="draft.simplifiedMediaMode" type="checkbox" /></label></section>
      <section class="settings-section"><h3>调试</h3><label class="settings-toggle"><span><b>显示检查器入口</b><small>允许从聊天工具菜单打开安全的调试摘要</small></span><input v-model="draft.debugInspector" type="checkbox" /></label></section>
      <p class="settings-help">模型、服务地址和 provider 凭据由服务器管理，浏览器不会保存或展示密钥。</p>
    </div>
    <footer class="wizard-footer"><button class="quiet" type="button" @click="emit('close')">取消</button><button class="primary" type="submit" :disabled="saving">{{ saving ? '保存中…' : '保存设置' }}</button></footer>
  </form>
</template>

