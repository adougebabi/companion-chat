<script setup>
import {ActionButton,Surface} from './primitives.mjs';
defineProps({groups:{type:Array,required:true}});
defineEmits(['open']);
</script>
<template>
 <Surface v-for="group in groups" :key="group.id" as="section" class="card notification-group">
  <details>
   <summary class="row"><b>{{group.label}}</b><span class="spacer"/><span>{{group.items.length}} 条记录</span></summary>
   <ActionButton v-for="item in group.items" :key="item.id" class="notification-item" :data-testid="'notification-diagnostic-'+item.id" @click="$emit('open',item.id)">
    <b>{{item.label}}</b><p>{{item.reason}}</p><span>查看原诊断 ↗</span>
   </ActionButton>
  </details>
 </Surface>
</template>
