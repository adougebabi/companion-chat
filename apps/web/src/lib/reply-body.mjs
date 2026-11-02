// Defense for older persisted replies and in-flight output. Core owns parsing
// before persistence; presentation must never fall back to the raw protocol.
const replyNames = ['最终回复', '回复正文', '回复内容', '对话回复', '正文', '回复', '答复', '回应', '回答', 'assistant reply', 'assistant response', 'final response', 'final answer', 'final', 'response', 'reply', 'answer'];
const internalNames = ['事务记录', '事務記錄', '事务日志', '事务', '事件记录', '认知更新', '认知分析', '认知记录', '认知', '思考过程', '内部思考', '思考', '分析', 'transaction records', 'transaction record', 'transaction ledger', 'transaction log', 'event log', 'action log', 'internal reasoning', 'internal thoughts', 'cognitive analysis', 'cognition', 'reasoning', 'analysis', 'thoughts', 'thought', 'ledger', 'metadata'];
const names = [...replyNames, ...internalNames].sort((a, b) => b.length - a.length);
const sectionName = names.join('|');
const headingPattern = new RegExp(`^(${sectionName})(?=$|[\\s:：（(】\\]*_#])([\\s\\S]*)$`, 'i');
const inlineHeadingPattern = new RegExp(`(\\S[ \\t]+)(#{1,6}[ \\t]*(?:${sectionName})(?=$|[ \\t:：]))`, 'gim');
const metadataPattern = /^\s*(?:[-*]\s*)?(?:\*\*)?(?:发言时间|发送时间|发送消息类型|消息类型|使用工具|工具使用|工具调用|关键数据|时间戳|timestamp|sent\s*(?:at|time)|message\s*type|tools?\s*used|tool calls?|key\s*data)(?:\*\*)?\s*[:：]/i;
const timestampPattern = /^\s*(?:[-*]\s*)?\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}(?::\d{2}(?:\.\d+)?)?(?:Z|[+-]\d{2}:?\d{2})?\s*$/;

