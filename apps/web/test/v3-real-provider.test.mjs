import test from 'node:test';
import assert from 'node:assert/strict';
import * as store from '../src/v3/proto/store.js';
test('real facade starts without fixture facts and refuses arbitrary local writes', () => {
  assert.equal(store.isMock, false);
  assert.deepEqual(store.list('characters'), []);
  const snapshot = store.get();
  let called = false;
  assert.throws(() => store.update(data => {
    called = true;
    data.session = {
      authenticated: true
    };
  }, {
    identity: '系统运维'
  }), /尚未连接真实命令/);
  assert.equal(called, false);
  assert.equal(store.get(), snapshot);
  assert.equal(store.get('session'), null);
});
test('snapshot selection follows available server identities and never defaults to fixture c1', () => {
  store.replaceSnapshot({
    ...store.get(),
    characters: {
      'real-a': {
        id: 'real-a'
      }
    },
    ui: {
      charId: 'real-a',
      identity: '交流',
      notes: false
    }
  });
  assert.equal(store.get('ui').charId, 'real-a');
  store.setUi({
    charId: 'deleted'
  });
  store.replaceSnapshot({
    ...store.get(),
    characters: {
      'real-b': {
        id: 'real-b'
      }
    },
    ui: {
      charId: 'real-b',
      identity: '交流',
      notes: false
    }
  });
  assert.equal(store.get('ui').charId, 'real-b');
  assert.equal(store.find('c1'), null);
});
