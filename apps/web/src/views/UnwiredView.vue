<script setup lang="ts">
defineProps<{ title: string; note: string; bp?: string; actionLabel?: string }>();
defineEmits<{ action: [] }>();
</script>
<template>
  <section class="page unwired-page" :aria-labelledby="'unwired-' + title">
    <header class="pagehead">
      <h1 :id="'unwired-' + title">{{ title }}</h1>
      <span v-if="bp" class="badge b-plain">{{ bp }}</span>
      <span class="badge b-未知">尚未接入</span>
    </header>
    <div class="statebox">
      <div class="big" aria-hidden="true">◌</div>
      <h2>尚未接入</h2>
      <p>{{ note }}</p>
      <button v-if="actionLabel" class="btn" type="button" @click="$emit('action')">{{ actionLabel }}</button>
    </div>
  </section>
</template>
