import assert from "node:assert/strict";
import test from "node:test";
import { validateFields } from "../src/lib/form-validation.ts";
import { parseWorkflowTemplate, mediaSettingsPatch, autonomySettingsPatch, workflowSummary } from "../src/lib/configuration-forms.ts";
import { workflowPresentation } from "../src/lib/diagnostic-presentation.ts";
import { userError } from "../src/lib/user-feedback.ts";
import { backgroundDraft, backgroundFromDraft, actorUserBackgroundFields } from "../src/lib/actor-user-background.ts";
test("field validation rejects whitespace, invalid numbers and timezone aliases while preserving false and zero", () => {
  const errors = validateFields([{
    id: "name",
    label: "名称",
    value: "  ",
    required: true
  }, {
    id: "queue",
    label: "并发",
    value: 1.5,
    kind: "number",
    integer: true,
    min: 1,
    max: 8
  }, {
    id: "boolean-number",
    label: "数字",
    value: true,
    kind: "number"
  }, {
    id: "url",
    label: "地址",
    value: "file:///private/example",
    kind: "url"
  }, {
    id: "zone",
    label: "时区",
    value: "Local",
    kind: "timezone"
  }, {
    id: "zero",
    label: "预算",
    value: 0,
    required: true,
    kind: "number",
    min: 0
  }, {
    id: "false",
    label: "已确认",
    value: false,
    required: true
  }]);
  assert.deepEqual(Object.keys(errors), ["name", "queue", "boolean-number", "url", "zone"]);
  assert.deepEqual(validateFields([{
    id: "password",
    label: "密码",
    value: "  abc ",
    trim: false,
    minLength: 6
  }]), {});
});
test("workflow imports keep numeric placeholders compatible and reject invalid roots", () => {
  const value = parseWorkflowTemplate('{"1":{"class_type":"Generate","inputs":{"seed":{{seed}},"weight":{{chest_lora_weight}},"text":"{{prompt}}"}}}');
  assert.equal(value["1"].inputs.seed, "{{seed}}");
  assert.equal(value["1"].inputs.weight, "{{chest_lora_weight}}");
  assert.equal(value["1"].inputs.text, "{{prompt}}");
  assert.deepEqual(workflowSummary(value), {
    nodes: 1,
    placeholders: ["{{seed}}", "{{chest_lora_weight}}", "{{prompt}}"]
  });
  for (const input of ["[]", "null", "{}", "{broken}"]) assert.throws(() => parseWorkflowTemplate(input));
});
test("guided media saves preserve branched workflows, aliases and unknown fields without mutating the original", () => {
  const original = {
    base_url: "http://old.test",
    workflow: {
      old: true
    },
    media_workflows: {
      face: {
        face: true
      }
    },
    visual_identity_workflows: {
      sheet: {
        sheet: true
      }
    },
    visual_identity_workflow: {
      seed: {
        oldSeed: true
      }
    },
    custom: {
      keep: [1, 2]
    }
  };
  const saved = mediaSettingsPatch(original, "http://new.test", {
    new: true
  });
  assert.equal(saved.baseUrl, "http://new.test");
  assert.deepEqual(saved.workflow, {
    new: true
  });
  for (const key of ["base_url", "media_workflows", "visual_identity_workflows", "visual_identity_workflow", "custom"]) assert.deepEqual(saved[key], original[key]);
  saved.custom.keep.push(3);
  assert.deepEqual(original.custom.keep, [1, 2]);
  const cleared = mediaSettingsPatch(original, "http://new.test", {
    new: true
  }, undefined, true);
  assert.equal(cleared.visual_identity_workflow, undefined);
  assert.deepEqual(cleared.visual_identity_workflows, original.visual_identity_workflows);
});
test("guided autonomy updates preserve advanced values and configured custom actions", () => {
  const original = {
    quiet_hours: {
      from: "22:00",
      to: "08:00"
    },
    custom: {
      keep: true
    },
    topic_suppression_seconds: 1800
  };
  const saved = autonomySettingsPatch(original, {
    mode: "paused",
    actions: ["custom_action"],
    budget: 0
  });
  assert.deepEqual(saved.quiet_hours, original.quiet_hours);
  assert.deepEqual(saved.custom, original.custom);
  assert.equal(saved.topic_suppression_seconds, 1800);
  assert.equal(saved.budget_remaining, 0);
  assert.deepEqual(saved.allowed_actions, ["custom_action"]);
});
test("background round trips preserve all supported fields and the unknown/no meeting distinction", () => {
  const background = {
    name: "样本",
    occupation: "编辑",
    background: "资料",
    interests: "画画",
    preferences: "安静",
    location: "上海",
    location_scope: "同城",
    timezone: "Asia/Shanghai",
    relationship_distance: "线上",
    meeting_confirmed: false
  };
  assert.deepEqual(backgroundFromDraft(backgroundDraft(background)), {
    ...Object.fromEntries(actorUserBackgroundFields.map(({
      key
    }) => [key, null])),
    ...background
  });
  assert.equal(backgroundFromDraft({
    meeting_confirmed: "unknown"
  }).meeting_confirmed, null);
});
test("workflow rows distinguish executions and do not fabricate missing metadata", () => {
  const first = workflowPresentation({
    workflow_id: "same",
    run_id: "run-a",
    status: "completed",
    start_time: "2026-10-13T01:00:00Z"
  });
  const second = workflowPresentation({
    execution: {
      workflowId: "same",
      runId: "run-b"
    },
    status: "WORKFLOW_EXECUTION_STATUS_RUNNING"
  });
  assert.notEqual(first.key, second.key);
  assert.equal(second.status, "running");
  assert.equal(second.startedAt, "");
  assert.equal(workflowPresentation({
    workflow_id: "legacy",
    status: 3
  }).status, "unknown");
});
test("user-facing errors explain the action while technical codes remain separately available", () => {
  assert.match(userError("回复未完成：agent_run_failed"), /重试/);
  assert.doesNotMatch(userError("回复未完成：agent_run_failed"), /agent_run_failed/);
  assert.match(userError("provider_request_failed"), /模型服务/);
  assert.match(userError("agent_partial_committed"), /不能直接重试/);
});
