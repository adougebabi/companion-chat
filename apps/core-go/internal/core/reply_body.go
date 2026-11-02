package core

import (
	"regexp"
	"strings"
)

var replySectionNumber = regexp.MustCompile(`^(?:[0-9]+|[一二三四五六七八九十]+)[.、)）]\s*`)
var replyPrivateXML = regexp.MustCompile(`(?is)<(?:think|analysis|reasoning|cognition)>.*?(?:</(?:think|analysis|reasoning|cognition)>|$)`)
var replyLedgerField = regexp.MustCompile(`(?i)^(?:[-*]\s*)?(?:发言时间|发送时间|发送消息类型|消息类型|使用工具|工具使用|工具调用|关键数据|时间戳|sent\s*(?:at|time)|timestamp|message\s*type|tools?\s*used|tool\s*calls?|key\s*data)\s*[:：]`)
var replyProtocolWrapper = regexp.MustCompile("(?i)^(?:```|~~~)(?:markdown|md|text|plaintext)?$")
var replyBilingualSuffix = regexp.MustCompile(`(?i)\s*[（(](?:reply|response|cognition|reasoning|analysis|ledger|transaction log)[）)]\s*$`)
var replyTimestamp = regexp.MustCompile(`^(?:[-*]\s*)?\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}(?::\d{2}(?:\.\d+)?)?(?:Z|[+-]\d{2}:?\d{2})?$`)
var replyInlineHeading = regexp.MustCompile(`(?i)(\S[ \t]+)(#{1,6}[ \t]*(?:事务记录|事务日志|认知分析|认知记录|回复正文|回复内容|最终回复|认知|回复|思考|分析|transaction records|transaction record|transaction ledger|transaction log|cognitive analysis|assistant reply|assistant response|final answer|final response|cognition|reasoning|analysis|metadata|thoughts|ledger|response|reply|answer)(?:[ \t:：]|$))`)

// replySection recognizes protocol labels, not arbitrary Markdown headings or
// English words in prose. Internal blocks stay private until an explicit reply.
func replySection(line string) (string, string) {
	line = strings.TrimSpace(line)
	decorated := strings.HasPrefix(line, "#") || strings.HasPrefix(line, "**") || strings.HasPrefix(line, "【") || strings.HasPrefix(line, "[")
	line = strings.TrimSpace(strings.TrimLeft(line, "#>"))
	line = strings.TrimLeft(line, "*_[]【】 \t")
	line = replySectionNumber.ReplaceAllString(line, "")
	line = strings.TrimLeft(line, "*_[]【】 \t")
	title, inline := line, ""
	if index := strings.IndexAny(title, ":："); index >= 0 {
		title, inline = title[:index], strings.TrimSpace(strings.TrimLeft(title[index:], ":："))
		for _, close := range []string{"** ", "__ "} {
			inline = strings.TrimSpace(strings.TrimPrefix(inline, close))
		}
	}
	title = strings.ToLower(strings.Trim(title, "*_#[]【】 \t"))
	title = strings.TrimSpace(replyBilingualSuffix.ReplaceAllString(title, ""))
	if decorated {
		for _, label := range []string{"transaction records", "transaction record", "transaction ledger", "transaction log", "cognitive analysis", "assistant response", "assistant reply", "final response", "final answer", "认知分析", "认知记录", "事务记录", "事务日志", "回复正文", "回复内容", "最终回复", "cognition", "reasoning", "analysis", "response", "metadata", "thoughts", "ledger", "answer", "reply", "回复", "认知", "思考", "分析"} {
			if strings.HasPrefix(title, label+" ") {
				kind, _ := replySection(label)
				return kind, strings.TrimSpace(line[len(label):])
			}
		}
	}
	switch title {
	case "回复", "回复正文", "回复内容", "最终回复", "正文", "答复", "回应", "回答", "answer", "reply", "response", "final", "final answer", "final response", "assistant reply", "assistant response":
		return "reply", inline
	case "认知", "认知记录", "认知分析", "思考", "思考过程", "分析", "内部思考", "cognition", "cognitive analysis", "reasoning", "analysis", "thought", "thoughts", "internal thoughts", "事务记录", "事务日志", "事务", "ledger", "transaction ledger", "transaction log", "transaction record", "transaction records", "metadata":
		return "private", ""
	}
	return "", ""
}

