import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";
import { proxy, snapshot, subscribe } from "valtio/vanilla";
import { act, mountReact, installDOM } from "./helpers/react-dom.mjs";
const environment = installDOM();
const vite = await createServer({
  root: fileURLToPath(new URL("../", import.meta.url)),
  appType: "custom",
  logLevel: "silent",
  server: { middlewareMode: true },
});
test.after(async () => {
  await vite.close();
  environment.restore();
});
const load = (path) => vite.ssrLoadModule("/src/" + path);
const button = (text, root = document) =>
  [...root.querySelectorAll("button")].find(
    (node) => node.textContent.trim() === text,
  );
function storeFacade(state) {
  return {
    state,
    subscribe: (listener) => subscribe(state, listener),
    getSnapshot: () => snapshot(state),
  };
}

test("React settings stay busy through receipt readback and preserve newer model input", async () => {
  const { default: Management } = await load(
    "react-app/pages/ManagementPage.jsx",
  );
  let finishWrite,
    finishRead,
    calls = 0;
  const catalog = {
    endpoints: [
      {
        id: "service",
        base_url: "https://model.example",
        kind: "openai-compatible",
      },
    ],
    bindings: [
      {
        role: "generic_llm",
        endpoint_id: "service",
        model_id: "old",
        timeout_seconds: 120,
      },
    ],
    versions: {},
    effective_values: {},
  };
  const w = {
    ...storeFacade(
      proxy({
        session: { actorId: "owner-react" },
        managementTab: "配置中心",
        characters: [],
        operations: [],
        managementSection: "",
        runtimeCatalog: {},
      }),
    ),
    services: {
      configurationClient: {
        read: async () => structuredClone(catalog),
        models: async () => ({ models: [] }),
        catalog: async () => ({ agents: [] }),
        binding: async () => {
          calls++;
          return new Promise((resolve) => (finishWrite = resolve));
        },
        command: async () => new Promise((resolve) => (finishRead = resolve)),
      },
      diagnosticsClient: { page: async () => ({ items: [] }) },
    },
    notify() {},
    navigatePath() {},
  };
  const view = await mountReact(vite, Management, { workspace: w });
  try {
    const model = view.host.querySelector('input[aria-label="模型标识"]');
    const choose = async value => view.input(model,value);
    assert.ok(model);
    await choose("new");
    const form = view.host.querySelector("form");
    await act(async () =>
      form.dispatchEvent(
        new window.Event("submit", { bubbles: true, cancelable: true }),
      ),
    );
    await view.flush();
    const save = () => view.host.querySelector('[data-testid="config-save"]'),
      status = () =>
        view.host.querySelector('[data-testid="config-save-status"]');
    assert.equal(calls, 1);
    assert.equal(save().disabled, true);
    assert.match(status().textContent, /等待服务端确认/);
    await choose("newer-unsaved");
    await act(async () => finishWrite({}));
    await view.flush();
    assert.equal(save().disabled, true);
    assert.equal(typeof finishRead, "function");
    await act(async () =>
      finishRead({
        status: "applied",
        applied_versions: { "binding:generic_llm": 1 },
      }),
    );
    await view.flush();
    assert.equal(save().disabled, false);
    assert.match(status().textContent, /已保存/);
    assert.equal(model.value, "newer-unsaved");
  } finally {
    await view.dispose();
  }
});

