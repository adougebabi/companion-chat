import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";
import { React, act, mountReact, installDOM } from "./helpers/react-dom.mjs";
import { proxy, snapshot, subscribe } from "valtio/vanilla";
const environment = installDOM();
test.after(() => environment.restore());
const vite = await createServer({
  root: fileURLToPath(new URL("../", import.meta.url)),
  appType: "custom",
  logLevel: "silent",
  server: { middlewareMode: true },
});
test.after(() => vite.close());
const load = (path) => vite.ssrLoadModule("/src/" + path);
const workspace = () => {
  const state = proxy({
    session: { actorId: "owner" },
    characters: [
      {
        id: "A",
        name: "测试角色",
        job: "编辑",
        bio: "喜欢阅读",
        tags: ["读书"],
        initial: "角",
        color: "#eee",
      },
    ],
    selectedId: "A",
    errors: {},
    loading: {},
    operations: [],
  });
  return {
    state,
    subscribe: (fn) => subscribe(state, fn),
    getSnapshot: () => snapshot(state),
    select(id) {
      state.selectedId = id;
    },
    navigate() {},
    services: {
      configurationClient: {
        read: async () => ({ bindings: [{ role: "generic_llm" }] }),
      },
    },
  };
};

test("React buttons preserve submit, disabled, escaped text and fresh click handlers", async () => {
  const { Button } = await load("components/ui/button.tsx");
  let calls = [];
  function View({ value, disabled }) {
    return React.createElement(
      "form",
      {
        onSubmit: (e) => {
          e.preventDefault();
          calls.push(value);
        },
      },
      React.createElement(Button, { type: "submit", disabled }, value),
    );
  }
  const view = await mountReact(vite, View, {
    props: { value: "<first>", disabled: true },
  });
  try {
    const button = view.host.querySelector("button");
    assert.equal(button.dataset.slot, "button");
    assert.equal(button.textContent, "<first>");
    await view.click(button);
    assert.deepEqual(calls, []);
    await view.update({ value: "second", disabled: false });
    await view.click(view.host.querySelector("button"));
    assert.deepEqual(calls, ["second"]);
  } finally {
    await view.dispose();
  }
});

test("React Checkbox and Switch keep booleans and selected collection values distinct", async () => {
  const { FieldInput } = await load("react-app/controls.tsx");
  let latest;
  function View() {
    const [values, setValues] = React.useState([]),
      [enabled, setEnabled] = React.useState(false);
    latest = { values, enabled };
    return React.createElement(
      React.Fragment,
      null,
      React.createElement(FieldInput, {
        type: "checkbox",
        value: "reading",
        modelValue: values,
        onModelChange: setValues,
        "aria-label": "阅读",
      }),
      React.createElement(FieldInput, {
        type: "checkbox",
        className: "switch",
        checked: enabled,
        onChange: (e) => setEnabled(e.target.checked),
        "aria-label": "启用",
      }),
    );
  }
  const view = await mountReact(vite, View);
  try {
    await view.click(view.host.querySelector('[role="checkbox"]'));
    await view.click(view.host.querySelector('[role="switch"]'));
    assert.deepEqual(latest, { values: ["reading"], enabled: true });
    await view.click(view.host.querySelector('[role="checkbox"]'));
    assert.deepEqual(latest.values, []);
  } finally {
    await view.dispose();
  }
});

test("React Select retains empty, boolean and numeric domain values through keyboard choices", async () => {
  const { ChoiceSelect } = await load("react-app/controls.tsx");
  let selected = "";
  function View() {
    const [value, setValue] = React.useState("");
    return React.createElement(
      ChoiceSelect,
      {
        value,
        onChange: (e) => {
          selected = e.target.value;
          setValue(selected);
        },
        "aria-label": "类型",
      },
      ["", false, 0, null].map((value, index) =>
        React.createElement(
          "option",
          { key: index, value },
          ["未选择", "否", "零", "未记录"][index],
        ),
      ),
    );
  }
  const view = await mountReact(vite, View);
  try {
    const trigger = view.host.querySelector('[role="combobox"]');
    assert.match(trigger.textContent, /未选择/);
    for (const [label, expected] of [
      ["否", false],
      ["零", 0],
      ["未记录", null],
      ["未选择", ""],
    ]) {
      await act(async () =>
        trigger.dispatchEvent(
          new window.KeyboardEvent("keydown", {
            key: "ArrowDown",
            bubbles: true,
          }),
        ),
      );
      await view.flush();
      const option = [...document.querySelectorAll('[role="option"]')].find(
        (node) => node.textContent === label,
      );
      assert.ok(option, label);
      await view.click(option);
      assert.equal(selected, expected);
    }
  } finally {
    await view.dispose();
  }
});

