import assert from 'node:assert/strict';
import test from 'node:test';
import {readFile} from 'node:fs/promises';
import {replyBodyText} from '../src/lib/reply-body.mjs';
import {activeChatFixture} from './support/active-chat.mjs';

const sharedSamples = JSON.parse(await readFile(new URL('../../core-go/internal/core/testdata/reply_output_drift.json', import.meta.url), 'utf8'));
for (const sample of sharedSamples) {
  test(`shared Go/JS drift contract: ${sample.name}`, () => {
    assert.equal(replyBodyText(sample.input), sample.want);
    assert.equal(replyBodyText(sample.want), sample.want, 'filter remains idempotent after Core parsing');
  });
}

const auditReply = `# 回复
当然可以啊，别客气。💙

（轻轻地喝了口水，调整坐姿更舒服一些）

累的时候多聊聊天是最好的方式了。你今天遇到什么烦心事了吗？还是 simply 就想找个人陪你说说话？我都在这里陪着你。

# 事务记录
- 发言时间：2026-11-02T20:52:47.651+08:00
- 发送消息类型：普通文字
- 使用工具：无
- 关键数据：无需保留特殊数据`;
const expectedAuditReply = `当然可以啊，别客气。💙

（轻轻地喝了口水，调整坐姿更舒服一些）

累的时候多聊聊天是最好的方式了。你今天遇到什么烦心事了吗？还是 simply 就想找个人陪你说说话？我都在这里陪着你。`;

for (const [name, raw, expected] of [
  ['audit chat reply excludes transaction records', auditReply, expectedAuditReply],
  ['English headings discard cognition and ledger', '## Cognition\nThe user needs reassurance.\n## Assistant Reply\nI am here. **Take your time.**\n## Transaction Log\nTools used: none', 'I am here. **Take your time.**'],
  ['duplicates do not repeat reply text', '# 回复\n今天慢慢来。\n# 认知\ninternal\n# 回复\n今天慢慢来。\n# 事务记录\nprivate', '今天慢慢来。'],
  ['distinct reply sections degrade to joined plain body', '# 回复\n你好。\n# 回复\n我在这里。\n# 事务日志\nprivate', '你好。\n\n我在这里。'],
  ['missing reply heading stops at internal content', '这会儿有点凉，记得加件外套。\n认知：\ndo not expose\n事务记录：\n使用工具：无', '这会儿有点凉，记得加件外套。'],
  ['explicit reply ignores preceding protocol preamble', 'Here is my structured response.\n### Reply:\nHello!\n### Analysis\nsecret', 'Hello!'],
  ['empty reply never falls back to private text', '# 回复\n# 认知\n内部认知\n# 事务记录\n使用工具：无', ''],
  ['only cognition has no visible body', '# Cognition\nsecret\n# Transaction Ledger\nprivate', ''],
  ['fenced protocol unwraps safely', '```markdown\n## Final Answer\nI can help.\n## Reasoning\nsecret\n## Ledger\nprivate\n```', 'I can help.'],
  ['bold and bracketed headings', '**回复**：你好。\n【认知】\nsecret\n【事务记录】\nprivate', '你好。'],
  ['bilingual headings', '> 回复（Reply）：你好。\n> 认知（Cognition）\nsecret\n> 事务记录（Ledger）\nprivate', '你好。'],
  ['inline reply preserves bold body', '# Reply: **Hello.**\n# Cognition\nprivate', '**Hello.**'],
  ['inline reply preserves bracketed body', '# Reply: [hello]\n# Ledger\nprivate', '[hello]'],
  ['bold label may enclose the colon', '**回复：** 你好。', '你好。'],
  ['flattened historical inbox body', '# 回复 当然可以啊，别客气。 # 认知 内部认知 # 回复 当然可以啊，别客气。 # 事务记录 - 使用工具：无', '当然可以啊，别客气。'],
  ['plain body metadata removed', '当然可以。\n- 发言时间：2026-11-02T20:57:47+08:00\n- 使用工具：无\n2026-11-02T20:57:47+08:00', '当然可以。'],
  ['normal Markdown, English and code are preserved', '# 周末计划\nLet’s go hiking.\n\n- **带水**\n\n```js\nconsole.log("hello");\n```', '周末计划\nLet’s go hiking.\n\n- **带水**\n\n```js\nconsole.log("hello");\n```'],
  ['ordinary fenced XML and inline protocol comments stay literal', '# 回复\nHTML 示例：\n```html\n<analysis>keep this example</analysis>\n<!-- before # 回复 keep this comment # 认知 literal -->\n```\n# 认知\nprivate', 'HTML 示例：\n```html\n<analysis>keep this example</analysis>\n<!-- before # 回复 keep this comment # 认知 literal -->\n```'],
  ['private XML takes priority over fences inside its content', '<think>\n```html\nprivate reasoning\n```\n</think>\nHello.', 'Hello.'],
  ['ordinary reply prose is not mistaken for a heading', '回复你之前，我想先问问。\nResponse times vary.\n分析这个问题需要一些时间。', '回复你之前，我想先问问。\nResponse times vary.\n分析这个问题需要一些时间。'],
]) {
  test(name, () => assert.equal(replyBodyText(raw), expected));
}