test("React model settings save a task override and restore inheritance", async () => {
  const { default: Management } = await load("react-app/pages/ManagementPage.jsx");
  const role = "agent:reflection:reflection_proposal_v2";
  const server = {
    endpoints:[{id:"service",base_url:"https://model.example",kind:"openai-compatible"}],
    bindings:[{role:"generic_llm",endpoint_id:"service",model_id:"generic",timeout_seconds:120},{role,endpoint_id:"service",model_id:"special",timeout_seconds:30}],
    versions:{["binding:"+role]:3,"endpoint:service":1},effective_values:{},
    agent_model_routes:[{key:role,agent_id:"reflection",variant:"reflection_proposal_v2",writable:true,source:"variant",state:"configured",model_id:"special"}],
  };
  const submitted=[];
  let finishClear,clearCount=0,loseReceipt=false,loseCatalogRead=false,lastRole='';
  const w={...storeFacade(proxy({session:{actorId:"owner-agent-react"},managementTab:"配置中心",characters:[],operations:[],managementSection:"模型用途",runtimeCatalog:{}})),
    services:{configurationClient:{
      read:async()=>{if(loseCatalogRead){loseCatalogRead=false;throw Error('catalog unavailable');}return structuredClone(server);},models:async()=>({models:[]}),catalog:async()=>({agents:[]}),save:async body=>{Object.assign(server.effective_values,body.values||{});},
      binding:async body=>{
        submitted.push(structuredClone(body));
        lastRole=body.role;
        if(body.clearOverride){if(++clearCount===1)await new Promise(resolve=>finishClear=resolve);server.bindings=server.bindings.filter(binding=>binding.role!==role);server.agent_model_routes[0]={...server.agent_model_routes[0],source:"default",model_id:server.bindings.find(binding=>binding.role==='generic_llm').model_id};if(clearCount===2)loseReceipt=true;}
        else {const previous=server.bindings.find(binding=>binding.role===body.role);server.bindings=[...server.bindings.filter(binding=>binding.role!==body.role),{role:body.role,endpoint_id:body.endpointId,model_id:body.modelId,timeout_seconds:body.timeoutSeconds,temperature:Object.hasOwn(body,'temperature')?body.temperature:previous?.temperature??null,top_p:Object.hasOwn(body,'topP')?body.topP:previous?.top_p??null}];if(body.role===role)server.agent_model_routes[0]={...server.agent_model_routes[0],source:'variant',model_id:body.modelId};else if(server.agent_model_routes[0].source==='default')server.agent_model_routes[0].model_id=body.modelId;}
        server.versions["binding:"+body.role]=(server.versions["binding:"+body.role]||0)+1;
        if(body.modelId==='readback-failure')loseCatalogRead=true;
      },
      command:async()=>{if(loseReceipt){loseReceipt=false;throw Error('receipt unavailable');}return {status:"applied",applied_versions:{["binding:"+lastRole]:server.versions["binding:"+lastRole]}};},recover:async()=>({status:'applied'}),
    },diagnosticsClient:{page:async()=>({items:[]})}},notify(){},navigatePath(){}};
  const view=await mountReact(vite,Management,{workspace:w});
  try {
    await view.click(button("按 Agent 或任务变体覆盖",view.host));
    const trigger=view.host.querySelector('[aria-label="选择 Agent 模型任务"]');
    await view.click(trigger);
    const option=[...document.querySelectorAll('[role="option"]')].find(node=>node.textContent.trim()==='记忆与自我反思');
    assert.ok(option,'task variant was not offered by the real React page');
    await view.click(option);
    const row=()=>view.host.querySelector('section[aria-label="记忆与自我反思"]');
    assert.match(row().querySelector('[data-testid="agent-model-source"]').textContent,/任务变体覆盖.*special/);
    assert.ok(row().compareDocumentPosition(view.host.querySelector('section[aria-label="思考与通用分析"]')) & window.Node.DOCUMENT_POSITION_FOLLOWING);
    await view.input(row().querySelector('input[aria-label="模型标识"]'),'updated');
    await view.input(row().querySelector('input[aria-label="温度（可选）"]'),'0');
    await view.input(row().querySelector('input[aria-label="Top P（可选）"]'),'0.75');
    const submit=async()=>{await act(async()=>view.host.querySelector('form').dispatchEvent(new window.Event('submit',{bubbles:true,cancelable:true})));await view.flush();};
    await submit();
    assert.equal(submitted[0].role,role);assert.equal(submitted[0].modelId,'updated');assert.equal(submitted[0].temperature,0);assert.equal(submitted[0].topP,0.75);
    assert.match(row().querySelector('[data-testid="agent-model-source"]').textContent,/updated/);
    await view.click(button('恢复继承',row()));
    await submit();
    assert.equal(submitted[1].clearOverride,true);
    assert.deepEqual(Object.keys(submitted[1]).sort(),['clearOverride','expectedVersions','idempotencyKey','role']);
    assert.deepEqual(submitted[1].expectedVersions,{['binding:'+role]:4});
    const generic=view.host.querySelector('section[aria-label="思考与通用分析"] input[aria-label="模型标识"]');
    await view.input(generic,'newer-unsaved');
    await act(async()=>finishClear());await view.flush();
    assert.match(row().querySelector('[data-testid="agent-model-source"]').textContent,/默认用途.*generic/);
    assert.equal(generic.value,'newer-unsaved');
    await view.input(generic,'new-default');await submit();
    assert.match(row().querySelector('[data-testid="agent-model-source"]').textContent,/默认用途.*new-default/);
    await view.click(row().querySelector('[aria-label="模型服务"]'));
    const service=[...document.querySelectorAll('[role="option"]')].find(node=>node.textContent.trim()==='service');
    assert.ok(service);await view.click(service);
    await view.input(row().querySelector('input[aria-label="模型标识"]'),'new-override');
    await submit();
    assert.equal(submitted[3].role,role);assert.equal(submitted[3].modelId,'new-override');assert.equal(submitted[3].endpointId,'service');
    assert.match(row().querySelector('[data-testid="agent-model-source"]').textContent,/任务变体覆盖.*new-override/);
    await view.click(button('恢复继承',row()));await submit();
    assert.equal(submitted[4].clearOverride,true);
    await view.click(button('核对原提交',view.host));
    await view.click(button('读取最新基线（保留输入）',view.host));
    await submit();
    assert.equal(submitted.length,5,'a recovered deletion must not recreate the old override');
    assert.match(row().querySelector('[data-testid="agent-model-source"]').textContent,/默认用途.*new-default/);
    await view.input(view.host.querySelector('section[aria-label="思考与通用分析"] input[aria-label="模型标识"]'),'readback-failure');await submit();
    assert.equal(submitted.length,6);
    assert.match(view.host.textContent,/模型配置已保存，但生效来源读取失败/);
    await view.click(button('读取最新基线（保留输入）',view.host));
    assert.match(row().querySelector('[data-testid="agent-model-source"]').textContent,/默认用途.*readback-failure/);
    assert.equal(submitted.length,6,'a failed source reread must not repeat an accepted write');
  } finally { await view.dispose(); }
});

