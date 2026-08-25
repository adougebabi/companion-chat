import { defineStore } from "pinia";
import {
  BrowserClient,
  type BrowserDiagnosticEvent,
  type BrowserSafeSettings,
} from "@fluctlight/browser-client";

const client = new BrowserClient(import.meta.env.VITE_BFF_ORIGIN ?? "");

export const useControlCenterStore = defineStore("control-center", {
  state: () => ({
    fluctlights: [] as Array<{ id: string; identity: Record<string, unknown>; status: string }>,
    diagnostics: [] as BrowserDiagnosticEvent[],
    settings: null as BrowserSafeSettings | null,
    loading: false,
    saving: false,
    error: "",
  }),
  actions: {
    async loadFluctlights() {
      try {
        this.fluctlights = await client.listFluctlights();
      } catch {
        this.error = "Fluctlights are unavailable.";
      }
    },
    async createFluctlight(name: string) {
      this.error = "";
      try {
        await client.createFluctlight({ name });
        await this.loadFluctlights();
      } catch {
        this.error = "Fluctlight could not be created.";
      }
    },
    async loadDiagnostics() {
      this.loading = true;
      this.error = "";
      try {
        this.diagnostics = await client.diagnostics({ limit: 100 });
      } catch {
        this.error = "Diagnostics are only available to the authenticated Owner.";
      } finally {
        this.loading = false;
      }
    },
    async clearDiagnostics() {
      this.error = "";
      try {
        await client.clearDiagnostics();
        this.diagnostics = [];
      } catch {
        this.error = "Diagnostics could not be cleared.";
      }
    },
    async loadSettings() {
      this.loading = true;
      this.error = "";
      try {
        this.settings = await client.settings();
      } catch {
        this.error = "Settings are unavailable.";
      } finally {
        this.loading = false;
      }
    },
    async saveSettings(values: Record<string, unknown>, secret?: string) {
      this.saving = true;
      this.error = "";
      try {
        this.settings = await client.updateSettings({
          values,
          secrets: secret ? { "provider:primary": secret } : {},
        });
      } catch {
        this.error = "Settings could not be saved.";
      } finally {
        this.saving = false;
      }
    },
    async configureProvider(input: {
      endpointId: string;
      kind: string;
      baseUrl: string;
      secretPurpose: string;
      role: string;
      modelId: string;
      tokenBudget: number;
      timeoutSeconds: number;
    }) {
      this.saving = true;
      this.error = "";
      try {
        await client.configureProviderEndpoint({
          endpointId: input.endpointId,
          kind: input.kind,
          baseUrl: input.baseUrl,
          secretPurpose: input.secretPurpose,
        });
        await client.configureModelRole({
          role: input.role,
          endpointId: input.endpointId,
          modelId: input.modelId,
          tokenBudget: input.tokenBudget,
          timeoutSeconds: input.timeoutSeconds,
        });
      } catch {
        this.error = "Provider preflight failed; settings were not activated.";
      } finally {
        this.saving = false;
      }
    },
  },
});
