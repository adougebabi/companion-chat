<script setup lang="ts">
import { nextTick, onMounted, ref } from "vue";

import { useConversationStore } from "./stores/conversations";
import { useControlCenterStore } from "./stores/control-center";

const store = useConversationStore();
const controlCenter = useControlCenterStore();
type View = "chat" | "actors" | "moments" | "diagnostics" | "settings";
const view = ref<View>("chat");
const providerUrl = ref("");
const providerSecret = ref("");
const endpointId = ref("primary");
const providerKind = ref("openai-compatible");
const modelId = ref("");
const role = ref("cognitive_assessment");
const tokenBudget = ref(2048);
const timeoutSeconds = ref(60);
const comfyUiUrl = ref("");
const comfyUiWorkflow = ref("");
const newFluctlightName = ref("Browser Acceptance Fluctlight");
const draft = ref("");
const authPassword = ref("");
const composer = ref<HTMLTextAreaElement | null>(null);
const transcript = ref<HTMLElement | null>(null);

async function send() {
  const text = draft.value;
  draft.value = "";
  await store.send(text);
  await nextTick();
  transcript.value?.scrollTo({ top: transcript.value.scrollHeight, behavior: "smooth" });
  composer.value?.focus();
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === "Enter" && !event.shiftKey) {
    event.preventDefault();
    void send();
  }
}

async function selectView(next: typeof view.value) {
  view.value = next;
  if (next === "actors") await controlCenter.loadFluctlights();
  if (next === "diagnostics") await controlCenter.loadDiagnostics();
  if (next === "settings") {
    await controlCenter.loadSettings();
    providerUrl.value = String(controlCenter.settings?.values.providerUrl ?? "");
    const comfy = controlCenter.settings?.values["media.comfyui"];
    if (comfy && typeof comfy === "object" && !Array.isArray(comfy)) {
      comfyUiUrl.value = String((comfy as Record<string, unknown>).baseUrl ?? "");
      const workflow = (comfy as Record<string, unknown>).workflow;
      comfyUiWorkflow.value = workflow && typeof workflow === "object" ? JSON.stringify(workflow, null, 2) : "";
    }
  }
}

async function saveSettings() {
  const values: Record<string, unknown> = { providerUrl: providerUrl.value };
  if (comfyUiUrl.value.trim() || comfyUiWorkflow.value.trim()) {
    if (!comfyUiUrl.value.trim() || !comfyUiWorkflow.value.trim()) {
      controlCenter.error = "ComfyUI URL and API workflow are both required.";
      return;
    }
    try {
      const workflow = JSON.parse(comfyUiWorkflow.value);
      if (!workflow || typeof workflow !== "object" || Array.isArray(workflow)) throw new Error("workflow_not_object");
      values["media.comfyui"] = { baseUrl: comfyUiUrl.value.trim(), workflow };
    } catch {
      controlCenter.error = "ComfyUI API workflow must be a JSON object.";
      return;
    }
  }
  await controlCenter.saveSettings(values, providerSecret.value);
  if (modelId.value.trim()) {
    await controlCenter.configureProvider({
      endpointId: endpointId.value,
      kind: providerKind.value,
      baseUrl: providerUrl.value,
      secretPurpose: `provider:${endpointId.value}`,
      role: role.value,
      modelId: modelId.value,
      tokenBudget: tokenBudget.value,
      timeoutSeconds: timeoutSeconds.value,
    });
  }
  providerSecret.value = "";
}

async function createFluctlightAndConversation() {
  const name = newFluctlightName.value.trim() || "New Fluctlight";
  const created = await controlCenter.createFluctlight(name);
  if (created?.id) {
    await store.startConversation([created.id]);
    view.value = "chat";
  }
}

function prettyPayload(payload: Record<string, unknown>) {
  return JSON.stringify(payload, null, 2);
}

async function signIn() {
  await store.login(authPassword.value);
  authPassword.value = "";
}

onMounted(() => void store.initialize());
</script>

