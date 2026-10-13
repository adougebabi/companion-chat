<script setup lang="ts">
import { Inbox, RefreshCw } from "@lucide/vue";
import { Button } from "@/components/ui/button";
withDefaults(defineProps<{ loading?: boolean; error?: string; empty?: boolean; loadingLabel?: string; emptyLabel?: string; retryable?: boolean }>(), { loadingLabel: "正在读取…", emptyLabel: "暂无内容" });
defineEmits<{ retry: [] }>();
</script>
<template>
  <div v-if="loading" class="async-state" role="status" aria-live="polite" aria-busy="true">
    <span>{{ loadingLabel }}</span><div class="skeleton-lines" aria-hidden="true"><i /><i /><i /></div>
  </div>
  <div v-else-if="error" class="async-state" role="alert"><p>{{ error }}</p><Button v-if="retryable" variant="outline" type="button" @click="$emit('retry')"><RefreshCw :size="16" aria-hidden="true" />重新读取</Button></div>
  <div v-else-if="empty" class="async-state"><Inbox :size="24" aria-hidden="true" /><strong>{{ emptyLabel }}</strong><slot name="empty" /></div>
  <slot v-else />
</template>
