import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
const source = name => readFileSync(new URL(`../src/v3/proto/components/${name}.js`, import.meta.url), 'utf8');
function load(name, bindings, exports) {
  return Function(...Object.keys(bindings), source(name).replace(/^import .*;\s*$/gm, '').replaceAll('export function ', 'function ') + `\nreturn {${exports}}`)(...Object.values(bindings));
}
const esc = value => String(value ?? '').replaceAll('<', '&lt;');
const presentation = {
  esc,
  stateLabel: value => ({
    running: '执行中',
    pending: '等待处理'
  })[value] || '状态待确认',
  errorMessage: () => '反思处理失败',
  instant: value => value,
  JsonDetails: () => '<details>原始状态</details>'
};
test('reflection failure, replacement and scheduled retry are visible independently of missing windows', () => {
  const {
    ReflectionProgress,
    reflectionTaskPresentation
  } = load('reflection-progress', presentation, 'ReflectionProgress,reflectionTaskPresentation');
  const failed = ReflectionProgress({
    window: null,
    latest_task: {
      intent_id: 'original/one',
      status: 'superseded',
      error_code: 'reflection_task_failed',
      attempt_count: 0
    }
  });
  assert.match(failed, /banner warn/);
  assert.match(failed, /反思处理失败/);
  assert.match(failed, /取代/);
  assert.match(failed, /尚未开始执行/);
  assert.match(failed, /original%2Fone/);
  assert.doesNotMatch(failed, /状态待确认/);
  const retry = reflectionTaskPresentation({
    status: 'retry_wait',
    next_attempt_at: '2026-10-22T02:00:00Z',
    attempt_count: 2,
    error_code: 'reflection_task_failed'
  });
  assert.match(retry.description, /已安排重试/);
  assert.match(retry.description, /已尝试 2 次/);
  assert.equal(retry.attention, true);
  const pending = reflectionTaskPresentation({
    status: 'pending',
    attempt_count: 0
  });
  assert.equal(pending.attention, false);
  assert.match(pending.description, /尚未开始执行/);
  assert.match(ReflectionProgress({
    window: null,
    latest_task: null
  }), /还没有反思任务记录/);
});
function node() {
  const children = new Map();
  return {
    innerHTML: '',
    textContent: '',
    disabled: false,
    hidden: false,
    value: '',
    isConnected: true,
    reportValidity: () => true,
    querySelector(s) {
      if (!children.has(s)) children.set(s, node());
      return children.get(s);
    },
    querySelectorAll() {
      return [];
    }
  };
}
test('calendar month count is scoped to visible month, refresh has feedback, empty date validates', async () => {
  const {
    mountCalendarReading
  } = load('calendar-reading', {
    esc,
    Field: () => '',
    displayTimezone: () => 'Asia/Shanghai',
    store: {
      subscribe() {},
      get: () => ({
        actorId: 'owner'
      })
    },
    errorMessage: () => '读取暂不可用'
  }, 'mountCalendarReading');
  const el = node();
  const dispose = mountCalendarReading(el, {
    scope: ['fid', 'diary'],
    initialMonth: '2026-09',
    read: async () => ({
      items: [{
        id: 'oct',
        date: '2026-10-01'
      }],
      next_cursor: ''
    }),
    dateOf: row => row.date,
    render: row => row.id,
    label: '日记'
  });
  await new Promise(resolve => setImmediate(resolve));
  const calendar = el.querySelector('[data-calendar]');
  assert.match(calendar.innerHTML, /本月已载入 0 条日记；共已载入 1 条/);
  assert.match(el.querySelector('[data-reading-status]').innerHTML, /已更新/);
  let validations = 0;
  const form = {
    elements: {
      date: {
        value: ''
      }
    },
    reportValidity() {
      validations++;
      return false;
    }
  };
  calendar.querySelector('[data-reading-find]').onsubmit({
    preventDefault() {},
    target: form
  });
  assert.equal(validations, 1);
  assert.match(calendar.innerHTML, /2026-09/);
  dispose();
});
test('foundation proposal defaults to omitted fields, invalid JSON focuses the editable changes', async () => {
  let body;
  const root = node();
  root.querySelector('[name=changes]').value = '{}';
  root.querySelector('[name=reason]').value = '解释';
  let focused = false;
  const changes = root.querySelector('[name=changes]');
  changes.setCustomValidity = () => {};
  changes.setAttribute = () => {};
  changes.focus = () => {
    focused = true;
  };
  const {
    foundationProposal
  } = load('foundation-proposal', {
    beginDraftSubmission() {
      throw new Error('must not submit invalid changes');
    },
    settleDraftSubmission() {},
    Field: options => {
      if (options.name === 'changes') body = options.value;
      return '';
    },
    JsonDetails: () => '',
    governanceClient: {},
    get: () => ({
      actorId: 'owner'
    }),
    subscribe() {},
    drawer: () => ({
      el: root,
      close() {}
    }),
    esc,
    confirm: async () => true,
    toast() {}
  }, 'foundationProposal');
  foundationProposal('fid', {
    revision: 3,
    identity: {}
  }, () => {});
  assert.equal(body, '{}');
  assert.doesNotMatch(body, /background_story/);
  await root.querySelector('[data-submit]').onclick();
  assert.equal(focused, true);
  assert.match(root.querySelector('[data-error]').textContent, /非空 JSON 对象/);
});
test('existing schedule explains protected record even with no entries and offers close without a write', () => {
  let options,
    closed = false;
  const root = node();
  const {
    scheduleRequestForm
  } = load('schedule-request', {
    lifeClient: new Proxy({}, {
      get() {
        throw new Error('existing schedule must not write');
      }
    }),
    drawer: value => {
      options = value;
      return {
        el: root,
        close() {
          closed = true;
        }
      };
    },
    esc,
    badge: () => '',
    Field: () => '',
    zonedInput: () => '',
    zonedInputInstant: () => ''
  }, 'scheduleRequestForm');
  scheduleRequestForm('fid', {
    calendar: {
      from: '2026-10-22',
      timezone: 'Asia/Shanghai'
    },
    schedule: {
      items: []
    }
  });
  assert.match(options.body, /包含 0 项安排/);
  assert.match(options.body, /没有约定条目也不代表没有日程记录/);
  assert.doesNotMatch(options.body, /已有已接受/);
  root.querySelector('[data-close]').onclick();
  assert.equal(closed, true);
});
