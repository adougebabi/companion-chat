import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { esc } from '../src/v3/proto/components/controls.js';
import { domainName, stateLabel, errorMessage } from '../src/v3/domain-presentation.mjs';
import { requestDecisionText, RequestDecisionSource, RequestFailureLink } from '../src/v3/proto/components/request-failure.js';
const tick = () => new Promise(resolve => setImmediate(resolve));
const storage = () => {
  const values = new Map();
  return {
    getItem: k => values.get(k) ?? null,
    setItem: (k, v) => values.set(k, v),
    removeItem: k => values.delete(k)
  };
};
function load(file, bindings, name) {
  const text = readFileSync(new URL('../src/v3/proto/' + file, import.meta.url), 'utf8').replace(/^import .+;\s*$/gm, '').replace(/^export \{.+;\s*$/gm, '').replaceAll('export ', '');
  return Function(...Object.keys(bindings), text + '\nreturn ' + name)(...Object.values(bindings));
}
function element() {
  const nodes = new Map();
  return {
    isConnected: true,
    innerHTML: '',
    querySelector(s) {
      if (!nodes.has(s)) nodes.set(s, {
        isConnected: true,
        innerHTML: '',
        textContent: '',
        hidden: false,
        disabled: false,
        insertAdjacentHTML(_where, text) {
          this.innerHTML += text;
        },
        addEventListener() {}
      });
      return nodes.get(s);
    },
    querySelectorAll: () => []
  };
}
test('inline progress follows original request, localizes tasks, keeps state on read failure and ignores changed Owner', async () => {
  const previous = globalThis.document;
  const handlers = new Map();
  globalThis.document = {
    hidden: false,
    addEventListener: (kind, fn) => handlers.set(kind, fn),
    removeEventListener: kind => handlers.delete(kind)
  };
  let actor = 'owner',
    request = {
      id: 'saved-request',
      status: 'pending',
      tasks: [{
        kind: 'cognition.processing',
        status: 'started',
        task_id: 'task'
      }]
    },
    fail = false,
    finished = 0;
  const reads = [];
  const host = element();
  const requestReceipt = load('components/async-page.js', {
    get: () => ({
      actorId: actor
    }),
    esc,
    drawer: () => {
      throw Error('must stay inline');
    },
    domainName,
    stateLabel,
    errorMessage,
    JsonDetails:(value,title)=>'<details>'+esc(title)+'<pre>'+esc(JSON.stringify(value))+'</pre></details>',
    requestDecisionText,
    RequestDecisionSource,
    RequestFailureLink,
    InventoryEffects: () => '',
    setTimeout: () => 1,
    clearTimeout() {},
    lifeClient: {
      request: async (fid, id) => {
        reads.push([fid, id]);
        if (fail) throw Error('fetch failed');
        return request;
      }
    }
  }, 'requestReceipt');
  try {
    const panel = requestReceipt('role', {
      request
    }, () => finished++, {
      container: host
    });
    assert.match(host.querySelector('[data-request]').innerHTML, /回应处理.*已开始/);
    fail = true;
    host.querySelector('[data-query]').onclick();
    await tick();
    assert.match(host.querySelector('[data-request]').innerHTML, /更新失败，上次状态已保留/);
    assert.match(host.querySelector('[data-request]').innerHTML, /等她回应/);
    fail = false;
    request = {
      ...request,
      status: 'completed'
    };
    host.querySelector('[data-query]').onclick();
    await tick();
    assert.equal(finished, 1);
    assert.deepEqual(reads, [['role', 'saved-request'], ['role', 'saved-request']]);
    const html = host.querySelector('[data-request]').innerHTML;
    actor = 'other';
    request = {
      ...request,
      status: 'failed'
    };
    host.querySelector('[data-query]').onclick();
    await tick();
    assert.equal(host.querySelector('[data-request]').innerHTML, html);
    panel.dispose();
    assert.equal(handlers.size, 0);
  } finally {
    globalThis.document = previous;
  }
});
test('outfit action reads fresh wardrobe and outfit versions at click time', async () => {
  const box = element(),
    wear = {
      isConnected: true,
      dataset: {
        wear: 'plan'
      },
      disabled: false
    };
  box.querySelectorAll = s => s === '[data-wear]' ? [wear] : [];
  let currentReads = 0,
    submitted;
  const mount = load('components/outfit-library.js', {
    inventoryClient: {
      tags: async () => ({
        tags: []
      }),
      current: async () => ({
        revision: ++currentReads === 1 ? 1 : 9
      }),
      outfits: async () => ({
        profile_id: 'profile',
        items: [{
          id: 'plan',
          revision: 2
        }],
        next_cursor: ''
      }),
      outfit: async () => ({
        id: 'plan',
        revision: 8,
        active_profile_id: 'profile'
      })
    },
    esc,
    drawer() {},
    toast() {},
    OutfitPreview: () => '',
    requestInventoryAction: (...args) => submitted = args
  }, 'mountOutfitLibrary');
  const panel = mount(box, {
    fid: 'role'
  });
  await tick();
  await wear.onclick();
  assert.equal(currentReads, 2);
  assert.equal(submitted[3], 9);
  assert.deepEqual(submitted[2], {
    outfitId: 'plan',
    expectedOutfitRevision: 8,
    profileId: 'profile'
  });
  panel.dispose();
});
test('slow restoration of an old suggestion cannot replace a freshly submitted progress panel', async () => {
  const saved = storage();
  saved.setItem('fluctlight-studio:owner:moment:suggest:global:last', JSON.stringify({
    id: 'old',
    fid: 'role'
  }));
  let release;
  const delayed = new Promise(resolve => release = resolve),
    shown = [];
  const host = element(),
    form = host.querySelector('form');
  form.elements = {
    text: host.querySelector('[name=text]'),
    fid: {
      value: 'role'
    }
  };
  form.reportValidity = () => true;
  const mount = load('components/moment-composer.js', {
    get: () => ({
      actorId: 'owner'
    }),
    list: () => [],
    studioDraftStorage: () => saved,
    persistentCommand: () => ({
      pending: () => null,
      submit: async () => ({
        request: {
          id: 'new',
          status: 'pending'
        }
      })
    }),
    esc,
    Field: () => '',
    errorMessage,
    stateLabel,
    toast() {},
    requestReceipt: (_fid, envelope) => {
      shown.push(envelope.request.id);
      return {
        dispose() {}
      };
    },
    lifeClient: {
      request: () => delayed
    },
    momentsClient: {},
    mediaClient: {},
    mountCursorRecords() {},
    drawer() {},
    confirm() {}
  }, 'mountMomentComposer');
  const panel = mount(host, {
    mode: 'suggest'
  });
  form.elements.fid.onchange();
  form.elements.text.value = '新建议';
  form.elements.text.oninput();
  form.onsubmit({
    preventDefault() {}
  });
  await tick();
  assert.deepEqual(shown, ['new']);
  release({
    id: 'old',
    status: 'failed'
  });
  await tick();
  assert.deepEqual(shown, ['new']);
  panel.dispose();
});
