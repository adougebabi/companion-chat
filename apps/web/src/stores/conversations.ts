import { defineStore } from "pinia";
import {
  BrowserClient,
  type BrowserConversation,
  type BrowserMessage,
  type BrowserTurnEvent,
} from "@fluctlight/browser-client";

const client = new BrowserClient(import.meta.env.VITE_BFF_ORIGIN ?? "");

type StreamPayload = { text?: string; message?: BrowserMessage; message_id?: string; code?: string };

function createLocalMessage(conversationId: string, text: string, sequence: number, authorActorId = "human") : BrowserMessage {
  return {
    id: `local-${crypto.randomUUID()}`,
    conversationId,
    sequence,
    authorActorId,
    kind: "user",
    text,
    attachmentRefs: [],
    createdAt: new Date().toISOString(),
  };
}

export const useConversationStore = defineStore("conversations", {
  state: () => ({
    conversation: null as BrowserConversation | null,
    messages: [] as BrowserMessage[],
    authenticated: null as boolean | null,
    authLoading: false,
    authError: "" as string,
    loading: false,
    sending: false,
    error: "" as string,
    attachmentRef: "",
    abortController: null as AbortController | null,
  }),
  getters: {
    hasConversation: (state) => Boolean(state.conversation?.id),
  },
  actions: {
    async initialize() {
      this.authLoading = true;
      this.authError = "";
      try {
        const session = await client.session();
        this.authenticated = session.authenticated;
        if (this.authenticated) await this.openConversation();
      } catch {
        if (this.authenticated !== true) {
          this.authenticated = false;
          this.authError = "The Core platform is unavailable.";
        }
      } finally {
        this.authLoading = false;
      }
    },
    async login(password: string) {
      this.authLoading = true;
      this.authError = "";
      try {
        const session = await client.login(password);
        this.authenticated = session.authenticated;
        if (this.authenticated) await this.openConversation();
      } catch {
        if (this.authenticated !== true) {
          this.authenticated = false;
          this.authError = "The password was not accepted.";
        } else {
          this.error = "The conversation could not be loaded.";
        }
      } finally {
        this.authLoading = false;
      }
    },
    async logout() {
      try {
        await client.logout();
      } finally {
        this.authenticated = false;
        this.conversation = null;
        this.messages = [];
      }
    },
    async openConversation() {
      if (this.authenticated === false) return;
      this.loading = true;
      this.error = "";
      try {
        if (!this.conversation) {
          const page = await client.createConversation({ title: "New conversation" });
          this.conversation = page.conversation;
          this.messages = page.messages;
        } else {
          const page = await client.messages(this.conversation.id);
          this.conversation = page.conversation;
          this.messages = page.messages;
        }
      } catch {
        this.error = "The conversation could not be loaded.";
      } finally {
        this.loading = false;
      }
    },
    async send(text: string) {
      const normalized = text.trim();
      if (!normalized || !this.conversation || this.sending) return;
      this.error = "";
      this.sending = true;
      this.abortController = new AbortController();
      const userMessage = createLocalMessage(this.conversation.id, normalized, this.messages.length + 1);
      this.messages.push(userMessage);
      let assistantDraft: BrowserMessage | null = null;
      try {
        const response = await client.turn(
          this.conversation.id,
          {
            text: normalized,
            attachmentRefs: this.attachmentRef ? [this.attachmentRef] : [],
            idempotencyKey: `turn-${crypto.randomUUID()}`,
          },
          this.abortController.signal,
        );
        if (!response.body) throw new Error("stream_missing");
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = "";
        let assistantText = "";
        let expectedSequence = 0;
        const applyEvent = (event: BrowserTurnEvent) => {
          if (event.sequence !== expectedSequence) throw new Error("turn_sequence_invalid");
          expectedSequence += 1;
          const payload = event.payload as StreamPayload;
          if (event.type === "token") {
            assistantText += payload.text ?? "";
            if (!assistantDraft) {
              assistantDraft = {
                id: `stream-${crypto.randomUUID()}`,
                conversationId: this.conversation!.id,
                sequence: this.messages.length + 1,
                authorActorId: "fluctlight",
                kind: "assistant",
                text: assistantText,
                attachmentRefs: [],
                createdAt: new Date().toISOString(),
              };
              this.messages.push(assistantDraft);
            } else {
              assistantDraft.text = assistantText;
            }
          }
          if (event.type === "message" && payload.message) this.messages.push(payload.message);
          if (event.type === "error") throw new Error(payload.code ?? "turn_failed");
        };
        while (true) {
          const next = await reader.read();
          if (next.done) break;
          buffer += decoder.decode(next.value, { stream: true });
          const lines = buffer.split("\n");
          buffer = lines.pop() ?? "";
          for (const line of lines) {
            if (!line.trim()) continue;
            applyEvent(JSON.parse(line) as BrowserTurnEvent);
          }
        }
        buffer += decoder.decode();
        if (buffer.trim()) {
          applyEvent(JSON.parse(buffer) as BrowserTurnEvent);
        }
        const page = await client.messages(this.conversation.id);
        this.conversation = page.conversation;
        this.messages = page.messages;
        this.attachmentRef = "";
      } catch (error) {
        if (error instanceof DOMException && error.name === "AbortError") {
          if (assistantDraft) this.messages = this.messages.filter((message) => message.id !== assistantDraft?.id);
        } else {
          this.error = "The turn could not be completed. Your message is saved.";
        }
      } finally {
        this.abortController = null;
        this.sending = false;
      }
    },
    cancel() {
      this.abortController?.abort();
    },
  },
});