test("React plugin settings save and reread independent Image and Moment switches", async () => {
  const {default: Management} = await load("react-app/pages/ManagementPage.jsx");
  const server = {endpoints:[], bindings:[], versions:{}, effective_values:{"plugins.domains":{image:true,moment:true}},
    domain_plugins:[{id:"image",version:"1",enabled:true,health:"unconfigured",error_code:"media_not_configured",required_dependencies:["comfyui"],optional_dependencies:[]},{id:"moment",version:"1",enabled:true,health:"not_checked",optional_dependencies:["image"]}]};
  const writes=[];let failNextRead=false;
  const w={...storeFacade(proxy({session:{actorId:"owner-plugin-react"},managementTab:"配置中心",characters:[],operations:[],managementSection:"能力插件",runtimeCatalog:{}})),
    services:{configurationClient:{read:async()=>{if(failNextRead){failNextRead=false;throw Error('catalog unavailable');}return structuredClone(server);},models:async()=>({models:[]}),catalog:async()=>({agents:[]}),
      save:async body=>{writes.push(structuredClone(body));server.effective_values["plugins.domains"]=structuredClone(body.values["plugins.domains"]);server.versions["plugins.domains"]=writes.length;for(const plugin of server.domain_plugins)plugin.enabled=server.effective_values["plugins.domains"][plugin.id];if(writes.length===2)failNextRead=true;},
      command:async()=>({status:"applied",applied_versions:{"plugins.domains":writes.length}})},diagnosticsClient:{page:async()=>({items:[]})}},notify(){},navigatePath(){}};
  let view=await mountReact(vite,Management,{workspace:w});
  try {
    await view.click(view.host.querySelector('[data-testid="config-anchor-能力插件"]'));
    let image=view.host.querySelector('[aria-label="启用图片能力"]');
    let moment=view.host.querySelector('[aria-label="启用动态能力"]');
    assert.ok(image&&moment);assert.equal(image.getAttribute('aria-checked'),'true');assert.equal(moment.getAttribute('aria-checked'),'true');
    await view.click(image);
    await act(async()=>view.host.querySelector('form').dispatchEvent(new window.Event('submit',{bubbles:true,cancelable:true})));
    await view.flush();
    assert.deepEqual(writes[0].values["plugins.domains"],{image:false,moment:true});
    assert.match(view.host.textContent,/图片 · v1 · 已停用 · 图片服务未配置 · 生图需要 ComfyUI/);
    assert.match(view.host.textContent,/动态 · v1 · 已启用 · 健康状态未检测 · 图片为可选依赖/);
    await view.dispose();
    view=await mountReact(vite,Management,{workspace:w});
    await view.click(view.host.querySelector('[data-testid="config-anchor-能力插件"]'));
    image=view.host.querySelector('[aria-label="启用图片能力"]');moment=view.host.querySelector('[aria-label="启用动态能力"]');
    assert.equal(image.getAttribute('aria-checked'),'false');assert.equal(moment.getAttribute('aria-checked'),'true');
    assert.equal(writes.length,1,'rereading must not repeat an accepted write');
    await view.click(moment);
    await act(async()=>view.host.querySelector('form').dispatchEvent(new window.Event('submit',{bubbles:true,cancelable:true})));await view.flush();
    assert.equal(writes.length,2);
    assert.match(view.host.textContent,/插件配置已保存，但生效来源读取失败/);
    await view.click(button('读取最新基线（保留输入）',view.host));
    assert.equal(writes.length,2,'readback failure must not repeat the accepted plugin write');
  } finally { await view.dispose(); }
});

