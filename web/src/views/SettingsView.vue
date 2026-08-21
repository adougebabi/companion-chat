<script setup lang="ts">
import SettingsMenu from '../components/settings/SettingsMenu.vue';
const emit = defineEmits<{ (event: 'system'): void; (event: 'create'): void; (event: 'personas'): void }>();
</script>

<template>
  <section class="settings-view"><header class="pane-header"><div class="header-copy"><h1>设置</h1><p>管理应用与摇光实例</p></div></header><SettingsMenu @system="emit('system')" @create="emit('create')" @personas="emit('personas')" /></section>
</template>

