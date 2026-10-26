import test from 'node:test';
import assert from 'node:assert/strict';
import { errorMessage } from '../src/v3/domain-presentation.mjs';
import { chatFailureText, chatWaitText, chatWaitingPresentation } from '../src/v3/chat-feedback.mjs';
import { characterSwitchHref, sidebarSectionPath } from '../src/v3/workspace-navigation.mjs';
import { constraintMessage, installFieldValidation } from '../src/v3/proto/components/forms.js';
import { MessageBubble } from '../src/v3/proto/components/chat.js';
test('readable failures preserve Chinese business meaning and leave raw values untouched', () => {
  const error = new Error('Browser request failed: 401');
  assert.match(errorMessage(error), /检查密码/);
  assert.equal(error.message, 'Browser request failed: 401');
  assert.equal(errorMessage('请先保存世界，再继续创建'), '请先保存世界，再继续创建');
  assert.match(chatFailureText('回复未完成：request_timeout'), /稍后重试/);
  assert.doesNotMatch(chatFailureText('回复未完成：request_timeout'), /request_timeout/);
  assert.equal(errorMessage('unmapped_code', '读取失败，请重试'), '读取失败，请重试');
});
test('long waiting remains observable processing after thirty seconds without a fake failure', () => {
  const state = {
    hasPendingTurn: true
  };
  const text = chatWaitText(state, 1000, 126000);
  assert.equal(text, '对方 正在输入');
  assert.equal(chatWaitingPresentation(state, 1000, 30000).showDiagnostics, false);
  assert.equal(chatWaitingPresentation(state, 1000, 31000).detail, '对方还在想…');
  assert.equal(chatWaitingPresentation(state, 1000, 90000).showStop, false);
  assert.equal(chatWaitingPresentation(state, 1000, 91000, '雨禾').label, '雨禾 正在输入');
  assert.equal(chatWaitingPresentation(state, 1000, 91000).showStop, true);
  assert.equal(chatWaitText({}, 1000, 126000), '');
});
test('switching characters keeps list context and filters but never copies object or request identity', () => {
  assert.equal(characterSwitchHref('/c/a/growth/reflection?status=failed&q=coffee&message=old&conversation=old&returnTo=x', 'b'), '#/c/b/growth/reflection?status=failed&q=coffee');
  assert.equal(characterSwitchHref('/c/a/life/schedule?date=2026-10-22', 'b'), '#/c/b/life/schedule?date=2026-10-22');
  assert.equal(characterSwitchHref('/c/a/growth/memory/private-id', 'b'), '#/c/b/growth');
  assert.equal(characterSwitchHref('/feed?q=hello', 'b'), '#/feed?q=hello');
  assert.equal(sidebarSectionPath('/c/b/governance/history'), '/c/b/growth/history');
});
test('constraint messages are Chinese, preserve native ranges, and reject whitespace required values', () => {
  assert.equal(constraintMessage({
    required: true,
    value: '  ',
    validity: {}
  }), '请填写此项。');
  assert.match(constraintMessage({
    min: '0',
    validity: {
      rangeUnderflow: true
    }
  }), /不小于 0/);
  assert.match(constraintMessage({
    type: 'email',
    validity: {
      typeMismatch: true
    }
  }), /邮箱/);
});
test('native invalid capture marks controls inline, focuses first only, and clears only its own description', async () => {
  const old = globalThis.document,
    handlers = {},
    errors = [];
  let focus = '';
  globalThis.document = {
    createElement() {
      const error = {
        id: '',
        hidden: false,
        setAttribute() {}
      };
      errors.push(error);
      return error;
    }
  };
  const root = {
    addEventListener(type, handler) {
      handlers[type] = handler;
    },
    removeEventListener(type) {
      delete handlers[type];
    }
  };
  const control = name => ({
    required: true,
    value: '',
    validity: {
      valueMissing: true
    },
    isConnected: true,
    attributes: {
      'aria-describedby': 'help'
    },
    setAttribute(key, value) {
      this.attributes[key] = value;
    },
    getAttribute(key) {
      return this.attributes[key];
    },
    removeAttribute(key) {
      delete this.attributes[key];
    },
    after() {},
    focus() {
      focus = name;
    },
    scrollIntoView() {}
  });
  const first = control('first'),
    second = control('second'),
    stop = installFieldValidation(root);
  try {
    let prevented = 0;
    handlers.invalid({
      target: first,
      preventDefault() {
        prevented++;
      }
    });
    handlers.invalid({
      target: second,
      preventDefault() {
        prevented++;
      }
    });
    await Promise.resolve();
    assert.equal(prevented, 2);
    assert.equal(focus, 'first');
    assert.equal(first.attributes['aria-invalid'], 'true');
    assert.match(first.attributes['aria-describedby'], /^help constraint-/);
    handlers.input({
      target: first
    });
    assert.equal(first.attributes['aria-describedby'], 'help');
    assert.equal(errors[0].hidden, true);
    assert.equal(first.attributes['aria-invalid'], undefined);
  } finally {
    stop();
    globalThis.document = old;
  }
});
test('removed message copy and management source actions escape identifiers and preserve user text', () => {
  const message = {
    id: '"<id>',
    from: 'char',
    text: '<private> & message',
    at: '2026-10-22 12:00 (北京时间)'
  };
  const html = MessageBubble(message, {
    viewMode: '管理'
  });
  assert.doesNotMatch(html, /data-copy-message|复制消息/);
  assert.match(html, /data-source=/);
  assert.match(html, /&lt;private&gt; &amp; message/);
  assert.doesNotMatch(MessageBubble(message, {
    viewMode: '交流'
  }), /data-source=/);
});
test('footer reportValidity paths reject whitespace without clearing page custom constraints', () => {
  const handlers = {};
  const field = {
    required: true,
    willValidate: true,
    value: '   ',
    validity: {
      customError: false
    },
    validationMessage: '',
    setCustomValidity(message) {
      this.validationMessage = message;
      this.validity.customError = !!message;
    }
  };
  const root = {
    querySelectorAll() {
      return [field];
    },
    addEventListener(type, handler) {
      handlers[type] = handler;
    },
    removeEventListener(type) {
      delete handlers[type];
    }
  };
  const stop = installFieldValidation(root);
  try {
    handlers.click({
      type: 'click'
    });
    assert.equal(field.validationMessage, '请填写此项。');
    field.value = '有效输入';
    handlers.keydown({
      type: 'keydown',
      key: 'Enter'
    });
    assert.equal(field.validationMessage, '');
    field.setCustomValidity('此字段已有业务约束');
    field.value = '  ';
    handlers.click({
      type: 'click'
    });
    assert.equal(field.validationMessage, '此字段已有业务约束');
    field.setCustomValidity('');
    handlers.click({
      type: 'click'
    });
    field.setCustomValidity('后续业务校验失败');
    field.value = '有效';
    handlers.click({
      type: 'click'
    });
    assert.equal(field.validationMessage, '后续业务校验失败');
  } finally {
    stop();
  }
});