test("React configuration retries a definitively rejected plugin write with a fresh identity", async () => {
  const {default: Management} = await load("react-app/pages/ManagementPage.jsx");
  const server={endpoints:[],bindings:[],versions:{},effective_values:{"plugins.domains":{image:true,moment:true}},domain_plugins:[]};
  const writes=[];
  const w={...storeFacade(proxy({session:{actorId:"owner-plugin-rejected"},managementTab:"配置中心",characters:[],operations:[],managementSection:"能力插件",runtimeCatalog:{}})),
    services:{configurationClient:{read:async()=>structuredClone(server),models:async()=>({models:[]}),catalog:async()=>({agents:[]}),
      save:async body=>{writes.push(structuredClone(body));if(writes.length===1)throw Object.assign(Error('配置未保存'),{status:422,code:'domain_plugins_config_invalid',details:{configuration_admission:'rejected'}});server.effective_values["plugins.domains"]=structuredClone(body.values["plugins.domains"]);},
      command:async()=>({status:'applied',applied_versions:{"plugins.domains":1}})},diagnosticsClient:{page:async()=>({items:[]})}},notify(){},navigatePath(){}};
  const view=await mountReact(vite,Management,{workspace:w});
  try {
    await view.click(view.host.querySelector('[data-testid="config-anchor-能力插件"]'));
    await view.click(view.host.querySelector('[aria-label="启用图片能力"]'));
    const submit=async()=>{await act(async()=>view.host.querySelector('form').dispatchEvent(new window.Event('submit',{bubbles:true,cancelable:true})));await view.flush();};
    await submit();
    assert.equal(writes.length,1);
    assert.equal(button('核对原提交',view.host),undefined,'a definite rejection must not look like an unknown commit');
    await submit();
    assert.equal(writes.length,2);
    assert.notEqual(writes[0].idempotencyKey,writes[1].idempotencyKey);
    assert.deepEqual(writes[1].values["plugins.domains"],{image:false,moment:true});
  } finally {await view.dispose();}
});

