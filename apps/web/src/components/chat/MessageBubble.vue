<script setup lang="ts">
import { computed } from "vue";
import type { BrowserMessage } from "@fluctlight/browser-client";
import { Check, CheckCheck, CircleAlert, Clock } from "@lucide/vue";

import { messageTimeLabel } from "../../lib/message-time";
import { useStreamReveal } from "../../composables/use-stream-reveal";
import { deliveryLabel, formatMessageText } from "./delivery";

const props = defineProps<{
  message: BrowserMessage;
  mine: boolean;
  name: string;
  avatarLabel: string;
  streaming?: boolean;
  status?: "pending" | "failed" | "sent" | "none";
  mediaUrl?: (assetId: string) => string;
}>();

const animate = computed(() => Boolean(props.streaming));
const shown = useStreamReveal(computed(() => formatMessageText(props.message.text)), animate);
const statusText = computed(() => deliveryLabel(props.message, props.status ?? "none"));
</script>
<template>
  <article class="message-row msg" :class="mine ? 'from-user me' : 'from-fluctlight'" :data-streaming="streaming ? 'true' : 'false'">
    <div class="avatar" aria-hidden="true">{{ avatarLabel }}</div>
    <div class="message-bubble bubble">
      <p>{{ shown }}<span v-if="streaming" class="stream-caret" aria-hidden="true">▍</span></p>
      <div v-if="message.attachmentRefs?.length && mediaUrl" class="message-media">
        <img v-for="assetId in message.attachmentRefs" :key="assetId" :src="mediaUrl(assetId)" :alt="`${name} 的附件`" loading="lazy" />
      </div>
      <div class="message-meta meta">
        <time v-if="message.createdAt" :datetime="message.senderSentAt ?? message.createdAt">{{ messageTimeLabel(message) }}</time>
        <span v-if="status && status !== 'none'" class="delivery-status" :class="status" :aria-label="statusText">
          <CircleAlert v-if="status === 'failed'" :size="13" aria-hidden="true" />
          <Clock v-else-if="status === 'pending'" :size="13" aria-hidden="true" />
          <CheckCheck v-else-if="status === 'sent'" :size="15" aria-hidden="true" />
          <Check v-else :size="13" aria-hidden="true" />
        </span>
      </div>
    </div>
  </article>
</template>
