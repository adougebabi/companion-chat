<script setup>
import {onMounted,ref} from 'vue';
import {useWorkspace} from '../workspace.mjs';
import {errorMessage} from '../../v3/domain-presentation.mjs';
const w=useWorkspace(),setup=ref(null),password=ref(''),confirmation=ref(''),confirmationInput=ref(null),token=ref(''),busy=ref(false),error=ref('');
async function read(){try{setup.value=(await w.client.setupStatus()).setupAvailable;error.value='';}catch(e){error.value=errorMessage(e);}}
async function submit(){
  if(busy.value)return;
  if(setup.value&&password.value!==confirmation.value){error.value='两次输入的密码不一致，请重新核对。';confirmationInput.value?.focus();return;}
  busy.value=true;error.value='';
  try{if(setup.value)await w.client.setup(token.value,password.value);else await w.client.login(password.value);password.value='';confirmation.value='';token.value='';await w.bootstrap();w.navigatePath('/library');}
  catch(e){error.value=errorMessage(e,'登录未完成，请核对后重试。');}finally{busy.value=false;}
}
onMounted(read);
</script>
<template><main class="approved-auth"><form aria-labelledby="auth-title" class="card stack" @submit.prevent="submit"><h1>摇光</h1><h2 id="auth-title">{{setup?'设置所有者账户':'所有者登录'}}</h2><p v-if="setup===null" role="status">正在读取账户配置…</p><template v-else><label v-if="setup">初始化令牌<input v-model="token" type="password" required autocomplete="off"/></label><label>密码<input id="auth-password" v-model="password" type="password" required minlength="12" :autocomplete="setup?'new-password':'current-password'"/></label><label v-if="setup">确认密码<input ref="confirmationInput" v-model="confirmation" type="password" required minlength="12" autocomplete="new-password"/></label><button class="button primary" :disabled="busy">{{busy?'正在确认…':setup?'设置并登录':'登录'}}</button></template><p v-if="error" role="alert" class="inline-error">{{error}}</p><button v-if="setup===null&&error" class="button" type="button" @click="read">重新读取</button></form></main></template>