test("React world editor preserves world identity, CAS and saved category on reread", async () => {
  const { createWorkspace } = await load("state/workspace.mjs"),
    store = await load("v3/proto/store.js"),
    services = await load("v3/proto/services.js"),
    { default: World } = await load("react-app/pages/WorldPage.jsx");
  let server = {
      id: "W",
      name: "原世界",
      revision: 1,
      flavor: "日常",
      active: true,
      venueCategories: [],
    },
    writes = 0;
  services.configureDomainService("world", {
    worlds: async () => [structuredClone(server)],
    places: async () => [],
  });
  const w = createWorkspace();
  store.replaceSnapshot({
    ...store.get(),
    session: { actorId: "world-owner" },
    characters: { A: { id: "A", name: "测试角色" } },
    ui: { charId: "A" },
  });
  await w.loadWorlds();
  const oldUpdate = w.client.updateCatalogWorld,
    oldCreate = w.client.createCatalogWorld,
    oldCatalog = w.client.worldCatalog,
    oldSnapshot = w.client.catalogWorld;
  const rawWorld = () => ({
    ...structuredClone(server),
    fluctlight_id: "A",
    venue_categories: structuredClone(server.venueCategories),
  });
  w.client.worldCatalog = async () => ({
    items: [rawWorld()],
    active_binding: { world_id: "W", revision: 1 },
    next_cursor: null,
  });
  w.client.catalogWorld = async () => ({
    available: true,
    world: rawWorld(),
    places: [],
    npcs: [],
    scenes: [],
  });
  w.client.createCatalogWorld = async () =>
    assert.fail("must not recreate an existing world");
  w.client.updateCatalogWorld = async (fid, id, body) => {
    assert.equal(fid, "A");
    assert.equal(id, "W");
    assert.equal(body.expectedRevision, server.revision);
    writes++;
    server = {
      ...server,
      name: body.name,
      revision: server.revision + 1,
      ...(body.venueCategories
        ? { venueCategories: structuredClone(body.venueCategories) }
        : {}),
    };
    return { id: "W", revision: server.revision };
  };
  const view = await mountReact(vite, World, { workspace: w });
  try {
    await view.click(button("进入世界"));
    await view.click(button("添加分类"));
    const input = document.querySelector('[role="dialog"] input');
    assert.ok(input);
    await view.input(input, "商店");
    await act(async () =>
      input
        .closest("form")
        .dispatchEvent(
          new window.Event("submit", { bubbles: true, cancelable: true }),
        ),
    );
    await view.flush();
    assert.equal(writes, 1);
    assert.equal(server.venueCategories[0].name, "商店");
    assert.match(view.host.textContent, /商店/);
    await act(async () => {
      w.state.worlds = [];
      await w.loadWorlds();
    });
    await view.flush();
    assert.match(view.host.textContent, /商店/);
    await view.click(button("编辑世界"));
    const name = document.querySelector('[data-testid="world-name"]');
    assert.ok(name);
    await view.input(name, "更新世界");
    await act(async () =>
      name
        .closest("form")
        .dispatchEvent(
          new window.Event("submit", { bubbles: true, cancelable: true }),
        ),
    );
    await view.flush();
    assert.equal(writes, 2);
    assert.equal(server.name, "更新世界");
    assert.equal(server.venueCategories.length, 1);
  } finally {
    await view.dispose();
    w.client.updateCatalogWorld = oldUpdate;
    w.client.createCatalogWorld = oldCreate;
    w.client.worldCatalog = oldCatalog;
    w.client.catalogWorld = oldSnapshot;
    w.dispose();
  }
});

