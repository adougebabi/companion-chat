import assert from 'node:assert/strict';
import test from 'node:test';
import { Field } from '../src/v3/proto/components/forms.js';
import { CharacterCard } from '../src/v3/proto/components/character-card.js';
test('field hints retain caller error descriptions and each control has an independent accessible identity', () => {
  const first = Field({
    label: '名称',
    name: 'name',
    hint: '帮助文字',
    attrs: {
      required: true,
      'aria-describedby': 'server-error'
    }
  });
  const second = Field({
    label: '名称',
    name: 'name',
    hint: '另一项帮助'
  });
  const id = first.match(/<input[^>]* id="([^"]+)"/)[1],
    other = second.match(/<input[^>]* id="([^"]+)"/)[1];
  assert.notEqual(id, other);
  assert.match(first, new RegExp(`aria-describedby="server-error ${id}-hint"`));
  assert.match(first, /aria-required="true"/);
});
test('a character without an image still has a named conversation entry and separate management controls', () => {
  const html = CharacterCard({
    id: 'person',
    name: '<角色>',
    avatar: '',
    status: '正常',
    groupId: 'ungrouped'
  }, []);
  assert.match(html, /aria-label="与&lt;角色&gt;对话"/);
  assert.match(html, /href="#\/c\/person\/chat"/);
  assert.match(html, /avatar-fallback/);
  assert.doesNotMatch(html, /<img[^>]*src=""/);
  assert.match(html, /data-lifecycle="person"/);
  assert.match(html, /data-grp="person"/);
});
