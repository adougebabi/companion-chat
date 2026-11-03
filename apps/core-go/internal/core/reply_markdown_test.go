package core

import "testing"

func TestStripReplyMarkdownHeadings(t *testing.T) {
	for _, sample := range []struct{ name, input, want string }{
		{"evidence-heading", "# 协议测试\n\n原正文。", "协议测试\n\n原正文。"},
		{"all-levels", "# 一\n## 二\n### 三\n#### 四\n##### 五\n###### 六", "一\n二\n三\n四\n五\n六"},
		{"spacing", "  ## \t标题  \n段落  内空格\t保留。", "  标题  \n段落  内空格\t保留。"},
		{"closing-markers", "### 标题 ###  \n# C#\n## 标题 #tag", "标题\nC#\n标题 #tag"},
		{"empty-heading", "#\n## \n### ###\n正文", "\n\n\n正文"},
		{"crlf", "# 标题\r\n正文\r\n######\r\n", "标题\r\n正文\r\n\r\n"},
		{"ordinary-hashes", "#话题\n####### 七个不是标题\n正文 # 标记\nC#\n\\# 转义\n`# 内联代码`", "#话题\n####### 七个不是标题\n正文 # 标记\nC#\n\\# 转义\n`# 内联代码`"},
		{"other-markdown", "- 列表\n> # 引用\n**加粗**\n标题\n===\n[链接](https://example.com/#anchor)", "- 列表\n> # 引用\n**加粗**\n标题\n===\n[链接](https://example.com/#anchor)"},
		{"backtick-code", "# 标题\n```md\n# 保留\n###### 也保留\n```\n## 后续", "标题\n```md\n# 保留\n###### 也保留\n```\n后续"},
		{"tilde-code", "~~~md\n## 保留\n~~~\n# 后续", "~~~md\n## 保留\n~~~\n后续"},
		{"indented-code", "示例：\n    # 注释\n\t## 注释\n   ### 标题", "示例：\n    # 注释\n\t## 注释\n   标题"},
		{"fence-length-and-type", "````md\n```\n# 保留\n~~~\n## 保留\n```` trailing\n### 保留\n`````\n# 后续", "````md\n```\n# 保留\n~~~\n## 保留\n```` trailing\n### 保留\n`````\n后续"},
		{"indented-fence", "   ~~~md\n# 保留\n   ~~~~\n# 后续", "   ~~~md\n# 保留\n   ~~~~\n后续"},
		{"unclosed-fence", "正文\n```md\n# 保留\n## 保留", "正文\n```md\n# 保留\n## 保留"},
		{"empty", "", ""},
	} {
		t.Run(sample.name, func(t *testing.T) {
			if got := stripReplyMarkdownHeadings(sample.input); got != sample.want {
				t.Fatalf("cleaned = %q, want %q", got, sample.want)
			}
			if got := stripReplyMarkdownHeadings(sample.want); got != sample.want {
				t.Fatalf("cleaner changed already cleaned text: %q", got)
			}
		})
	}
}

func TestReplyDraftAndPublicationBodyStripMarkdownHeadings(t *testing.T) {
	for _, sample := range []struct{ name, input, want string }{
		{"plain", "# 协议测试\n\n正文不改写。", "协议测试\n\n正文不改写。"},
		{"protocol", "# 回复\n## 今天的事\n正文。\n# 认知\nprivate", "今天的事\n正文。"},
		{"envelope", `{"text":"# 今天的事\n正文。"}`, "今天的事\n正文。"},
		{"literal-newlines", `# 今天的事\n\n正文。`, "今天的事\n\n正文。"},
		{"code", "# 今天的事\n示例：\n```python\n# 注释不变\nprint('# tag')\n```\n## 后续", "今天的事\n示例：\n```python\n# 注释不变\nprint('# tag')\n```\n后续"},
		{"standalone-code", "```markdown\n# Reply\n## 标题作为代码保留\n```", "```markdown\n# Reply\n## 标题作为代码保留\n```"},
		{"standalone-indented-code", "    # Reply\n    ## 注释\n\t# 注释", "    # Reply\n    ## 注释\n\t# 注释"},
		{"standalone-indented-envelope", `{"text":"    # 注释"}`, "    # 注释"},
		{"fenced-edge-whitespace", "   ```python\n# 注释  \n```  ", "   ```python\n# 注释  \n```  "},
		{"nested-fences", "# 今天的事\n示例：\n````md\n```\n# Reply\n# 保留\n~~~\n## 保留\n````\n## 后续", "今天的事\n示例：\n````md\n```\n# Reply\n# 保留\n~~~\n## 保留\n````\n后续"},
		{"indented-code", "# 今天的事\n示例：\n\n    # Reply\n    ## 注释\n\t# 注释\n\n## 后续", "今天的事\n示例：\n\n    # Reply\n    ## 注释\n\t# 注释\n\n后续"},
	} {
		t.Run(sample.name, func(t *testing.T) {
			draft, err := parseReplyDraft(sample.input)
			if err != nil || draft.Text != sample.want {
				t.Fatalf("draft = %q, want %q; error = %v", draft.Text, sample.want, err)
			}
			if got := extractReplyBody(sample.want); got != sample.want {
				t.Fatalf("publication changed clean body: %q", got)
			}
		})
	}
}

func TestReplyMarkdownPolicyLeavesMomentHeadingsUnchanged(t *testing.T) {
	const text = "# 茶的选择\n\n正文。"
	draft, err := parseReplyDraftForScene(text, replySceneMoment)
	if err != nil || draft.Text != text {
		t.Fatalf("Moment heading changed: %q, %v", draft.Text, err)
	}
}

func TestReplyDraftRejectsHeadingMarkersWithoutBody(t *testing.T) {
	for _, raw := range []string{"#", "## ", "#\n\n######", "### ###"} {
		draft, err := parseReplyDraft(raw)
		if err == nil || draft.Text != "" {
			t.Fatalf("heading-only text became a reply: %q, %v", draft.Text, err)
		}
	}
}