for (const [name, raw, expected] of [
  ['all six ATX levels', '# 一\n## 二\n### 三\n#### 四\n##### 五\n###### 六', '一\n二\n三\n四\n五\n六'],
  ['heading indentation and body whitespace', '前文。\n  ## \t标题  \n段落  内空格\t保留。', '前文。\n  标题  \n段落  内空格\t保留。'],
  ['optional closing hashes', '### 标题 ###  \n# C#\n## 标题 #tag\n#\t制表符\t##', '标题\nC#\n标题 #tag\n制表符'],
  ['empty heading delimiters', '前文\n#\n## \n### ###\n原正文。', '前文\n\n\n\n原正文。'],
  ['CRLF headings', '# 标题\r\n原正文。\r\n## 后续', '标题\n原正文。\n后续'],
  ['hashtags, escaped hashes and inline code', '#话题\n####### 七个不是标题\n正文 # 标记\nC#\n\\# 转义\n`# 内联代码`\n`# Reply`', '#话题\n####### 七个不是标题\n正文 # 标记\nC#\n\\# 转义\n`# 内联代码`\n`# Reply`'],
  ['other Markdown', '- 列表\n> # 引用\n**加粗**\n标题\n===\n[链接](https://example.com/#anchor)', '- 列表\n> # 引用\n**加粗**\n标题\n===\n[链接](https://example.com/#anchor)'],
  ['backtick code inside a reply section', '# 回复\n# 标题\n```md\n# 保留\n###### 也保留\n```\n## 后续\n# 认知\nprivate', '标题\n```md\n# 保留\n###### 也保留\n```\n后续'],
  ['tilde code', '~~~md\n## 保留\n~~~\n# 后续', '~~~md\n## 保留\n~~~\n后续'],
  ['indented code and three-space heading', '示例：\n    # Reply\n    ## 注释\n\t## 注释\n   ### 标题', '示例：\n    # Reply\n    ## 注释\n\t## 注释\n   标题'],
  ['standalone indented code', '    # Reply\n    ## 注释\n\t# 注释', '    # Reply\n    ## 注释\n\t# 注释'],
  ['code at prose whitespace boundaries', '\n   ```python\n# 注释  \n```  \n\n', '   ```python\n# 注释  \n```  '],
  ['standalone fenced reply label stays code', '```markdown\n# Reply\n## 标题作为代码保留\n```', '```markdown\n# Reply\n## 标题作为代码保留\n```'],
  ['fence width, marker and trailing text', '````md\n```\n# Reply\n# 保留\n~~~\n## 保留\n```` trailing\n### 保留\n`````\n# 后续', '````md\n```\n# Reply\n# 保留\n~~~\n## 保留\n```` trailing\n### 保留\n`````\n后续'],
  ['indented fence boundaries', '   ~~~md\n# 保留\n   ~~~~\n# 后续', '   ~~~md\n# 保留\n   ~~~~\n后续'],
  ['unclosed fence and trailing whitespace', '原正文。\n```md\n# 保留\n## 保留\n  \n', '原正文。\n```md\n# 保留\n## 保留\n  \n'],
  ['backtick in fence info is not a fence', '```md`literal\n# 标题', '```md`literal\n标题'],
  ['JSON envelope heading', '{"text":"# 今天的事\\n正文。"}', '今天的事\n正文。'],
  ['JSON envelope indented code', '{"text":"    # 注释"}', '    # 注释'],
  ['malformed envelope heading', '{"text":"## 今天的事\\n正文。"', '今天的事\n正文。'],
]) {
  test(`reply Markdown cleanup: ${name}`, () => {
    assert.equal(replyBodyText(raw), expected);
    assert.equal(replyBodyText(expected), expected, 'cleanup remains idempotent');
  });
}

test('active chat filters historical assistant output but preserves the user message and stored text', async () => {
  const fixture = await activeChatFixture();
  try {
    await fixture.update(() => {
      fixture.store.messages = [
        {id: 'user', kind: 'user', text: '# 认知\n这是我的原文。', createdAt: '2026-11-02T12:57:47Z'},
        {id: 'assistant', kind: 'assistant', text: auditReply, createdAt: '2026-11-02T12:57:48Z'},
      ];
    });
    const bubbles = fixture.view.host.querySelectorAll('.companion-bubble');
    assert.equal(bubbles[0].textContent, '# 认知\n这是我的原文。');
    assert.equal(bubbles[1].textContent, expectedAuditReply);
    assert.doesNotMatch(bubbles[1].textContent, /事务记录|发言时间|使用工具/);
    assert.equal(fixture.store.messages[1].text, auditReply);
  } finally {
    await fixture.dispose();
  }
});

test('active chat displays clean fallback bodies from malformed and ambiguous assistant output', async () => {
  const fixture = await activeChatFixture();
  const samples = sharedSamples.filter(sample => ['malformed_json_missing_brace', 'malformed_json_duplicate_reply', 'unclosed_bilingual_headings'].includes(sample.name));
  try {
    await fixture.update(() => {
      fixture.store.messages = samples.map((sample, index) => ({id: sample.name, kind: 'assistant', text: sample.input, createdAt: `2026-11-02T12:57:4${index}Z`}));
    });
    const bubbles = fixture.view.host.querySelectorAll('.companion-bubble');
    assert.equal(bubbles.length, samples.length);
    samples.forEach((sample, index) => {
      assert.equal(bubbles[index].textContent, sample.want);
      assert.notEqual(bubbles[index].textContent, sample.input);
      assert.doesNotMatch(bubbles[index].textContent, /#|private|"text"|认知|事务记录/);
      assert.equal(fixture.store.messages[index].text, sample.input);
    });
  } finally {
    await fixture.dispose();
  }
});
