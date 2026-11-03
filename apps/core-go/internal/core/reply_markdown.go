package core

import (
	"strings"
	"unicode"
)

// A fence closes only with the same marker, at least its opening length and
// whitespace after it. A shorter or different fence inside code is literal.
type replyMarkdownFence struct {
	marker byte
	width  int
}

func (f *replyMarkdownFence) consume(line string) bool {
	content := strings.TrimLeft(line, " ")
	if len(line)-len(content) > 3 || len(content) == 0 || (content[0] != '`' && content[0] != '~') {
		return f.width > 0
	}
	marker := content[0]
	width := len(content) - len(strings.TrimLeft(content, string(marker)))
	if f.width > 0 {
		if marker == f.marker && width >= f.width && strings.TrimSpace(content[width:]) == "" {
			f.width = 0
		}
		return true
	}
	if width >= 3 && (marker != '`' || !strings.Contains(content[width:], "`")) {
		f.marker, f.width = marker, width
		return true
	}
	return false
}

func replyIndentedCodeLine(line string) bool {
	indent := len(line) - len(strings.TrimLeft(line, " "))
	return indent >= 4 || (indent < len(line) && line[indent] == '\t')
}

// Trim surrounding prose whitespace without turning leading indented code
// into a heading or removing whitespace from the edge of a fenced example.
func trimReplyText(text string) string {
	lines := strings.Split(text, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	protected := make([]bool, len(lines))
	var fence replyMarkdownFence
	for index, line := range lines {
		protected[index] = fence.consume(line) || replyIndentedCodeLine(line)
	}
	for len(lines) > 0 && !protected[len(lines)-1] && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return ""
	}
	if !protected[0] {
		lines[0] = strings.TrimLeftFunc(lines[0], unicode.IsSpace)
	}
	if !protected[len(lines)-1] {
		lines[len(lines)-1] = strings.TrimRightFunc(lines[len(lines)-1], unicode.IsSpace)
	}
	return strings.Join(lines, "\n")
}

// stripReplyMarkdownHeadings removes only ATX heading delimiters. Preserve
// indentation, line endings, non-heading hashes and all fenced/indented code.
func stripReplyMarkdownHeadings(text string) string {
	lines := strings.Split(text, "\n")
	var fence replyMarkdownFence
	for index, line := range lines {
		if fence.consume(line) || replyIndentedCodeLine(line) {
			continue
		}
		content := strings.TrimLeft(line, " ")
		indent := len(line) - len(content)
		width := len(content) - len(strings.TrimLeft(content, "#"))
		if width < 1 || width > 6 {
			continue
		}
		ending := ""
		if strings.HasSuffix(content, "\r") {
			content, ending = strings.TrimSuffix(content, "\r"), "\r"
		}
		if width < len(content) && content[width] != ' ' && content[width] != '\t' {
			continue
		}
		body := strings.TrimLeft(content[width:], " \t")
		// Optional closing hashes are delimiters only when separated by space.
		trimmed := strings.TrimRight(body, " \t")
		closing := len(strings.TrimRight(trimmed, "#"))
		if closing < len(trimmed) && (closing == 0 || trimmed[closing-1] == ' ' || trimmed[closing-1] == '\t') {
			body = strings.TrimRight(trimmed[:closing], " \t")
		}
		lines[index] = line[:indent] + body + ending
	}
	return strings.Join(lines, "\n")
}