test("React library search, clear and character menu consume the actual directory snapshot", async () => {
  const { default: Library } = await load("react-app/pages/LibraryPage.jsx");
  const w = workspace(),
    view = await mountReact(vite, Library, { workspace: w });
  try {
    assert.match(view.host.textContent, /测试角色/);
    const search = view.host.querySelector('input[type="search"]');
    await view.input(search, "不存在");
    assert.match(view.host.textContent, /暂无匹配角色/);
    const clear = [...view.host.querySelectorAll("button")].find(
      (node) => node.textContent === "清除筛选",
    );
    await view.click(clear);
    assert.match(view.host.textContent, /测试角色/);
    assert.equal(search.value, "");
  } finally {
    await view.dispose();
  }
});

test("React semantic profile preserves custom facts and hides unknown body fields", async () => {
  const { default: Tree } = await load("react-app/shared/SemanticTree.jsx");
  const view = await mountReact(vite, Tree, {
    props: {
      value: {
        portrait_text: "agreeableness: 0.5\n自定义经历保留",
        body_fields: {
          chest_cup: { status: "unknown", value: null },
          hair_color: { status: "known", value: "黑色" },
        },
      },
      identity: { gender: "male" },
    },
  });
  try {
    assert.match(view.host.textContent, /亲和程度适中/);
    assert.match(view.host.textContent, /自定义经历保留/);
    assert.match(view.host.textContent, /黑色/);
    assert.doesNotMatch(view.host.textContent, /胸部尺寸/);
  } finally {
    await view.dispose();
  }
});

test("notification disclosure keeps every record and starts closed", async () => {
  const { default: Group } = await load(
    "react-app/shared/NotificationGroup.jsx",
  );
  const seen = [];
  const view = await mountReact(vite, Group, {
    props: {
      group: {
        name: "角色",
        notices: [
          { id: "one", title: "第一条", body: "回复" },
          { id: "two", title: "第二条", body: "动态" },
        ],
      },
      onOpen: (id) => seen.push(id),
    },
  });
  try {
    assert.match(view.host.textContent, /2 条未读/);
    const trigger = view.host.querySelector(
      '[data-slot="collapsible-trigger"]',
    );
    assert.equal(trigger.getAttribute("aria-expanded"), "false");
    await view.click(trigger);
    assert.match(view.host.textContent, /回复/);
    assert.match(view.host.textContent, /动态/);
    await view.click(
      [...view.host.querySelectorAll("button")].find((node) =>
        node.textContent.includes("第二条"),
      ),
    );
    assert.deepEqual(seen, ["two"]);
  } finally {
    await view.dispose();
  }
});

test("controller effects observe committed DOM and release subscriptions on unmount", async () => {
  const { useController } = await load("react-app/page.tsx");
  let count = 0,
    detach = 0,
    currentCell;
  function factory(props, context, controller) {
    currentCell = controller.cell("one");
    controller.observe(
      currentCell,
      () => {
        count++;
        assert.equal(
          document.querySelector("[data-step]").textContent,
          currentCell.value,
        );
      },
      { flush: "post" },
    );
    controller.onDispose(() => detach++);
    return () =>
      React.createElement("h2", { "data-step": true }, currentCell.value);
  }
  function View() {
    return useController(factory, {});
  }
  const view = await mountReact(vite, View);
  await act(async () => {
    currentCell.value = "two";
  });
  await view.flush();
  assert.equal(count, 1);
  await view.dispose();
  assert.equal(detach, 1);
  currentCell.value = "after";
  await Promise.resolve();
  assert.equal(count, 1);
});
