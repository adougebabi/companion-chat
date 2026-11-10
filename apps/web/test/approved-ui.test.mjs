import test from "node:test";
import assert from "node:assert/strict";
import { configurationPlan } from "../src/approved-ui/configuration-plan.mjs";
import { acknowledgeConfiguration, clearAcknowledgedCredentials } from "../src/approved-ui/configuration-ack.mjs";
import { connectionDraftDirty, restoreConnectionDraft } from "../src/approved-ui/configuration-draft.mjs";
import { stagedCommand } from "../src/approved-ui/staged-command.mjs";
import { approvedHref, approvedRoute } from "../src/approved-ui/routing.mjs";
const storage = () => {
  const values = new Map();
  return {
    values,
    getItem: k => values.get(k) ?? null,
    setItem: (k, v) => values.set(k, String(v)),
    removeItem: k => values.delete(k)
  };
};
const form = () => ({
  catalog: {
    versions: {
      "setting:test": 3,
      "endpoint:remote": 8
    },
    effective_values: {
      test: {
        enabled: false
      }
    },
    endpoints: [],
    bindings: []
  },
  settings: [{
    fields: [{
      label: "开关",
      _settingKey: "test",
      _key: "enabled",
      type: "checkbox",
      value: true,
      _initial: false
    }]
  }],
  endpoints: [],
  bindings: [],
  quiet: {
    enabled: false,
    start: "",
    end: "",
    days: ["周日", "周一", "周二", "周三", "周四", "周五", "周六"],
    behavior: "暂停主动联系",
    character: "全部角色"
  },
  characters: []
});
test("plugin settings save a complete pair and retain changes made during acknowledgement", () => {
  const input = form();
  input.catalog.effective_values = {"plugins.domains": {image: true, moment: true}};
  input.settings = [{fields: [
    {label:"图片", _settingKey:"plugins.domains", _key:"image", type:"checkbox", value:false, _initial:true},
    {label:"动态", _settingKey:"plugins.domains", _key:"moment", type:"checkbox", value:true, _initial:true},
  ]}];
  const operation = configurationPlan(input).find(item => item.operation === "settings");
  assert.deepEqual(operation.body.values["plugins.domains"], {image:false, moment:true});
  input.settings[0].fields[1].value = false;
  const versions = {};
  acknowledgeConfiguration({...input, versions}, operation, {applied_versions:{"plugins.domains":1}});
  assert.equal(input.settings[0].fields[1].value, false);
  assert.equal(input.settings[0].fields[1]._initial, true);
  assert.deepEqual(configurationPlan(input)[0].body.values["plugins.domains"], {image:false, moment:false});
});
test("an already-cleared override cannot be recreated by a recovered draft", () => {
  const input = form();
  input.settings = [];
  input.endpoints = [{id:'service',name:'service',protocol:'openai-compatible',url:'https://model.test',secret:'',clearSecret:false,_source:{id:'service',kind:'openai-compatible',base_url:'https://model.test'}}];
  input.bindings = [{role:'agent:reflection:reflection_proposal_v2',endpoint:'service',model:'old',timeout:30,clearOverride:true}];
  assert.deepEqual(configurationPlan(input), []);
});
test("model sampling saves zero, clears one field and survives endpoint rebind", () => {
  const input = form();input.settings=[];
  const endpoint={id:'service',kind:'openai-compatible',base_url:'https://model.test'};
  const source={role:'agent:reflection',endpoint_id:'service',model_id:'model',timeout_seconds:30,temperature:0.5,top_p:0.8};
  input.catalog.endpoints=[endpoint];input.catalog.bindings=[source];
  input.endpoints=[{id:'service',name:'service',protocol:endpoint.kind,url:endpoint.base_url,secret:'',clearSecret:false,_source:endpoint}];
  input.bindings=[{role:source.role,name:'reflection',endpoint:'service',model:'model',timeout:30,temperature:0.5,topP:0.8,_source:source}];
  assert.deepEqual(configurationPlan(input),[]);
  input.bindings[0].temperature=0;input.bindings[0].topP='';
  let binding=configurationPlan(input).find(operation=>operation.operation==='binding');
  assert.equal(binding.body.temperature,0);assert.equal(binding.body.topP,null);
  input.bindings[0].temperature=0.5;input.bindings[0].topP=0.8;input.endpoints[0].url='https://new-model.test';
  binding=configurationPlan(input).find(operation=>operation.operation==='binding');
  assert.equal(binding.body.temperature,0.5);assert.equal(binding.body.topP,0.8);
  const other={id:'other',kind:'openai-compatible',base_url:'https://other-model.test'};
  input.catalog.endpoints.push(other);input.endpoints.push({id:'other',name:'other',protocol:other.kind,url:other.base_url,secret:'',clearSecret:false,_source:other});
  input.bindings[0].endpoint='other';
  binding=configurationPlan(input).find(operation=>operation.operation==='binding');
  assert.equal(binding.body.endpointId,'other');assert.equal(binding.body.temperature,0.5);assert.equal(binding.body.topP,0.8);
  input.bindings[0].topP=0;assert.throws(()=>configurationPlan(input),/Top P/);
});
test("older connection drafts preserve new sampling fields on restore", () => {
  const catalog={endpoints:[{id:'service',kind:'openai-compatible',base_url:'https://model.test'}],bindings:[{role:'generic_llm',endpoint_id:'service',model_id:'model',timeout_seconds:30,temperature:0.4,top_p:0.9}],versions:{}};
  const old={version:1,versions:{},endpoints:[{id:'service',name:'service',protocol:'openai-compatible',url:'https://model.test'}],bindings:[{role:'generic_llm',endpoint:'service',model:'model',timeout:30}]};
  assert.equal(connectionDraftDirty(old,catalog),false);
  old.bindings[0].model='new-model';const restored=restoreConnectionDraft(JSON.stringify(old),catalog);
  assert.equal(restored.dirty,true);assert.equal(Object.hasOwn(restored.bindings[0],'temperature'),false);assert.equal(Object.hasOwn(restored.bindings[0],'topP'),false);
});
test("configuration validates the entire form before writing and maps temporary endpoint IDs", () => {
  const input = form();
  input.endpoints = [{
    id: "new-1",
    name: "new-server",
    protocol: "openai-compatible",
    url: "https://model.test/v1",
    secret: "",
    clearSecret: false
  }];
  input.bindings = [{
    name: "通用",
    role: "generic_llm",
    endpoint: "new-1",
    model: "model",
    tokens: 1000,
    timeout: 60,
    window: 8192,
    safetyMargin: 512
  }];
  const plan = configurationPlan(input);
  assert.equal(plan[0].body.endpointId, "new-server");
  assert.equal(plan[1].body.endpointId, "new-server");
  assert.equal(plan[1].body.maxInputTokens, undefined);
  assert.equal(plan[0].formId, "new-1");
  input.settings[0].fields.push({
    label: "配置",
    _settingKey: "test",
    _key: "json",
    type: "json",
    value: "{broken",
    _definition: {}
  });
  assert.throws(() => configurationPlan(input), /有效 JSON/);
  input.settings[0].fields.pop();
  input.endpoints[0].url = "https://user:secret@model.test/v1";
  assert.throws(() => configurationPlan(input), /嵌入凭据/);
});
test("acknowledgement advances only receipt versions and preserves newer input and revert changes", () => {
  const input = form(),
    versions = {
      ...input.catalog.versions
    },
    operation = configurationPlan(input)[0];
  input.settings[0].fields[0].value = false;
  acknowledgeConfiguration({
    ...input,
    versions
  }, operation, {
    applied_versions: {
      "setting:test": 4
    }
  });
  assert.equal(versions["endpoint:remote"], 8);
  assert.equal(versions["setting:test"], 4);
  assert.equal(input.settings[0].fields[0].value, false);
  assert.equal(input.settings[0].fields[0]._initial, true);
  assert.equal(input.catalog.effective_values.test.enabled, true);
  assert.equal(configurationPlan(input)[0].body.values.test.enabled, false);
});
test("endpoint acknowledgements preserve edits made in flight and rebind temporary IDs", () => {
  const input = form();
  input.endpoints = [{
    id: "temp",
    name: "new-server",
    protocol: "openai-compatible",
    url: "https://model.test/v1",
    secret: "",
    clearSecret: false
  }];
  input.bindings = [{
    role: "generic_llm",
    endpoint: "temp",
    model: "draft"
  }];
  const operation = {
    operation: "endpoint",
    formId: "temp",
    body: {
      endpointId: "new-server",
      kind: "openai-compatible",
      baseUrl: "https://model.test/v1/",
      secretPurpose: "provider:new-server"
    }
  };
  input.endpoints[0].name = "newer-name";
  input.endpoints[0].url = "https://new.test/v1";
  acknowledgeConfiguration({
    ...input,
    versions: {}
  }, operation, {
    applied_versions: {
      "endpoint:new-server": 1
    }
  });
  assert.equal(input.endpoints[0].name, "newer-name");
  assert.equal(input.endpoints[0].url, "https://new.test/v1");
  assert.equal(input.endpoints[0]._source.base_url, "https://model.test/v1");
  assert.equal(input.bindings[0].endpoint, "new-server");
});
test("world stages recover immutable apply body and key after lost response and refresh", async () => {
  const saved = storage(),
    calls = [];
  let previews = 0;
  const create = () => {
    previews++;
    return {
      runtimeWorldId: "world-A",
      expectedRuntimeRevision: 4
    };
  };
  const execute = async (body, key) => {
    calls.push({
      body,
      key
    });
    if (calls.length === 1) throw new Error("lost response");
    return {
      revision: 5
    };
  };
  await assert.rejects(stagedCommand({
    storage: saved,
    key: "operation"
  }).stage("apply", create, execute), /lost response/);
  const next = stagedCommand({
    storage: saved,
    key: "operation"
  });
  await next.stage("apply", () => ({
    runtimeWorldId: "world-B",
    expectedRuntimeRevision: 9
  }), execute);
  assert.equal(previews, 1);
  assert.deepEqual(calls[0], calls[1]);
  assert.equal(JSON.stringify([...saved.values.values()]).includes('"revision":5'), false);
  next.finish();
  assert.equal(saved.values.size, 0);
});
test("canonical React routes preserve their targets and retired legacy URLs are rejected", () => {
  for (const page of ["聊天", "此刻", "日程", "目标", "衣柜", "日记", "记忆", "世界", "人物关系", "媒体"]) {
    const route = approvedRoute(approvedHref(page, "real-role"));
    assert.equal(route.page, page);
    assert.equal(route.fid, page === "媒体" ? null : "real-role");
  }
  assert.equal(approvedRoute("#/c/real-role/growth/memory/memory-A").feature.params.memId, "memory-A");
  assert.equal(approvedRoute("#/studio?legacy=1").removed, true);
  assert.equal(approvedRoute("#/auth").page, "登录");
});
test("unconfigured optional form values do not turn a sibling save into empty configuration writes", () => {
  const input = form();
  input.settings.push({
    fields: [{
      label: "模型版本",
      _settingKey: "kev",
      _key: "model_revision",
      type: "text",
      value: "",
      _initial: undefined
    }, {
      label: "工作流",
      _settingKey: "media.comfyui",
      _key: "workflow",
      type: "json",
      value: "",
      _initial: undefined,
      _definition: {}
    }]
  });
  const plan = configurationPlan(input);
  assert.deepEqual(Object.keys(plan[0].body.values), ["test"]);
});
test("character quiet policy keeps its own revision and validates weekdays and clocks before writes", () => {
  const input = form();
  input.characters = [{
    id: "role-B",
    name: "B"
  }];
  input.quiet = {
    ...input.quiet,
    character: "B",
    enabled: true,
    start: "22:00",
    end: "08:00",
    days: ["周一", "周五"],
    _policy: {
      fluctlight_id: "role-B",
      revision: 8,
      quiet_hours: {}
    }
  };
  const operation = configurationPlan(input).find(w => w.operation === "autonomy");
  assert.equal(operation.fid, "role-B");
  assert.equal(operation.body.expectedRevision, 8);
  assert.deepEqual(operation.body.quietHours.days, [1, 5]);
  input.quiet.start = "99:99";
  assert.throws(() => configurationPlan(input), /安静日期/);
});
test("secret acknowledgements clear only the acknowledged purpose and retain sibling/newer drafts", () => {
  const endpoints = [{
      id: "a",
      name: "a",
      secret: "A",
      clearSecret: false
    }, {
      id: "b",
      name: "b",
      secret: "B",
      clearSecret: true
    }],
    submitted = structuredClone(endpoints);
  clearAcknowledgedCredentials(endpoints, submitted, {
    secrets: {
      "provider:a": "A"
    }
  });
  assert.equal(endpoints[0].secret, "");
  assert.equal(endpoints[1].secret, "B");
  assert.equal(endpoints[1].clearSecret, true);
  endpoints[1].secret = "newer B";
  clearAcknowledgedCredentials(endpoints, submitted, {
    secrets: {
      "provider:b": "B"
    }
  });
  assert.equal(endpoints[1].secret, "newer B");
});
