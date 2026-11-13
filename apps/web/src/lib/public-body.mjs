// Presentation-only guards for historical records whose stored text contains
// model wrappers or leaked private prompts. Stored text stays unchanged, but
// every visible layer (including folded technical records and their download)
// shows the cleaned projection, never the wrapped original.
import {replyBodyText} from './reply-body.mjs';

// Private chat protocol / persona context that must never reach a summary.
const promptLeakPattern = /运行协议|working_persona|interpretation_boundary|portrait_text|persona\.detail|\{\s*"(?:working_persona|interpretation_boundary|portrait_text|persona|fluctlight|context|instructions?)"\s*:/;
// Assistant self-descriptions that mean the model answered outside the persona.
const assistantLeakPattern = /人工智能助手|AI\s*助手|作为\s*(?:一个|一名)?\s*(?:AI|人工智能|语言模型)|(?:大)?语言模型|协议相关指令|协议测试/i;

export const NOTICE_SUMMARY_LIMIT = 120;
export const HIDDEN_NOTICE_BODY = '这条消息包含内部内容，已在通知中隐藏，可打开原对话查看。';

/** Strip inline Markdown decoration while keeping the words.
 * @param {string} text
 */
function plainMarkdown(text) {
  return text
    .replace(/^\s{0,3}#{1,6}[ \t]+/gm, '')
    .replace(/^\s{0,3}>[ \t]?/gm, '')
    .replace(/^\s{0,3}(?:[-*_][ \t]*){3,}$/gm, '')
    .replace(/(\*\*|__)(?=\S)([\s\S]*?\S)\1/g, '$2')
    .replace(/`([^`\n]+)`/g, '$1');
}

/** True when text carries the private chat protocol, raw JSON keys or assistant voice.
 * @param {unknown} text
 */
export function hasPrivatePromptLeak(text) {
  const value = String(text || '');
  return promptLeakPattern.test(value) || assistantLeakPattern.test(value);
}

/** Full readable notice body for the detail dialog.
 * @param {unknown} body
 * @param {boolean} reply whether the body is a published conversation reply
 */
export function noticeBodyText(body, reply) {
  const text = reply ? replyBodyText(body) : String(body ?? '');
  if (hasPrivatePromptLeak(text)) return HIDDEN_NOTICE_BODY;
  return text;
}

/** One-line, length-bounded summary for a notification card.
 * @param {unknown} body
 * @param {boolean} reply
 */
export function noticeSummaryText(body, reply) {
  const text = plainMarkdown(noticeBodyText(body, reply)).replace(/\s+/g, ' ').trim();
  const chars = [...text];
  return chars.length > NOTICE_SUMMARY_LIMIT ? chars.slice(0, NOTICE_SUMMARY_LIMIT).join('').trimEnd() + '…' : text;
}

const horizontalRule = /^\s{0,3}(?:[-*_][ \t]*){3,}$/;
// "我正在生成一条……动态……我将发布以下内容：" style generation preambles.
const preamblePattern = /我(?:正在|将|会|来)?(?:为你)?(?:生成|撰写|创作|编写)(?:一条|一则|以下)|面向动态参与者|(?:将|会)发布以下内容|以下(?:是|为)?.{0,16}(?:动态|内容|正文)\s*[:：]\s*$/;
// Trailing generation notes describing how the moment was produced.
const generationNotePattern = /(?:这条|本条|该|此)(?:动态|内容|正文).{0,24}(?:基于|结合|根据|参考)|动态正文.{0,8}生成|未经授权的(?:图片|媒体)|生成说明|(?:符合|贴合).{0,12}(?:人格|人设)设定/;
// "**FT-苏晴** 20:08 · 图书馆日常" header lines copied from the feed layout.
const authorHeaderPattern = /^\s*(?:\*\*|__)[^*_\n]{1,40}(?:\*\*|__)\s*(?:\d{1,2}[:：]\d{2}|[·•｜|])[^\n]{0,30}$/;


/** Character-voice body of a published moment, without model wrappers.
 * @param {unknown} input
 */
export function momentBodyText(input) {
  const original = String(input ?? '').replace(/\r\n?/g, '\n');
  // Paragraphs separated by blank lines or Markdown horizontal rules.
  const blocks = [];
  let current = [];
  for (const line of original.split('\n')) {
    if (!line.trim() || horizontalRule.test(line)) {
      if (current.length) blocks.push(current.join('\n'));
      current = [];
    } else current.push(line);
  }
  if (current.length) blocks.push(current.join('\n'));
  // Only leading preambles and trailing generation notes are wrappers; the
  // same words in the middle of character prose are left untouched.
  while (blocks.length && preamblePattern.test(blocks[0])) blocks.shift();
  while (blocks.length && generationNotePattern.test(blocks.at(-1))) blocks.pop();
  const kept = blocks
    .map(block => block.split('\n').filter((line, index) => index > 0 || !authorHeaderPattern.test(line)).join('\n'))
    .filter(block => block.trim());
  const text = plainMarkdown(kept.join('\n\n')).replace(/\n{3,}/g, '\n\n').trim();
  return text || plainMarkdown(original).trim();
}
