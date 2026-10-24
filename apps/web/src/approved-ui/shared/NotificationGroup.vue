<script setup>
import {ActionButton,Surface} from './primitives.mjs';
import CharacterAvatar from './CharacterAvatar.vue';
import {userInstant} from '../../v3/presentation-time.mjs';
defineProps({group:{type:Object,required:true}});
defineEmits(['open']);
</script>
<template>
 <Surface as="section" class="card notification-group">
  <details>
   <summary class="row"><CharacterAvatar :name="group.name" :initial="group.initial" :color="group.color" /><b>{{group.name}}</b><span class="spacer" /><span>{{group.notices.length}} 条未读</span></summary>
   <p class="muted">{{group.notices[0]?.preview||group.notices[0]?.body}}</p>
   <ActionButton v-for="notice in group.notices" :key="notice.id" class="notification-item" @click="$emit('open',notice.id)">
    <b>{{notice.title||'新通知'}}</b><p>{{notice.preview||notice.body}}</p><small class="muted">{{userInstant(notice.source_occurred_at||notice.occurred_at||notice.created_at)}}</small>
   </ActionButton>
  </details>
 </Surface>
</template>
