<script setup lang="ts">
defineProps<{
  setupAvailable: boolean;
  loading: boolean;
  error?: string | null;
}>();

const emit = defineEmits<{
  signIn: [password: string];
  setup: [token: string, password: string];
}>();

import { ref } from "vue";

const authPassword = ref("");
const setupToken = ref("");
const newOwnerPassword = ref("");

function submitSignIn() {
  emit("signIn", authPassword.value);
  authPassword.value = "";
}

function submitSetup() {
  emit("setup", setupToken.value, newOwnerPassword.value);
  setupToken.value = "";
  newOwnerPassword.value = "";
}
</script>

<template>
  <section class="auth-panel" aria-labelledby="auth-title">
    <div class="auth-mark" aria-hidden="true">F</div>
    <p class="eyebrow">FLUCTLIGHT WORKSPACE</p>
    <h1 id="auth-title">{{ setupAvailable ? "创建所有者" : "所有者登录" }}</h1>
    <p class="auth-copy">
      {{ setupAvailable ? "输入由本机管理员签发的一次性设置令牌。" : "登录后继续与 Fluctlight 实例对话。" }}
    </p>

    <form v-if="setupAvailable" class="auth-form" @submit.prevent="submitSetup">
      <label for="setup-token">设置令牌</label>
      <input id="setup-token" v-model="setupToken" type="password" autocomplete="one-time-code" required :disabled="loading" />
      <label for="setup-password">所有者密码</label>
      <input id="setup-password" v-model="newOwnerPassword" type="password" autocomplete="new-password" minlength="6" required :disabled="loading" />
      <p v-if="error" class="error-banner" role="alert">{{ error }}</p>
      <button class="primary-button" type="submit" :disabled="loading || !setupToken || newOwnerPassword.length < 6">
        {{ loading ? "正在创建..." : "创建所有者" }}
      </button>
    </form>

    <form v-else class="auth-form" @submit.prevent="submitSignIn">
      <label for="auth-password">密码</label>
      <input id="auth-password" v-model="authPassword" type="password" autocomplete="current-password" minlength="6" required :disabled="loading" />
      <p v-if="error" class="error-banner" role="alert">{{ error }}</p>
      <button class="primary-button" type="submit" :disabled="loading || authPassword.length < 6">
        {{ loading ? "正在登录..." : "登录" }}
      </button>
    </form>
  </section>
</template>
