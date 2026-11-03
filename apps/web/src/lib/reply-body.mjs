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

// Match Core's fence rules: only the same marker with sufficient width and
// no trailing content closes a fence. Both boundary lines are protected code.
function markdownFence() {
  let marker = '', width = 0;
  /** @param {string} line */
  return line => {
    const match = /^ {0,3}(`+|~+)([\s\S]*)$/.exec(line);
    if (!match) return width > 0;
    const run = match[1], rest = match[2];
    if (width) {
      if (run[0] === marker && run.length >= width && !rest.trim()) width = 0;
      return true;
    }
    if (run.length >= 3 && (run[0] !== '`' || !rest.includes('`'))) {
      marker = run[0];
      width = run.length;
      return true;
    }
    return false;
  };
}

/** @param {string} line */
function indentedCodeLine(line) {
  return /^(?: {4}| *\t)/.test(line);
}

/** Trim prose edges without removing indentation or fenced code whitespace.
 * @param {string} text
 */
function trimReplyText(text) {
  const lines = text.split('\n');
  while (lines.length && !lines[0].trim()) lines.shift();
  const consumeFence = markdownFence();
  const protectedLines = lines.map(line => consumeFence(line) || indentedCodeLine(line));
  while (lines.length && !protectedLines[lines.length - 1] && !lines.at(-1).trim()) lines.pop();
  if (!lines.length) return '';
  if (!protectedLines[0]) lines[0] = lines[0].trimStart();
  if (!protectedLines[lines.length - 1]) lines[lines.length - 1] = lines.at(-1).trimEnd();
  return lines.join('\n');
}

/** Remove ATX heading delimiters, preserving hashes in prose and code.
 * @param {string} text
 */
function stripMarkdownHeadings(text) {
  const consumeFence = markdownFence();
  return text.split('\n').map(line => {
    if (consumeFence(line) || indentedCodeLine(line)) return line;
    const heading = /^( *)(#{1,6})(?=[ \t]|\r?$)[ \t]*([\s\S]*?)(\r?)$/.exec(line);
    if (!heading) return line;
    const body = heading[3].replace(/(?:[ \t]+|^)#+[ \t]*$/, '');
    return heading[1] + body + heading[4];
  }).join('\n');
}

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
  const consumeFence = markdownFence();
  let privateTag = '';
  for (const line of lines) {
    if (!privateTag && (consumeFence(line) || indentedCodeLine(line))) {
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
  let text = trimReplyText(input.replace(/\r\n?/g, '\n'));
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
  if (!/[\n#:：*_\[\]【】>]/.test(text) && replyNames.includes(text.trim().toLowerCase())) return text;
  let lines = text.split('\n');
  const wrapped = /^\s*(?:```|~~~)(?:markdown|md|text|plaintext)?\s*$/i.test(lines[0] || '') && lines.slice(1).some(line => section(line)?.reply === false);
  if (wrapped) {
    lines = lines.slice(1);
    if (/^\s*(?:```|~~~)\s*$/.test(lines.at(-1) || '')) lines.pop();
  }
  lines = cleanProtocolLines(lines);
  let hasReply = false;
  const scanFence = markdownFence();
  for (const line of lines) {
    if (scanFence(line) || indentedCodeLine(line)) continue;
    if (section(line)?.reply) hasReply = true;
  }
  const chunks = [];
  const consumeFence = markdownFence();
  let current = [], visible = !hasReply;
  function flush() {
    const body = trimReplyText(current.join('\n'));
    if (body && !chunks.includes(body)) chunks.push(body);
    current = [];
  }
  for (const line of lines) {
    if (consumeFence(line) || indentedCodeLine(line)) {
      if (visible) current.push(line);
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
    if (visible) current.push(line);
  }
  flush();
  return stripMarkdownHeadings(chunks.join('\n\n'));
}