func replyFence(line string) bool {
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~")
}

func stripReplyPrivateXML(text string) string {
	matches := replyPrivateXML.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return text
	}
	// Only the start of an XML block determines whether it is protocol or a
	// literal code example. An internal block may itself contain fenced code.
	code := make([]bool, len(text))
	fenced, offset := false, 0
	for _, line := range strings.SplitAfter(text, "\n") {
		isFence := replyFence(line)
		if fenced || isFence {
			for i := offset; i < offset+len(line); i++ {
				code[i] = true
			}
		}
		if isFence {
			fenced = !fenced
		}
		offset += len(line)
	}
	var result strings.Builder
	previous := 0
	for _, match := range matches {
		if code[match[0]] {
			continue
		}
		result.WriteString(text[previous:match[0]])
		previous = match[1]
	}
	result.WriteString(text[previous:])
	return result.String()
}

// extractReplyBody is used both at Writer decoding and at the shared message
// publication boundary. It never falls back to the raw envelope on ambiguity.
// Multiple reply sections become plain paragraphs; exact repeats are deduped.
func extractReplyBody(text string) string {
	text = strings.TrimSpace(stripReplyPrivateXML(strings.ReplaceAll(text, "\r\n", "\n")))
	lines := strings.Split(text, "\n")
	// Some small models fence the entire protocol. Ordinary fenced code remains
	// unchanged; only unwrap when the wrapper actually contains protocol labels.
	if len(lines) > 1 && replyProtocolWrapper.MatchString(strings.TrimSpace(lines[0])) {
		protocol := false
		for _, line := range lines[1:] {
			if section, _ := replySection(line); section != "" {
				protocol = true
			}
		}
		if protocol {
			lines = lines[1:]
			if len(lines) > 0 && replyFence(lines[len(lines)-1]) {
				lines = lines[:len(lines)-1]
			}
			lines = strings.Split(stripReplyPrivateXML(strings.Join(lines, "\n")), "\n")
		}
	}
	var expanded []string
	fenced := false
	for _, line := range lines {
		if replyFence(line) {
			fenced = !fenced
		}
		if !fenced {
			line = replyInlineHeading.ReplaceAllString(line, "$1\n$2")
		}
		expanded = append(expanded, strings.Split(line, "\n")...)
	}
	lines = expanded
	hasReply := false
	fenced = false
	for _, line := range lines {
		if replyFence(line) {
			fenced = !fenced
			continue
		}
		if section, _ := replySection(line); !fenced && section == "reply" {
			hasReply = true
		}
	}
	active, fenced := !hasReply, false
	var chunks []string
	var body []string
	flush := func() {
		chunk := normalizeReplyText(strings.Join(body, "\n"))
		body = nil
		if chunk == "" {
			return
		}
		for _, previous := range chunks {
			if chunk == previous {
				return
			}
		}
		chunks = append(chunks, chunk)
	}
	for _, line := range lines {
		if replyFence(line) {
			fenced = !fenced
			if active {
				body = append(body, line)
			}
			continue
		}
		if !fenced {
			if section, inline := replySection(line); section != "" {
				flush()
				active = section == "reply"
				if active && inline != "" {
					body = append(body, inline)
				}
				continue
			}
			metadata := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(line, "**", ""), "__", ""))
			if replyLedgerField.MatchString(metadata) || replyTimestamp.MatchString(metadata) {
				flush()
				active = false
				continue
			}
		}
		if active {
			body = append(body, line)
		}
	}
	flush()
	return strings.Join(chunks, "\n\n")
}