/** @param {string} line */
function section(line) {
  let value = line.trim();
  const decorated = /^(?:#{1,6}\s*|>\s*|\*\*|__|【|\[)/.test(value);
  value = value.replace(/^[#>]+\s*/, '').replace(/^(?:[-*]\s+)?(?:(?:\d+|[一二三四五六七八九十]+)[.、)）]\s*)?/, '');
  let emphasis = /^(?:\*\*|__)/.exec(value)?.[0] || '';
  if (emphasis) value = value.slice(emphasis.length);
  let bracket = value.startsWith('【') ? '】' : value.startsWith('[') ? ']' : '';
  if (bracket) value = value.slice(1);
  const match = headingPattern.exec(value);
  if (!match) return null;
  let body = match[2].replace(/^\s*[（(](?:reply|response|cognition|reasoning|analysis|ledger|transaction log)(?:[）)]|$)/i, '').trim();
  function closeDecoration() {
    if (bracket && body.startsWith(bracket)) { body = body.slice(1).trimStart(); bracket = ''; }
    if (emphasis && body.startsWith(emphasis)) { body = body.slice(emphasis.length).trimStart(); emphasis = ''; }
  }
  closeDecoration();
  const colon = /^[:：]/.test(body);
  if (colon) { body = body.slice(1).trimStart(); closeDecoration(); }
  if (/^#+\s*$/.test(body)) body = '';
  if (body && !colon && !decorated) return null;
  return {reply: replyNames.includes(match[1].toLowerCase()), body};
}

/** Remove protocol XML and repair flattened sections only outside code samples.
 * @param {string[]} lines
 */
function cleanProtocolLines(lines) {
  const result = [];
  let codeFence = '', privateTag = '';
  for (const line of lines) {
    if (codeFence) {
      result.push(line);
      if (line.trim().startsWith(codeFence)) codeFence = '';
      continue;
    }
    const fence = /^\s*(`{3,}|~{3,})/.exec(line);
    if (!privateTag && fence) {
      codeFence = fence[1];
      result.push(line);
      continue;
    }
    let remaining = line, visible = '';
    while (remaining) {
      if (privateTag) {
        const closing = `</${privateTag}>`, index = remaining.toLowerCase().indexOf(closing);
        if (index < 0) break;
        remaining = remaining.slice(index + closing.length);
        privateTag = '';
      } else {
        const opening = /<(think|analysis|reasoning|cognition)>/i.exec(remaining);
        if (!opening) { visible += remaining; break; }
        visible += remaining.slice(0, opening.index);
        privateTag = opening[1].toLowerCase();
        remaining = remaining.slice(opening.index + opening[0].length);
      }
    }
    // Legacy notification bodies sometimes flattened the whole protocol.
    result.push(...visible.replace(inlineHeadingPattern, '$1\n$2').split('\n'));
  }
  return result;
}

/** Recover a top-level text string without displaying JSON syntax or private fields.
 * @param {string} text
 */
function envelopeBody(text) {
  /** @param {number} start */
  function stringEnd(start) {
    for (let index = start + 1; index < text.length; index++) {
      if (text[index] === '\\') index++;
      else if (text[index] === '"') return index + 1;
    }
    return -1;
  }
  let depth = 0;
  for (let index = 0; index < text.length; index++) {
    if (text[index] === '{' || text[index] === '[') depth++;
    else if (text[index] === '}' || text[index] === ']') depth--;
    else if (text[index] === '"') {
      const end = stringEnd(index);
      if (end < 0) return '';
      let key;
      try { key = JSON.parse(text.slice(index, end)); } catch { return ''; }
      if (depth === 1 && key === 'text') {
        const colon = /^\s*:\s*/.exec(text.slice(end));
        if (colon) {
          const start = end + colon[0].length;
          if (text[start] !== '"') return '';
          const close = stringEnd(start);
          let value = text.slice(start, close < 0 ? text.length : close);
          if (close < 0) {
            if ((/\\+$/.exec(value)?.[0].length || 0) % 2) value = value.slice(0, -1);
            value += '"';
          }
          value = value.replace(/\r/g, '\\r').replace(/\n/g, '\\n').replace(/\t/g, '\\t');
          try { return JSON.parse(value); } catch { return ''; }
        }
      }
      index = end - 1;
    }
  }
  return '';
}

/** Extract only user-facing text, preserving ordinary prose, Markdown and code.
 * @param {unknown} input
 * @returns {string}
 */
export function replyBodyText(input) {
  if (typeof input !== 'string') return '';
  let text = input.replace(/\r\n?/g, '\n').trim();
  // Legacy records may contain Writer's JSON envelope rather than its text.
  const json = text.replace(/^```json\s*\n?/i, '').replace(/\n?```\s*$/, '').trim();
  if (json.startsWith('{')) {
    try {
      const envelope = JSON.parse(json);
      if (envelope && typeof envelope.text === 'string') text = envelope.text;
      else return '';
    } catch {
      // Format failure still displays the recovered body through the same
      // protocol/private-section filter. Never return the raw envelope.
      text = envelopeBody(json);
    }
  }
  if (!/[\n#:：*_\[\]【】>]/.test(text) && replyNames.includes(text.trim().toLowerCase())) return text.trim();
  let lines = text.split('\n');
  const wrapped = /^\s*(?:```|~~~)(?:markdown|md|text|plaintext)?\s*$/i.test(lines[0] || '') && lines.some(line => section(line));
  if (wrapped) {
    lines = lines.slice(1);
    if (/^\s*(?:```|~~~)\s*$/.test(lines.at(-1) || '')) lines.pop();
  }
  lines = cleanProtocolLines(lines);
  let hasReply = false, scanFence = '';
  for (const line of lines) {
    const fence = /^\s*(`{3,}|~{3,})/.exec(line);
    if (fence) { scanFence = scanFence ? '' : fence[1]; continue; }
    if (!scanFence && section(line)?.reply) hasReply = true;
  }
  const chunks = [];
  let current = [], visible = !hasReply, codeFence = '';
  function flush() {
    const body = current.join('\n').trim();
    if (body && !chunks.includes(body)) chunks.push(body);
    current = [];
  }
  for (const line of lines) {
    if (codeFence) {
      if (visible) current.push(line);
      if (line.trim().startsWith(codeFence)) codeFence = '';
      continue;
    }
    const heading = section(line);
    if (heading) {
      flush();
      visible = heading.reply;
      if (visible && heading.body && !metadataPattern.test(heading.body)) current.push(heading.body);
      continue;
    }
    if (metadataPattern.test(line) || timestampPattern.test(line)) {
      flush();
      visible = false;
      continue;
    }
    const fence = /^\s*(`{3,}|~{3,})/.exec(line);
    if (fence) codeFence = fence[1];
    if (visible) current.push(line);
  }
  flush();
  return chunks.join('\n\n');
}
