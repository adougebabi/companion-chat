import test from 'node:test';
import assert from 'node:assert/strict';
import {configurationFields} from '../src/v3/proto/components/configuration-fields.js';

test('reply expression labels the current setting and retains distinct compatibility choices',()=>{
 const field=configurationFields['conversation.reply'].find(field=>field.key==='expression_version');
 assert.deepEqual(field.options.map(option=>option.value),['fact-only-reply.v8','fact-only-reply.v7','fact-only-reply.v6','fact-only-reply.v5','fact-only-reply.v4']);
 assert.equal(field.options[0].label,'自然交流 · 当前表达设定');
 assert.deepEqual(field.options.slice(1).map(option=>option.label),[
  '自然交流 · 兼容上一套设定',
  '自然交流 · 兼容较早设定',
  '自然交流 · 兼容早期设定',
  '自然交流 · 兼容最早设定',
 ]);
 for(const option of field.options)assert.doesNotMatch(option.label,/v\d+|协议|fact-only-reply/i);
});
