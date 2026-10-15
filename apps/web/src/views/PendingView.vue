<script setup lang="ts">
import { onMounted } from "vue";

import Button from "@/components/ui/button/Button.vue";
import { formatDisplayValue } from "../lib/fluctlight-display";
import { useControlCenterStore } from "../stores/control-center";

defineEmits<{ openGovernance: []; openProfile: []; openDiagnostics: [] }>();
const control = useControlCenterStore();
onMounted(() => void control.loadCapabilityRequests());
</script>

<template>
  <section class="page life-page" aria-labelledby="pending-title">
    <header class="pagehead">
      <h1 id="pending-title">待处理</h1>
      <span class="badge b-plain">BP-17</span>
    </header>
    <p class="banner info">这里只列出待决定的事项，并跳到原来的处理页。本页不就地批准，避免绕过原页面的差异和确认。通知中心仍是「发生了什么」，两者分开。</p>
    <article class="card">
      <h2>能力审核</h2>
      <p v-if="control.capabilityRequestsLoading" class="small muted">正在读取能力审核…</p>
      <p v-else-if="control.capabilityRequestsError" class="statebox">能力审核列表尚未接入。{{ control.capabilityRequestsError }} 不把失败显示成 0 条。</p>
      <p v-else-if="control.capabilityRequestsLoaded && !control.capabilityRequests.length" class="small muted">当前没有待审核的能力请求。</p>
      <ul v-else class="detail-list">
        <li v-for="request in control.capabilityRequests" :key="String(request.id)">
          <strong>{{ formatDisplayValue(request.title) }}</strong>
          <small>{{ formatDisplayValue(request.capabilityKey) }} · {{ formatDisplayValue(request.status) }} · {{ formatDisplayValue(request.fluctlightId) }}</small>
          <Button variant="outline" type="button" @click="$emit('openGovernance')">到治理页处理</Button>
        </li>
      </ul>
    </article>
    <div class="life-grid">
      <article class="card">
        <h2>人格提案</h2>
        <p class="statebox">人格提案聚合尚未接入。请到角色资料处理，本页不代为接受。</p>
        <Button variant="outline" type="button" @click="$emit('openProfile')">打开角色资料</Button>
      </article>
      <article class="card">
        <h2>定义新版本</h2>
        <p class="statebox">待应用的定义版本聚合尚未接入。</p>
      </article>
      <article class="card">
        <h2>失败待决任务</h2>
        <p class="statebox">失败任务聚合尚未接入。请到运行诊断按关联标识查看。</p>
        <Button variant="outline" type="button" @click="$emit('openDiagnostics')">打开运行诊断</Button>
      </article>
    </div>
  </section>
</template>
