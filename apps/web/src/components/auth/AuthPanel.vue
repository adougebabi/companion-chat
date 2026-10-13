<script setup lang="ts">
import { ref } from "vue";

import Button from "@/components/ui/button/Button.vue";
import Input from "@/components/ui/input/Input.vue";
import FormField from "../product/FormField.vue";
import FeedbackNotice from "../product/FeedbackNotice.vue";
import { useFormErrors } from "../../composables/use-form-errors";

const props = defineProps<{
  setupAvailable: boolean;
  loading: boolean;
  error?: string | null;
}>();

const emit = defineEmits<{
  signIn: [password: string];
  setup: [token: string, password: string];
}>();

const authPassword = ref("");
const setupToken = ref("");
const newOwnerPassword = ref("");
const {errors,validate}=useFormErrors();

async function submitSignIn() {
  if(props.loading || !await validate([{id:'auth-password',label:'密码',value:authPassword.value,trim:false,required:true,minLength:6}]))return;
  emit("signIn", authPassword.value);
}

async function submitSetup() {
  if(props.loading || !await validate([{id:'setup-token',label:'设置令牌',value:setupToken.value,required:true},{id:'setup-password',label:'所有者密码',value:newOwnerPassword.value,trim:false,required:true,minLength:6}]))return;
  emit("setup", setupToken.value, newOwnerPassword.value);
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

    <form v-if="setupAvailable" class="auth-form" novalidate @submit.prevent="submitSetup">
      <FormField id="setup-token" label="设置令牌" required v-model:error="errors['setup-token']" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model="setupToken" type="password" autocomplete="one-time-code" :disabled="loading" /></FormField>
      <FormField id="setup-password" label="所有者密码" required v-model:error="errors['setup-password']" hint="至少 6 个字符。" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model="newOwnerPassword" type="password" autocomplete="new-password" minlength="6" :disabled="loading" /></FormField>
      <FeedbackNotice :message="error ?? ''" />
      <Button class="primary-button" type="submit" :disabled="loading">
        {{ loading ? "正在创建..." : "创建所有者" }}
      </Button>
    </form>

    <form v-else class="auth-form" novalidate @submit.prevent="submitSignIn">
      <FormField id="auth-password" label="密码" required v-model:error="errors['auth-password']" hint="输入所有者密码，至少 6 个字符。" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model="authPassword" type="password" autocomplete="current-password" minlength="6" :disabled="loading" /></FormField>
      <FeedbackNotice :message="error ?? ''" />
      <Button class="primary-button" type="submit" :disabled="loading">
        {{ loading ? "正在登录..." : "登录" }}
      </Button>
    </form>
  </section>
</template>