test("React composer retains draft and selection during updates and never sends a composing Enter", async () => {
  const { default: Chat } = await load("react-app/pages/ChatPage.tsx"),
    { createConversationController } = await load("state/conversation.ts");
  const store = createConversationController();
  store.fluctlightId = "A";
  store.conversation = { id: "C" };
  store.messages = [];
  let accepted,
    submissions = 0;
  Object.defineProperty(store, "send", {
    configurable: true,
    value: async (text, _retry, _queue, onAccepted) => {
      submissions++;
      accepted = onAccepted;
    },
  });
  const state = proxy({
    session: { actorId: "chat-owner" },
    selectedId: "A",
    characters: [
      {
        id: "A",
        name: "测试角色",
        initial: "角",
        color: "#eee",
        schedule: [],
        wearing: {},
        garments: [],
        _directory: {},
      },
    ],
    errors: {},
    loading: {},
  });
  const w = {
    ...storeFacade(state),
    current: {
      get value() {
        return state.characters[0];
      },
    },
    currentScope: () => JSON.stringify(["chat-owner", state.selectedId]),
    conversation: () => store,
    openConversation: async () => {},
    clock: () => "",
    detail: async () => {},
    loadWardrobe: async () => {},
    services: { mediaClient: { url: (id) => id } },
  };
  const view = await mountReact(vite, Chat, {
    workspace: w,
    props: { preview: false, onGo() {} },
  });
  try {
    const field = view.host.querySelector("textarea");
    await view.input(field, "尚未发送的草稿");
    field.focus();
    field.setSelectionRange(2, 5);
    await act(async () => {
      store.messages = [
        {
          id: "server",
          kind: "assistant",
          text: "后台读回",
          createdAt: "2026-10-26T00:00:00Z",
        },
      ];
    });
    await view.flush();
    assert.equal(field, view.host.querySelector("textarea"));
    assert.equal(field.value, "尚未发送的草稿");
    assert.equal(document.activeElement, field);
    assert.equal(field.selectionStart, 2);
    await act(async () =>
      field.dispatchEvent(
        new window.KeyboardEvent("keydown", {
          key: "Enter",
          isComposing: true,
          bubbles: true,
          cancelable: true,
        }),
      ),
    );
    assert.equal(submissions, 0);
    await view.click(button("发送"));
    assert.equal(submissions, 1);
    await view.input(field, "提交期间的新草稿");
    await act(async () => accepted());
    await view.flush();
    assert.equal(field.value, "提交期间的新草稿");
  } finally {
    await view.dispose();
  }
});

test("React chat can add a message after reopening a pending conversation", async () => {
  const { default: Chat } = await load("react-app/pages/ChatPage.tsx"),
    { createConversationController } = await load("state/conversation.ts");
  const store = createConversationController();
  store.fluctlightId = "A";
  store.conversation = { id: "C" };
  store.messages = [{id:"accepted-user",conversationId:"C",sequence:1,authorActorId:"chat-owner",kind:"user",text:"之前的话",attachmentRefs:[],createdAt:"2026-10-26T00:00:00Z",turnId:"turn-old",idempotencyKey:"key-old",turnStatus:"running"}];
  let submitted = "";
  Object.defineProperty(store, "send", {configurable:true,value:async (text,_retry,_queue,onAccepted)=>{submitted=text;onAccepted?.();return true;}});
  const state=proxy({session:{actorId:"chat-owner"},selectedId:"A",characters:[{id:"A",name:"测试角色",initial:"角",color:"#eee",schedule:[],wearing:{},garments:[],_directory:{}}],errors:{},loading:{}});
  const w={...storeFacade(state),current:{get value(){return state.characters[0];}},currentScope:()=>JSON.stringify(["chat-owner",state.selectedId]),conversation:()=>store,openConversation:async()=>{},clock:()=>"",detail:async()=>{},loadWardrobe:async()=>{},services:{mediaClient:{url:id=>id}}};
  const view=await mountReact(vite,Chat,{workspace:w,props:{preview:false,onGo(){}}});
  try {
    const field=view.host.querySelector("textarea");
    await view.input(field,"刚想起来还有一句");
    const followUp=button("追加消息",view.host);
    assert.ok(followUp,"a restored pending turn should expose the follow-up action");
    assert.equal(followUp.disabled,false);
    await view.click(followUp);
    assert.equal(submitted,"刚想起来还有一句");
  } finally { await view.dispose(); }
});

test("React setup confirms the exact password and focuses the mismatch without authenticating", async () => {
  const { default: Auth } = await load("react-app/pages/AuthPage.tsx"),
    real = await load("v3/providers/real.ts");
  const oldStatus = real.client.setupStatus,
    oldSetup = real.auth.setup;
  let calls = 0;
  real.client.setupStatus = async () => ({ setupAvailable: true });
  real.auth.setup = async (token, password) => {
    calls++;
    assert.equal(token, "controlled-setup-token");
    assert.equal(password, " exact password ");
  };
  const view = await mountReact(vite, Auth, {
    props: { onAuthenticated() {} },
  });
  try {
    await view.input(
      view.host.querySelector("#setup-token"),
      "controlled-setup-token",
    );
    await view.input(
      view.host.querySelector("#owner-password"),
      " exact password ",
    );
    await view.input(
      view.host.querySelector("#owner-password-confirm"),
      "different",
    );
    await view.click(button("完成初始化"));
    assert.equal(calls, 0);
    assert.match(view.host.textContent, /两次输入的密码不一致/);
    assert.equal(document.activeElement.id, "owner-password-confirm");
    await view.input(
      view.host.querySelector("#owner-password-confirm"),
      " exact password ",
    );
    await view.click(button("完成初始化"));
    assert.equal(calls, 1);
  } finally {
    await view.dispose();
    real.client.setupStatus = oldStatus;
    real.auth.setup = oldSetup;
  }
});

