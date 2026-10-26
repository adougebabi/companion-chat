import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { requestDecisionText, RequestDecisionSource, RequestFailureLink } from '../src/v3/proto/components/request-failure.js';
import { esc, badge } from '../src/v3/proto/components/controls.js';
import { JsonDetails } from '../src/v3/proto/components/records.js';
import { userInstant } from '../src/v3/presentation-time.mjs';
import { stateLabel, domainName } from '../src/v3/domain-presentation.mjs';
const failed = {
  id: 'original-request',
  status: 'failed',
  decision_reason: 'The request did not produce a committed result',
  created_at: '2026-10-20T15:01:58.000+00:00',
  payload: {
    text: '我的建议',
    stance: 'agree'
  },
  result: {
    error_code: 'owner_life_request_not_applied',
    disposition: 'not_applied'
  }
};
const bindings = {
  get: () => ({
    actorId: "owner"
  }),
  errorMessage: () => "读取暂不可用",
  domainName,
  esc,
  badge,
  JsonDetails,
  userInstant,
  instant: userInstant,
  domainStateLabel: stateLabel,
  requestDecisionText,
  RequestDecisionSource,
  RequestFailureLink
};
function load(path, names, extras = {}) {
  const source = readFileSync(new URL('../src/v3/proto/' + path, import.meta.url), 'utf8').replace(/^import .+;\s*$/gm, '').replaceAll('export ', '');
  const args = {
    ...bindings,
    ...extras
  };
  return Function(...Object.keys(args), source + `\nreturn {${names.join(',')}};`)(...Object.values(args));
}
function element() {
  const nodes = new Map();
  return {
    isConnected: true,
    innerHTML: '',
    querySelector(selector) {
      if (!nodes.has(selector)) nodes.set(selector, {
        innerHTML: '',
        insertAdjacentHTML() {},
        addEventListener() {}
      });
      return nodes.get(selector);
    },
    querySelectorAll: () => []
  };
}
function ordinary(html) {
  return html.replace(/<details\b[^>]*>[\s\S]*?<\/details>/g, '');
}
function checkDecision(html) {
  assert.match(ordinary(html), /没有可确认的已提交结果/);
  assert.doesNotMatch(ordinary(html), /failed|The request|2026-10-20T15:01:58/);
  assert.match(html, /<details[^>]*><summary>请求原始状态、理由与时间<\/summary>/);
  assert.match(html, /The request did not produce a committed result/);
  assert.match(html, /2026-10-20T15:01:58.000\+00:00/);
}
test('fixed settlement reasons are localized with status/effect evidence and authored Chinese stays intact', () => {
  const backend = readFileSync(new URL('../../core-go/internal/core/owner_life_requests.go', import.meta.url), 'utf8');
  const cases = [[failed, '没有可确认'], [{
    status: 'completed',
    decision_reason: 'The requested change was committed'
  }, '变更已提交'], [{
    status: 'accepted',
    decision_reason: 'An intention was recorded; fulfillment requires later evidence'
  }, '后续实际证据'], [{
    status: 'awaiting_goal',
    decision_reason: 'The proposed action awaits explicit Planner goal binding; no accepted promise yet'
  }, '约定尚未接受'], [{
    status: 'completed',
    decision_reason: 'The exact requested target state was already recorded; no new mutation was needed',
    result: {
      disposition: 'no_change'
    }
  }, '无需新增操作']];
  for (const [request, expected] of cases) {
    assert.ok(backend.includes(request.decision_reason));
    assert.ok(requestDecisionText(request).includes(expected));
  }
  for (const status of ['rejected', 'deferred', 'failed']) assert.equal(requestDecisionText({
    ...failed,
    status,
    decision_reason: '先休息，明天再说。'
  }), '先休息，明天再说。');
  const unknown = requestDecisionText({
    ...failed,
    decision_reason: 'Unexpected engine fault',
    result: {
      error_code: 'unknown',
      mutation_committed: true
    }
  });
  assert.match(unknown, /处理失败/);
  assert.doesNotMatch(unknown, /成功|未产生|没有操作|无副作用|Unexpected/);
  const assessment = requestDecisionText({
    ...failed,
    result: {
      error_code: 'owner_life_assessment_failed',
      mutation_committed: true
    }
  });
  assert.match(assessment, /未能完成判断/);
  assert.doesNotMatch(assessment, /未产生|无副作用|没有操作/);
  assert.doesNotMatch(requestDecisionText({
    status: 'mystery',
    decision_reason: 'The requested change was committed'
  }), /变更已提交|成功/);
  const unsafe = RequestDecisionSource({
    ...failed,
    decision_reason: '<img src=x onerror=alert(1)>'
  });
  assert.match(unsafe, /&lt;img/);
  assert.doesNotMatch(unsafe, /<img|<details[^>]*\bopen\b/);
  assert.equal(failed.decision_reason, 'The request did not produce a committed result');
});
test('terminal request receipt keeps original identity and exposes raw diagnostic record without resubmission', async () => {
  const d = {
    el: element(),
    close() {}
  };
  let observed;
  const queries = [];
  const {
    requestReceipt
  } = load('components/async-page.js', ['requestReceipt'], {
    stateLabel,
    drawer: () => d,
    InventoryEffects: () => '',
    lifeClient: {
      request: async (fid, id) => {
        queries.push([fid, id]);
        return failed;
      }
    }
  });
  requestReceipt('role', {
    request: failed
  }, r => {
    observed = r;
  });
  checkDecision(d.el.querySelector('[data-request]').innerHTML);
  await d.el.querySelector('[data-query]').onclick();
  assert.deepEqual(queries, [['role', 'original-request']]);
  requestReceipt('role', {
    request: {
      ...failed,
      result: {
        disposition: 'no_change'
      }
    }
  });
  assert.doesNotMatch(ordinary(d.el.querySelector('[data-request]').innerHTML), /目标状态已满足/);
  assert.equal(observed, failed);
  assert.match(d.el.querySelector('[data-request]').innerHTML, /#\/diag\/request\/original-request/);
});