<template>
  <main class="shell">
    <section v-if="store.authenticated !== true" class="auth-panel" aria-labelledby="auth-title">
      <p class="eyebrow">FLUCTLIGHT</p>
      <h1 id="auth-title">Owner sign in</h1>
      <p class="auth-copy">Sign in to open your private conversation and Control Center.</p>
      <form class="auth-form" @submit.prevent="signIn">
        <label for="auth-password">Password</label>
        <input id="auth-password" v-model="authPassword" type="password" autocomplete="current-password" required :disabled="store.authLoading" />
        <p v-if="store.authError" class="error-banner" role="alert">{{ store.authError }}</p>
        <button class="send-button" type="submit" :disabled="store.authLoading || !authPassword">{{ store.authLoading ? "Signing in..." : "Sign in" }}</button>
      </form>
    </section>
    <template v-else>
    <header class="topbar">
      <div>
        <p class="eyebrow">FLUCTLIGHT</p>
        <h1>{{ store.conversation?.title ?? "Conversation" }}</h1>
      </div>
      <div class="topbar-actions">
        <span class="status" :class="{ active: store.sending }">
          <span class="status-dot" aria-hidden="true" />
          {{ store.sending ? "Thinking" : "Ready" }}
        </span>
        <button class="secondary-button" type="button" @click="store.logout">Sign out</button>
      </div>
    </header>

    <nav class="tabbar" aria-label="Control center">
      <button v-for="item in [
        { id: 'chat', label: 'Chat' },
        { id: 'actors', label: 'Actors' },
        { id: 'moments', label: 'Moments' },
        { id: 'diagnostics', label: 'Diagnostics' },
        { id: 'settings', label: 'Settings' },
      ]" :key="item.id" class="tab" :class="{ selected: view === item.id }" type="button" @click="selectView(item.id as View)">
        {{ item.label }}
      </button>
    </nav>

    <template v-if="view === 'chat'">
      <section ref="transcript" class="transcript" aria-live="polite" aria-label="Conversation history">
      <div v-if="store.loading" class="empty-state">Loading conversation...</div>
      <div v-else-if="!store.messages.length" class="empty-state">
        <span class="empty-mark" aria-hidden="true">+</span>
        <h2>Start a conversation</h2>
        <p>Share a thought, a question, or what is happening around you.</p>
      </div>
      <article
        v-for="message in store.messages"
        :key="message.id"
        class="message-row"
        :class="message.kind === 'user' ? 'from-user' : 'from-fluctlight'"
      >
        <div class="avatar" aria-hidden="true">{{ message.kind === "user" ? "Y" : "F" }}</div>
        <div class="message-bubble">
          <p>{{ message.text }}</p>
          <span v-if="message.attachmentRefs?.length" class="attachment-chip">Attachment reference</span>
        </div>
      </article>
      </section>

      <p v-if="store.error" class="error-banner" role="alert">{{ store.error }}</p>

      <form class="composer" @submit.prevent="send">
      <label class="sr-only" for="message-composer">Message</label>
      <textarea
        id="message-composer"
        ref="composer"
        v-model="draft"
        rows="1"
        maxlength="32000"
        placeholder="Write a message..."
        :disabled="store.loading"
        @keydown="onKeydown"
      />
      <div class="composer-footer">
        <label class="attachment-input" for="attachment-reference">
          <span aria-hidden="true">+</span>
          <input id="attachment-reference" v-model="store.attachmentRef" aria-label="Attachment reference" type="text" placeholder="Attachment reference" maxlength="512" />
        </label>
        <div class="composer-actions">
          <button v-if="store.sending" class="secondary-button" type="button" @click="store.cancel">Cancel</button>
          <button class="send-button" type="submit" :disabled="store.sending || !draft.trim()">
            Send <span aria-hidden="true">↗</span>
          </button>
        </div>
      </div>
      </form>
    </template>

    <section v-else-if="view === 'actors'" class="control-panel" aria-labelledby="actors-title">
      <div class="panel-heading"><p class="eyebrow">ACTORS</p><h2 id="actors-title">Conversation participants</h2></div>
      <form class="actor-create-form" @submit.prevent="createFluctlightAndConversation">
        <label for="fluctlight-name">Create Fluctlight</label>
        <div class="actor-create-row">
          <input id="fluctlight-name" v-model="newFluctlightName" type="text" maxlength="256" />
          <button class="send-button" type="submit" :disabled="controlCenter.saving || controlCenter.loading">Create and chat</button>
        </div>
      </form>
      <div v-if="controlCenter.fluctlights.length" class="actor-list">
        <div v-for="fluctlight in controlCenter.fluctlights" :key="fluctlight.id" class="actor-row"><span class="avatar">F</span><div><strong>{{ String(fluctlight.identity.name ?? fluctlight.id) }}</strong><small>{{ fluctlight.id }}</small></div><span class="state-label">{{ fluctlight.status }}</span></div>
      </div>
      <div v-else-if="store.conversation" class="actor-list">
        <div class="actor-row"><span class="avatar">Y</span><div><strong>Owner Human</strong><small>{{ store.conversation.createdByActorId }}</small></div><span class="state-label">Owner</span></div>
      </div>
      <div v-else class="empty-state compact"><h2>No Fluctlight instances</h2><p>Create one through the authenticated Core directory.</p></div>
    </section>

    <section v-else-if="view === 'moments'" class="control-panel" aria-labelledby="moments-title">
      <div class="panel-heading"><p class="eyebrow">MOMENTS</p><h2 id="moments-title">Your shared timeline</h2></div>
      <div class="empty-state compact"><span class="empty-mark" aria-hidden="true">+</span><h2>No Moments yet</h2><p>Moments will appear here once the Core timeline has an authoritative entry.</p></div>
    </section>

    <section v-else-if="view === 'diagnostics'" class="control-panel" aria-labelledby="diagnostics-title">
      <div class="panel-heading panel-actions"><div><p class="eyebrow">CONTROL CENTER</p><h2 id="diagnostics-title">Diagnostics</h2></div><button class="secondary-button" type="button" @click="controlCenter.clearDiagnostics">Clear</button></div>
      <p v-if="controlCenter.error" class="error-banner" role="alert">{{ controlCenter.error }}</p>
      <div v-if="controlCenter.loading" class="empty-state compact">Loading diagnostics...</div>
      <div v-else-if="!controlCenter.diagnostics.length" class="empty-state compact"><h2>No diagnostic events</h2><p>Redacted model, turn and workflow events will be listed here for the Owner.</p></div>
      <div v-else class="diagnostic-list"><article v-for="event in controlCenter.diagnostics" :key="event.id" class="diagnostic-row"><div class="diagnostic-meta"><strong>{{ event.eventType }}</strong><span>{{ event.severity }}</span><small>{{ event.correlationId }}</small></div><pre>{{ prettyPayload(event.payload) }}</pre></article></div>
    </section>

    <section v-else class="control-panel" aria-labelledby="settings-title">
      <div class="panel-heading"><p class="eyebrow">SYSTEM</p><h2 id="settings-title">Settings</h2></div>
      <p v-if="controlCenter.error" class="error-banner" role="alert">{{ controlCenter.error }}</p>
      <form class="settings-form" @submit.prevent="saveSettings">
        <label>Provider URL<input v-model="providerUrl" type="url" placeholder="https://provider.internal" /></label>
        <label>Provider secret<input v-model="providerSecret" type="password" autocomplete="new-password" placeholder="Write-only secret" /></label>
        <label>ComfyUI URL<input v-model="comfyUiUrl" type="url" placeholder="http://comfyui:8188" /></label>
        <label>ComfyUI API workflow<textarea v-model="comfyUiWorkflow" rows="8" spellcheck="false" placeholder='{"node": {"inputs": {"text": "{{prompt}}"}}}' /></label>
        <div class="settings-grid">
          <label>Endpoint ID<input v-model="endpointId" type="text" maxlength="128" /></label>
          <label>Provider kind<input v-model="providerKind" type="text" maxlength="64" /></label>
          <label>Model ID<input v-model="modelId" type="text" maxlength="256" placeholder="Configured model" /></label>
          <label>Model role<select v-model="role"><option value="initialization">Initialization</option><option value="cognitive_assessment">Assessment</option><option value="action_realization">Realization</option><option value="reflection">Reflection</option><option value="embedding">Embedding</option><option value="media_prompt">Media prompt</option></select></label>
          <label>Token budget<input v-model.number="tokenBudget" type="number" min="1" step="1" /></label>
          <label>Timeout seconds<input v-model.number="timeoutSeconds" type="number" min="1" step="1" /></label>
        </div>
        <p class="field-note">Secrets are write-only and are never returned by the Core.</p>
        <button class="send-button" type="submit" :disabled="controlCenter.saving">{{ controlCenter.saving ? 'Saving...' : 'Save settings' }}</button>
      </form>
    </section>
    </template>
  </main>
