import test from 'node:test';
import assert from 'node:assert/strict';
import { Avatar, bindAvatars } from '../src/v3/proto/components/avatar.js';
import { MessageBubble } from '../src/v3/proto/components/chat.js';
import { chatFailureText, chatDiagnosticTurn } from '../src/v3/chat-feedback.mjs';
import { planWindow } from '../src/v3/presentation-time.mjs';
import { SchedulePlanItem, refreshPlanWindows } from '../src/v3/proto/components/schedule-records.js';
const previousDocument = globalThis.document;
globalThis.document = {
  addEventListener() {}
};
const {
  CurrentInventory,
  InventoryCard
} = await import('../src/v3/proto/components/inventory-records.js');
if (previousDocument === undefined) delete globalThis.document;else globalThis.document = previousDocument;
import { requestDecisionText, RequestFailureLink } from '../src/v3/proto/components/request-failure.js';
test('avatar uses a neutral initial for empty/broken media and escapes external attributes', () => {
  const empty = Avatar({
    src: '',
    name: '小岚'
  });
  assert.match(empty, /小岚的头像/);
  assert.match(empty, />小<\/span>/);
  assert.doesNotMatch(empty, /<img|xiaoyao/);
  assert.match(Avatar({
    src: '" onerror="x',
    name: '<script>'
  }), /&quot; onerror=&quot;x/);
  let onError,
    removed = 0;
  bindAvatars({
    querySelectorAll: () => [{
      complete: false,
      addEventListener: (event, fn) => {
        assert.equal(event, 'error');
        onError = fn;
      },
      remove: () => removed++
    }, {
      complete: true,
      naturalWidth: 0,
      addEventListener() {},
      remove: () => removed++
    }]
  });
  assert.equal(removed, 1);
  onError();
  assert.equal(removed, 2);
  const bubble = MessageBubble({
    id: 'one',
    from: 'char',
    text: '你好'
  }, {
    characterAvatar: '',
    characterName: '小岚',
    mediaById() {}
  });
  assert.match(bubble, /你好/);
  assert.doesNotMatch(bubble, /src=""|xiaoyao/);
});
test('chat budget/partial errors become readable while real user content remains intact', () => {
  assert.match(chatFailureText('回复未完成：prompt_tool_result_budget_exceeded'), /原消息已保留/);
  assert.doesNotMatch(chatFailureText('回复未完成：prompt_tool_result_budget_exceeded'), /prompt_/);
  assert.match(chatFailureText('agent_partial_committed'), /行动可能已经发生/);
  const original = '回复未完成：prompt_tool_result_budget_exceeded，这是我输入的原文';
  const bubble = MessageBubble({
    id: 'user',
    from: 'user',
    text: original,
    turnErrorCode: 'prompt_tool_result_budget_exceeded'
  }, {
    mediaById() {}
  });
  assert.match(bubble, /回复未完成：prompt_tool_result_budget_exceeded，这是我输入的原文/);
});
test('plan windows use half-open instants across midnight, offset and DST, never wall-clock completion', () => {
  const start = '2026-10-20T23:00:00+08:00',
    end = '2026-10-21T00:00:00+08:00';
  assert.equal(planWindow(start, end, Date.parse(start) - 1), 'future');
  assert.equal(planWindow(start, end, Date.parse(start)), 'current');
  assert.equal(planWindow(start, end, Date.parse(end)), 'past');
  assert.equal(planWindow(start, end, Date.parse('2026-10-20T15:15:00Z')), 'current');
  assert.equal(planWindow('2026-11-01T01:00:00-04:00', '2026-11-01T01:30:00-05:00', Date.parse('2026-11-01T06:00:00Z')), 'current');
  for (const [from, to] of [['bad', end], [end, start], [start, start]]) assert.equal(planWindow(from, to), 'unknown');
  const item = {
    id: 'original',
    start_at: start,
    end_at: end,
    activity: '睡眠',
    location: '',
    status: 'scheduled'
  };
  const html = SchedulePlanItem(item, 'Asia/Shanghai', Date.parse(start));
  assert.match(html, /is-current|当前计划时段/);
  assert.match(html, /已安排/);
  assert.match(html, /地点未记录/);
  assert.match(html, /aria-label="尚无执行结果"/);
  assert.doesNotMatch(html, /正在执行|>scheduled<|>家</);
  assert.match(html, /<details\b[^>]*class="schedule-item-options"[^>]*><summary>计划操作与来源<\/summary>/);
  assert.match(html, /data-edit="original"/);
  assert.equal(item.status, 'scheduled');
  const classes = new Map(),
    label = {
      textContent: ''
    };
  const row = {
    dataset: {
      scheduleSlot: item.id
    },
    classList: {
      toggle: (name, value) => classes.set(name, value)
    },
    querySelector: () => label
  };
  refreshPlanWindows({
    querySelectorAll: () => [row]
  }, [item], Date.parse(end));
  assert.equal(classes.get('is-current'), false);
  assert.equal(classes.get('is-past'), true);
  assert.equal(label.textContent, '历史计划时段');
});
test('chat diagnosis follows the failed retry identity, not a newer queued message or page error', () => {
  const failed = {
    id: 'persisted',
    kind: 'user',
    turnId: 'failed-turn',
    turnStatus: 'failed',
    turnErrorCode: 'prompt_tool_result_budget_exceeded'
  };
  const state = {
    conversation: {
      id: 'conversation-one'
    },
    fluctlightId: 'fl-one',
    error: '回复未完成：prompt_tool_result_budget_exceeded',
    messages: [failed, {
      id: 'local-queued',
      kind: 'user',
      turnId: 'queued-turn'
    }],
    queuedTurn: {
      turnId: 'queued-turn'
    },
    retryTurn: {
      conversationId: 'conversation-one',
      fluctlightId: 'fl-one',
      turnId: 'failed-turn'
    }
  };
  assert.equal(chatDiagnosticTurn(state), 'failed-turn');
  assert.equal(chatDiagnosticTurn({
    ...state,
    retryTurn: null
  }), 'failed-turn');
  assert.equal(chatDiagnosticTurn({
    ...state,
    error: '历史读取失败：网络不可用'
  }), null);
  assert.equal(chatDiagnosticTurn({
    ...state,
    retryTurn: {
      ...state.retryTurn,
      fluctlightId: 'other'
    },
    messages: [state.messages[1]]
  }), null);
  assert.equal(chatDiagnosticTurn({
    ...state,
    retryTurn: null,
    messages: [{
      ...failed,
      id: 'local-old'
    }, state.messages[1]]
  }), null);
});
test('a pending durable turn always has a diagnostic target after observer disconnect', () => {
  const state = {
    conversation: {
      id: 'conversation'
    },
    fluctlightId: 'role',
    error: '',
    activeTurnId: null,
    messages: [{
      id: 'accepted',
      kind: 'user',
      turnId: 'background-turn',
      turnStatus: 'running'
    }, {
      id: 'local-queued',
      kind: 'user',
      turnId: 'new-draft'
    }],
    queuedTurn: {
      turnId: 'new-draft'
    }
  };
  assert.equal(chatDiagnosticTurn(state), 'background-turn');
  assert.match(chatFailureText('回复未完成：provider_gateway_timeout'), /中转.*超时/);
  assert.match(chatFailureText('回复未完成：prompt_required_budget_exceeded'), /资料.*本次运行/);
});
test('empty inventory is explicit without claiming nakedness; actual categories use shared labels', () => {
  const html = CurrentInventory([], []);
  assert.match(html, /当前没有已登记的穿着衣物/);
  assert.doesNotMatch(html, /未穿衣|没有衣服/);
  const card = InventoryCard({
    id: 'one',
    fluctlight_id: 'fl-one',
    description: '外套',
    category: 'outdoor_clothing',
    slot: 'outerwear',
    ownership: 'unknown',
    availability: 'available'
  });
  assert.match(card, /户外服装/);
  assert.doesNotMatch(card, /outdoor_clothing/);
});
test('failed assessment links the original diagnostic without replaying or mislabeling a refusal', () => {
  const request = {
    id: 'req-123&x',
    status: 'failed',
    result: {
      error_code: 'owner_life_assessment_failed'
    },
    decision_reason: 'The request did not produce a committed result'
  };
  assert.match(requestDecisionText(request), /未能完成判断/);
  assert.match(RequestFailureLink(request), /eventType=owner.life.request.assessment_failed&q=req-123%26x/);
  assert.doesNotMatch(RequestFailureLink(request), /data-retry|已恢复|已完成/);
  assert.equal(RequestFailureLink({
    ...request,
    status: 'deferred'
  }), '');
  assert.equal(requestDecisionText({
    ...request,
    status: 'deferred',
    decision_reason: '先休息'
  }), '先休息');
});
