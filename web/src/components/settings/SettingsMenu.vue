<script setup lang="ts">
const emit = defineEmits<{ (event: 'system'): void; (event: 'create'): void; (event: 'personas'): void }>();
</script>

<template>
  <nav class="settings-menu" aria-label="设置菜单">
    <button type="button" @click="emit('system')"><span>⚙</span><div><b>系统设置</b><small>模型、媒体与服务配置</small></div><i aria-hidden="true">›</i></button>
    <button type="button" @click="emit('create')"><span>＋</span><div><b>创建摇光实例</b><small>开始一段新的关系</small></div><i aria-hidden="true">›</i></button>
    <button type="button" @click="emit('personas')"><span>⌁</span><div><b>管理联系人</b><small>查看和切换已有摇光实例</small></div><i aria-hidden="true">›</i></button>
  </nav>
</template>

