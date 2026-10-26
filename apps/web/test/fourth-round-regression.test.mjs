import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { chatSummaryPreference } from '../src/v3/chat-summary-preference.mjs';
import { momentReading } from '../src/v3/moment-reading.mjs';
import { DiagnosticCard } from '../src/v3/proto/components/diagnostic-card.js';
import { errorMessage } from '../src/v3/domain-presentation.mjs';
test('chat preview choice survives a new conversation and stays Owner/character scoped', () => {
  const values = new Map(),
    storage = {
      getItem: k => values.get(k),
      setItem: (k, v) => values.set(k, v)
    };
  const options = {
    owner: 'one',
    fid: 'role',
    storage,
    defaultFolded: false
  };
  chatSummaryPreference(options).save(true);
  assert.equal(chatSummaryPreference(options).read(), true);
  assert.equal(chatSummaryPreference({
    ...options,
    owner: 'two'
  }).read(), false);
  assert.equal(chatSummaryPreference({
    ...options,
    fid: 'other'
  }).read(), false);
  chatSummaryPreference(options).save(false);
  assert.equal(chatSummaryPreference(options).read(), false);
});
test('historical corrupt moment uses an explicit reading projection without changing the original', () => {
  const post = {
    author_type: 'fluctlight',
    text: '今天读完了一本书。ાન્ય'
  };
  const before = structuredClone(post);
  assert.deepEqual(momentReading(post), {
    text: '今天读完了一本书。',
    original: post.text,
    corrected: true
  });
  assert.deepEqual(post, before);
  for (const text of ['Gujarati 例句。ાન્ય', '她提到的名字是 અન્ય', '今天很好。✨']) assert.equal(momentReading({
    author_type: 'fluctlight',
    text
  }).corrected, false);
  assert.equal(momentReading({
    ...post,
    author_type: 'user'
  }).corrected, false);
});
test('diagnostics show gateway reason and measured token usage without obsolete caps', () => {
  const html = DiagnosticCard({
    id: 'run',
    ref: 'ref',
    kind: 'model-runs',
    data: {
      model_id: 'writer',
      status: 'failed',
      error_code: 'provider_gateway_timeout',
      binding_role: 'conversation_reply',
      metrics: {
        prompt_budget: {
          estimated_input_tokens: 48000,
          max_input_tokens: 49152
        }
      }
    }
  });
  assert.match(html, /中转服务等待超时/);
  assert.match(html, /估算输入 48000/);
  assert.doesNotMatch(html, /有效输入上限/);
  assert.doesNotMatch(errorMessage(new Error('INSERT INTO fluctlight_wardrobe_outfit_items constraint failed')), /INSERT|fluctlight_/);
});
test('diary readable count and calendar use the same visible rows, with generation status shown separately', async () => {
  const source = readFileSync(new URL('../src/v3/proto/components/calendar-reading.js', import.meta.url), 'utf8').replace(/^import .*;\s*$/gm, '').replaceAll('export function ', 'function ');
  function node() {
    const nodes = new Map();
    return {
      innerHTML: '',
      isConnected: true,
      querySelector(s) {
        if (!nodes.has(s)) nodes.set(s, node());
        return nodes.get(s);
      },
      querySelectorAll: () => []
    };
  }
  const mount = Function('esc', 'Field', 'displayTimezone', 'store', 'errorMessage', source + ';return mountCalendarReading')(String, () => '', () => 'Asia/Shanghai', {
    get: () => ({
      actorId: 'owner'
    }),
    subscribe() {}
  }, String);
  const el = node(),
    dispose = mount(el, {
      scope: ['round4'],
      initialMode: 'list',
      initialMonth: '2026-10',
      visible: d => !!d.body,
      render: d => d.body,
      renderHidden: d => '<p>' + d.status + '</p>',
      read: async () => ({
        items: [{
          id: 'one',
          date: '2026-10-21',
          body: '',
          status: 'running'
        }, {
          id: 'two',
          date: '2026-10-22',
          body: '',
          status: 'failed'
        }]
      }),
      dateOf: d => d.date
    });
  await new Promise(resolve => setImmediate(resolve));
  assert.match(el.querySelector('[data-calendar]').innerHTML, /本月已载入 0 条/);
  assert.match(el.querySelector('[data-calendar]').innerHTML, /另有 2 条生成记录/);
  assert.doesNotMatch(el.querySelector('[data-calendar]').innerHTML, /class="calendar-grid" hidden/);
  assert.match(el.querySelector('[data-papers]').innerHTML, /日记生成记录/);
  assert.match(el.querySelector('[data-papers]').innerHTML, /running|failed/);
  dispose();
});