test("React diagnostic detail preserves the complete original request and retains it on read failure", async () => {
  const { default: Detail } = await load(
    "react-app/pages/DiagnosticsDetailPage.tsx",
  );
  let fail = false,
    reads = 0;
  const source = {
    id: "original-request",
    status: "failed",
    reason: "失败原因",
    payload: { actual: "未经改写的原内容" },
    result: { mutation_committed: false },
  };
  const w = {
    client: {
      ownerRequest: async (id) => {
        assert.equal(id, "original-request");
        reads++;
        if (fail) throw Error("读取失败");
        return structuredClone(source);
      },
    },
    services: { diagnosticsClient: {} },
  };
  const view = await mountReact(vite, Detail, {
    workspace: w,
    props: { target: { kind: "request", id: "original-request" } },
  });
  try {
    await view.click(button("原始记录与技术详情"));
    assert.match(view.host.textContent, /未经改写的原内容/);
    assert.match(view.host.textContent, /original-request/);
    fail = true;
    await view.click(button("重新读取"));
    assert.equal(reads, 2);
    assert.match(view.host.textContent, /读取失败/);
    assert.match(view.host.textContent, /未经改写的原内容/);
  } finally {
    await view.dispose();
  }
});

test("React resource editor rejects whitespace and preserves edits made while an original save is pending", async () => {
  const { default: Editor } = await load(
    "react-app/pages/StudioEditorPage.tsx",
  );
  let release,
    writes = 0,
    payload;
  const resource = {
    id: "place",
    kind: "place",
    name: "原名称",
    body: { description: "原描述" },
    draft_version: "v1",
    revision: 1,
  };
  const w = {
    ...storeFacade(
      proxy({
        session: { actorId: "editor-render-owner" },
        characters: [],
        selectedId: "A",
        operations: [],
      }),
    ),
    services: {
      studioClient: {
        jobs: async () => ({ items: [], next_cursor: "" }),
        materials: async () => ({ items: [], next_cursor: "" }),
        referenceGrants: async () => ({ items: [], next_cursor: "" }),
        versions: async () => ({ items: [], next_cursor: "" }),
        resource: async () => structuredClone(resource),
        save: async (id, body) => {
          assert.equal(id, "place");
          writes++;
          payload = body;
          return new Promise(
            (resolve) =>
              (release = (value) => {
                Object.assign(resource, value);
                resolve(structuredClone(resource));
              }),
          );
        },
      },
    },
    navigatePath() {},
  };
  const view = await mountReact(vite, Editor, {
    workspace: w,
    props: { resourceId: "place" },
  });
  try {
    const name = view.host.querySelector("#resource-name");
    await view.input(
      view.host.querySelector("#resource-reason"),
      "明确保存理由",
    );
    await view.input(name, "   ");
    await view.click(button("保存草稿"));
    assert.equal(writes, 0);
    assert.match(view.host.textContent, /请填写资源名称/);
    await view.input(name, "新名称");
    await view.click(button("保存草稿"));
    assert.equal(writes, 1);
    assert.equal(payload.name, "新名称");
    assert.equal(payload.expectedVersion, "v1");
    await view.input(name, "保存期间的新名称");
    await act(async () =>
      release({
        ...resource,
        name: "新名称",
        draft_version: "v2",
        revision: 2,
      }),
    );
    await view.flush();
    assert.equal(name.value, "保存期间的新名称");
    assert.equal(button("保存草稿").disabled, false);
  } finally {
    await view.dispose();
  }
});
