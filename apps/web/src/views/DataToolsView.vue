<script setup lang="ts">
defineEmits<{ openRollback: [] }>();
</script>

<template>
  <section class="page life-page" aria-labelledby="data-tools-title">
    <header class="pagehead">
      <h1 id="data-tools-title">数据工具</h1>
      <span class="badge b-plain">BP-18</span>
    </header>
    <p class="banner warn">这三栏都还不能导入、备份或恢复。页面不会把 JSON 当成读档，也不会选择历史时间点。</p>
    <div class="tool-grid">
      <article class="card">
        <h2>配置包</h2>
        <p class="small">P1 / P2</p>
        <p>只包含全局定义：模板、人格片段、地点、事件模板、视觉风格。不含运行事实。导入后应成为创作中心的新版本，再经应用到实例才作用于角色。</p>
        <p class="statebox">配置包导入尚未接入。永久伤害事件校验不能绕过。JSON 创建角色属于创作定义，不是读档。</p>
      </article>
      <article class="card">
        <h2>业务数据备份</h2>
        <p class="small">P2</p>
        <p>实例运行数据的防损坏和迁移副本。只用于灾备和换机。恢复需要二次确认并记审计，不能当作剧情试错入口。</p>
        <p class="statebox">业务备份与恢复尚未接入。这里不提供时间点选择。</p>
      </article>
      <article class="card">
        <h2>时间线回档</h2>
        <p class="small">P3</p>
        <p>检查点恢复本期只做说明页，标记为未提供。</p>
        <button class="btn" type="button" @click="$emit('openRollback')">打开回档说明</button>
      </article>
    </div>
  </section>
</template>
