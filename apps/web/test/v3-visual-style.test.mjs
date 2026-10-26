import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
function load(path, names, bindings) {
  const source = readFileSync(new URL(path, import.meta.url), 'utf8').replace(/^import .+;\s*$/gm, '').replaceAll('export ', '');
  return Function(...Object.keys(bindings), source + `\nreturn {${names.join(',')}};`)(...Object.values(bindings));
}
const fields = ['appearance', 'clothing', 'constraints', 'references'].map(key => [key, key, '']);
test('visual comparison shows active-reference copy only for an actual canonical asset', () => {
  const {
    VisualIdentityState
  } = load('../src/v3/proto/components/visual-identity-records.js', ['VisualIdentityState'], {
    stateLabel: value => value,
    esc: value => String(value ?? ''),
    badge: value => String(value),
    JsonDetails: () => '',
    instant: value => value,
    InstantLabel: () => ''
  });
  const empty = VisualIdentityState({
    status: 'not_created',
    current_revision: 0
  });
  assert.match(empty, /尚无已接受的规范参考/);
  assert.doesNotMatch(empty, /当前已生效参考/);
  const ready = VisualIdentityState({
    status: 'active',
    current_revision: 2,
    canonical_asset_id: 'accepted-original',
    session: {
      status: 'running',
      current_attempt: 2,
      attempts: [{
        attempt_number: 1,
        candidate_asset_id: 'old-candidate',
        status: 'completed'
      }, {
        attempt_number: 2,
        status: 'running'
      }]
    }
  });
  assert.match(ready, /当前已生效参考/);
  assert.match(ready, /accepted-original/);
  const currentPane = ready.split('<section class="visual-comparison-pane">')[2].split('</section>')[0];
  assert.doesNotMatch(currentPane, /old-candidate/);
  assert.match(currentPane, /当前尝试尚无可读取的候选/);
});
test('invalid advanced JSON becomes submittable after a normal field repairs its value', () => {
  const nodes = Object.fromEntries([...fields.map(([key]) => key), 'visualStyleJson'].map(key => [key, {
    value: '',
    error: '',
    setCustomValidity(value) {
      this.error = value;
    }
  }]));
  const box = {
      querySelector: selector => nodes[selector.match(/name=(\w+)/)[1]]
    },
    body = {
      schema: 'visual-style.v1',
      appearance: 'old'
    };
  let changes = 0;
  const {
    bindVisualStyle
  } = load('../src/v3/proto/components/visual-style.js', ['bindVisualStyle'], {
    visualBriefFields: fields
  });
  bindVisualStyle(box, body, {
    changed: () => changes++
  });
  nodes.visualStyleJson.value = '{broken';
  nodes.visualStyleJson.oninput({
    target: nodes.visualStyleJson
  });
  assert.ok(nodes.visualStyleJson.error);
  nodes.appearance.value = 'repaired guidance';
  nodes.appearance.oninput({
    target: nodes.appearance
  });
  assert.equal(nodes.visualStyleJson.error, '');
  assert.equal(JSON.parse(nodes.visualStyleJson.value).appearance, 'repaired guidance');
  assert.equal(changes, 1);
});
test('visual brief history compares authored fields and keeps null as missing', () => {
  const {
    VisualBriefDiff
  } = load('../src/v3/proto/components/visual-identity-records.js', ['VisualBriefDiff'], {
    esc: value => String(value ?? '')
  });
  const initial = VisualBriefDiff(null, {
    appearance: '短发',
    clothing: '青色外套'
  });
  assert.match(initial, /修改前：尚未填写/);
  assert.match(initial, /修改后：短发/);
  assert.doesNotMatch(initial, /null/);
  const same = VisualBriefDiff({
    appearance: '短发'
  }, {
    appearance: '短发'
  });
  assert.match(same, /没有变化/);
});
