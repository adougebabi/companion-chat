import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { configurationFields, conversationReplyDisplayValue } from "../src/v3/proto/components/configuration-fields.js";
import { Field } from "../src/v3/proto/components/forms.js";

const replySettingsSource = await readFile(new URL("../src/components/settings/ReplyVisionSettings.vue", import.meta.url), "utf8");
const configurationSource = await readFile(new URL("../src/approved-ui/pages/ManagementPage.mjs", import.meta.url), "utf8");

test("unset conversation reply mode displays fact_only and an explicit legacy mode stays", () => {
  assert.match(replySettingsSource, /mode:"fact_only"/);
  assert.match(configurationSource, /conversationReplyDisplayValue/);
  assert.equal(conversationReplyDisplayValue(undefined).mode, "fact_only");
  assert.equal(conversationReplyDisplayValue({}).mode, "fact_only");
  assert.equal(conversationReplyDisplayValue({ temperature: 0.2 }).mode, "fact_only");
  const saved = conversationReplyDisplayValue({ mode: "legacy", temperature: 0.2 });
  assert.equal(saved.mode, "legacy");
  assert.equal(saved.temperature, 0.2);
  const modeField = configurationFields["conversation.reply"].find((field) => field.key === "mode");
  const unsetMarkup = Field({ label: modeField.label, name: modeField.key, value: conversationReplyDisplayValue({}).mode, type: "select", options: modeField.options });
  assert.match(unsetMarkup, /<option value="fact_only" selected>/);
  assert.doesNotMatch(unsetMarkup, /<option value="legacy" selected>/);
  const legacyMarkup = Field({ label: modeField.label, name: modeField.key, value: saved.mode, type: "select", options: modeField.options });
  assert.match(legacyMarkup, /<option value="legacy" selected>/);
  assert.doesNotMatch(legacyMarkup, /<option value="fact_only" selected>/);
});