</template>

<style>
:root {
  color: #16202a;
  background: #eef1f4;
  font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  font-synthesis: none;
}
* { box-sizing: border-box; }
body { margin: 0; min-width: 320px; }
button, textarea, input { font: inherit; }
.shell {
  width: min(940px, calc(100% - 32px));
  min-height: 100vh;
  margin: 0 auto;
  display: grid;
  grid-template-rows: auto 1fr auto auto;
  gap: 18px;
  padding: 30px 0 24px;
}
.auth-panel {
  align-self: center;
  width: min(100%, 420px);
  margin: 0 auto;
  padding: 28px;
  border: 1px solid #d8e0e4;
  border-radius: 6px;
  background: #fff;
  box-shadow: 0 12px 32px rgb(27 45 58 / 8%);
}
.auth-panel h1 { margin: 0 0 8px; }
.auth-copy { color: #61717f; line-height: 1.5; }
.auth-form { display: grid; gap: 10px; margin-top: 22px; }
.auth-form label { color: #42535d; font-size: .86rem; }
.auth-form input { width: 100%; padding: 11px 12px; border: 1px solid #c7d2d9; border-radius: 5px; }
.topbar-actions { display: flex; align-items: center; gap: 12px; }
.topbar { display: flex; align-items: center; justify-content: space-between; gap: 20px; }
.eyebrow { margin: 0 0 5px; color: #5b6c7b; font-size: 11px; font-weight: 750; letter-spacing: .14em; }
h1 { margin: 0; color: #18232e; font-size: clamp(1.35rem, 4vw, 1.9rem); letter-spacing: 0; }
.status { display: inline-flex; align-items: center; gap: 8px; color: #61717f; font-size: .86rem; }
.status-dot { width: 8px; height: 8px; border-radius: 50%; background: #a9b5be; }
.status.active .status-dot { background: #e59050; box-shadow: 0 0 0 4px #f8dfc9; }
.tabbar { display: flex; gap: 6px; overflow-x: auto; padding-bottom: 2px; border-bottom: 1px solid #d8dfe4; }
.tab { flex: 0 0 auto; padding: 8px 10px; border: 0; border-bottom: 2px solid transparent; background: transparent; color: #647480; cursor: pointer; font-size: .83rem; }
.tab:hover, .tab.selected { border-bottom-color: #326b75; color: #245763; }
.control-panel { min-height: 420px; overflow-y: auto; padding: 24px 8px; border-bottom: 1px solid #d8dfe4; }
.actor-create-form { display: grid; gap: 8px; max-width: 620px; margin-bottom: 18px; color: #42535d; font-size: .86rem; }
.actor-create-row { display: flex; gap: 8px; }
.actor-create-row input { flex: 1; min-height: 36px; padding: 0 10px; border: 1px solid #cbd5db; border-radius: 4px; color: #17232c; }
.panel-heading { display: flex; align-items: flex-end; justify-content: space-between; gap: 16px; margin-bottom: 22px; }
.panel-heading h2 { margin: 0; color: #1f2b35; font-size: 1.25rem; }
.panel-actions { align-items: center; }
.empty-state.compact { min-height: 260px; }
.actor-list, .diagnostic-list { display: grid; gap: 10px; }
.actor-row { display: flex; align-items: center; gap: 12px; padding: 12px; border: 1px solid #d8e0e4; border-radius: 6px; background: #fff; }
.actor-row > div { display: grid; gap: 3px; flex: 1; }
.actor-row small, .diagnostic-meta small { color: #7b8992; font-size: .76rem; }
.state-label { color: #36707b; font-size: .78rem; }
.diagnostic-row { padding: 12px; border: 1px solid #d8e0e4; border-radius: 6px; background: #fff; }
.diagnostic-meta { display: flex; align-items: center; gap: 9px; flex-wrap: wrap; }
.diagnostic-meta span { color: #8e5c2e; font-size: .75rem; text-transform: uppercase; }
.diagnostic-row pre { max-height: 180px; overflow: auto; margin: 10px 0 0; padding: 10px; background: #f5f7f8; color: #41515c; font: .76rem/1.45 ui-monospace, SFMono-Regular, Menlo, monospace; white-space: pre-wrap; overflow-wrap: anywhere; }
.settings-form { display: grid; gap: 16px; max-width: 620px; padding: 18px; border: 1px solid #d8e0e4; border-radius: 6px; background: #fff; }
.settings-form label { display: grid; gap: 7px; color: #42535d; font-size: .86rem; }
.settings-form input { min-height: 40px; padding: 0 10px; border: 1px solid #cbd5db; border-radius: 4px; color: #17232c; }
.settings-form select { min-height: 40px; padding: 0 10px; border: 1px solid #cbd5db; border-radius: 4px; background: #fff; color: #17232c; }
.settings-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.field-note { margin: -3px 0 0; color: #778690; font-size: .78rem; }
.transcript { min-height: 420px; max-height: calc(100vh - 260px); overflow-y: auto; padding: 24px 8px; border-top: 1px solid #d8dfe4; border-bottom: 1px solid #d8dfe4; }
.empty-state { min-height: 360px; display: grid; place-content: center; justify-items: center; color: #5d6d7b; text-align: center; }
.empty-mark { display: grid; place-items: center; width: 40px; height: 40px; margin-bottom: 16px; border: 1px solid #a7b6c0; border-radius: 50%; color: #3c7080; font-size: 1.5rem; }
.empty-state h2 { margin: 0; color: #1f2b35; font-size: 1.2rem; }
.empty-state p { max-width: 320px; margin: 8px 0 0; line-height: 1.5; }
.message-row { display: flex; gap: 12px; max-width: 78%; margin: 16px 0; }
.from-user { margin-left: auto; flex-direction: row-reverse; }
.avatar { flex: 0 0 30px; width: 30px; height: 30px; display: grid; place-items: center; border-radius: 50%; background: #d4e6e7; color: #27545e; font-size: .72rem; font-weight: 800; }
.from-user .avatar { background: #f5dfcf; color: #805033; }
.message-bubble { padding: 12px 15px; border: 1px solid #d8e0e4; border-radius: 6px 14px 14px 14px; background: #fff; box-shadow: 0 3px 12px rgb(39 55 66 / 5%); }
.from-user .message-bubble { border-color: #e7d2c0; border-radius: 14px 6px 14px 14px; background: #fffaf6; }
.message-bubble p { margin: 0; line-height: 1.55; white-space: pre-wrap; overflow-wrap: anywhere; }
.attachment-chip { display: inline-block; margin-top: 9px; color: #36707b; font-size: .75rem; }
.error-banner { margin: 0; padding: 10px 12px; border-left: 3px solid #bd584f; background: #fff3f1; color: #8b3933; font-size: .88rem; }
.composer { padding: 14px; border: 1px solid #cbd5db; border-radius: 8px; background: #fff; box-shadow: 0 8px 24px rgb(39 55 66 / 7%); }
.composer textarea { display: block; width: 100%; min-height: 56px; resize: vertical; border: 0; outline: 0; color: #17232c; line-height: 1.5; }
.composer textarea::placeholder { color: #9aa7b0; }
.composer-footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-top: 10px; }
.attachment-input { display: flex; align-items: center; gap: 7px; min-width: 0; color: #637582; }
.attachment-input > span { display: grid; place-items: center; width: 24px; height: 24px; border: 1px solid #b8c5cc; border-radius: 50%; color: #47717c; }
.attachment-input input { width: min(260px, 35vw); border: 0; outline: 0; color: #41515c; font-size: .8rem; }
.composer-actions { display: flex; gap: 8px; }
.send-button, .secondary-button { min-height: 36px; padding: 0 14px; border-radius: 5px; cursor: pointer; }
.send-button { border: 1px solid #326b75; background: #326b75; color: #fff; }
.send-button:disabled { cursor: not-allowed; opacity: .45; }
.secondary-button { border: 1px solid #bfccd2; background: #fff; color: #435661; }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0,0,0,0); white-space: nowrap; border: 0; }
@media (max-width: 640px) {
  .shell { width: min(100% - 20px, 940px); padding-top: 18px; gap: 12px; }
  .transcript { min-height: 0; max-height: calc(100vh - 230px); padding: 16px 2px; }
  .message-row { max-width: 92%; }
  .control-panel { min-height: 0; padding: 16px 2px; }
  .composer-footer { align-items: flex-end; }
  .attachment-input input { width: min(180px, 45vw); }
  .settings-grid { grid-template-columns: 1fr; }
  .send-button, .secondary-button { min-height: 40px; }
}
</style>
