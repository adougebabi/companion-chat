<script setup lang="ts">
import { onMounted, ref, watch } from "vue";

import Button from "@/components/ui/button/Button.vue";
import Input from "@/components/ui/input/Input.vue";
import FeedbackNotice from "../components/product/FeedbackNotice.vue";
import OperationalSettings from "../components/settings/OperationalSettings.vue";
import { userError } from "../lib/user-feedback";
import { useControlCenterStore } from "../stores/control-center";
import { useConversationStore } from "../stores/conversations";

const emit = defineEmits<{ openProfile: [] }>();
const store = useConversationStore();
const control = useControlCenterStore();
const settingsReady = ref(false);

async function load() {
  settingsReady.value = false;
  await control.loadSettings();
  settingsReady.value = Boolean(control.settings) && !control.settingsError;
  if (store.fluctlightId) await control.loadAutonomyActions(store.fluctlightId);
}

watch(() => store.fluctlightId, (id) => {
  if (id) void control.loadAutonomyActions(id);
  else control.autonomyActions = [];
});
onMounted(() => void load());
</script>

<template>
  <section class="page life-page" aria-labelledby="autonomy-title">
    <header class="pagehead">
      <h1 id="autonomy-title">自治与权限</h1>
      <span class="badge b-plain">BP-16</span>
      <Button variant="ghost" type="button" @click="emit('openProfile')">返回角色资料</Button>
    </header>
    <p class="banner info">下面的运行策略是全局的，保存后作用于全部实例。角色级免打扰、花费上限和主动触达还没有单独接口。</p>
    <FeedbackNotice :message="control.error ? userError(control.error) : ''" />
    <div class="life-grid">
      <article class="card"><h2>免打扰时段</h2><p class="statebox">角色免打扰时段尚未接入。</p></article>
      <article class="card"><h2>花费上限</h2><p class="statebox">角色花费上限尚未接入。</p></article>
      <article class="card" id="outreach"><h2>主动触达</h2><p class="statebox">角色主动触达频率尚未接入。全局通知开关也还没有通知服务。</p></article>
    </div>
    <article class="card">
      <h2>全局运行策略</h2>
      <p v-if="control.settingsError && !settingsReady" class="statebox">全局运行策略尚未读取成功。{{ control.settingsError }} 未显示可保存的空表单。</p>
      <OperationalSettings v-else-if="settingsReady" />
      <p v-else class="small muted">正在读取运行策略…</p>
    </article>
    <article class="card">
      <h2>自治动作</h2>
      <p class="small muted">暂停、延后和取消都要填写原因。空原因不会提交。</p>
      <label class="stack" for="autonomy-reason">治理原因<Input id="autonomy-reason" v-model="control.governanceReason" maxlength="2000" placeholder="说明为什么治理这条动作" /></label>
      <p v-if="!store.fluctlightId" class="statebox">请先选择角色。自治动作列表不会显示其他角色的记录。</p>
      <p v-else-if="!control.autonomyActions.length" class="small muted">当前没有自治动作记录。</p>
      <ul v-else class="detail-list">
        <li v-for="action in control.autonomyActions" :key="action.id">
          <strong>{{ action.action_type }}</strong>
          <small>{{ action.status }} · {{ action.created_at }}</small>
          <div class="row">
            <Button variant="outline" type="button" :disabled="control.saving" @click="control.governAutonomyAction(action.id, 'paused', store.fluctlightId)">暂停</Button>
            <Button variant="outline" type="button" :disabled="control.saving" @click="control.governAutonomyAction(action.id, 'deferred', store.fluctlightId)">延后</Button>
            <Button variant="outline" type="button" :disabled="control.saving" @click="control.governAutonomyAction(action.id, 'cancelled', store.fluctlightId)">取消</Button>
          </div>
        </li>
      </ul>
    </article>
  </section>
</template>
