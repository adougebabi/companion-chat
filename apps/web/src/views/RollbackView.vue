<script setup lang="ts">
defineEmits<{ back: [] }>();
</script>

<template>
  <section class="page life-page" aria-labelledby="rollback-title">
    <header class="pagehead">
      <h1 id="rollback-title">时间线回档</h1>
      <span class="badge b-plain">BP-18</span>
      <span class="badge b-未知">未提供</span>
      <button class="btn ghost sm" type="button" @click="$emit('back')">返回数据工具</button>
    </header>
    <p class="banner warn">P3 能力尚未提供。本页只有说明，没有恢复操作。</p>
    <article class="card">
      <h2>如果将来提供，会影响这些记录</h2>
      <ul class="detail-list">
        <li><strong>日记</strong><small>回档会改写或撤掉该时间点之后的日记与共同经历。当前日记接口本身也尚未接入。</small></li>
        <li><strong>记忆</strong><small>已修订或已遗忘的记忆可能回到检查点上的版本，之后的修订不再有效。</small></li>
        <li><strong>关系</strong><small>关系字段和关系历史会回到检查点。只回滚关系字段的现有操作仍然留在人物关系页，不受本页影响。</small></li>
      </ul>
      <p class="statebox">未提供。不选择历史时间点，也不提交恢复。</p>
    </article>
  </section>
</template>
