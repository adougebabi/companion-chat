import test from 'node:test';
import assert from 'node:assert/strict';
import {configurationFields} from '../src/v3/proto/components/configuration-fields.js';

test('reply expression offers v8 user protocol and retains frozen v7/v6/v5/v4 choices',()=>{
 const field=configurationFields['conversation.reply'].find(field=>field.key==='expression_version');
 assert.deepEqual(field.options.map(option=>option.value),['fact-only-reply.v8','fact-only-reply.v7','fact-only-reply.v6','fact-only-reply.v5','fact-only-reply.v4']);
 assert.match(field.options[0].label,/v8 当前协议/);
 for(const option of field.options.slice(1))assert.match(option.label,/兼容协议/);
});
