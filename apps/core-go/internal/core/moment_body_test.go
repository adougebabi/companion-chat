package core

import (
	"errors"
	"testing"
)

func TestMomentPublicationTextExtractsBodyWithoutInventingMedia(t *testing.T) {
	for _, sample := range []struct {
		name, raw, want string
		hasMedia        bool
	}{
		{name: "plain-fallback", raw: "今天读到一个值得记下的段落。", want: "今天读到一个值得记下的段落。"},
		{name: "moment-heading", raw: "# 发布动态\n今天读到一个值得记下的段落。\n# 认知\nprivate\n# 事务记录\nprivate", want: "今天读到一个值得记下的段落。"},
		{name: "fenced-protocol", raw: "```markdown\n# 认知\nprivate\n# 发布动态\n留作稍后的讨论。\n# 事务记录\nprivate\n```", want: "留作稍后的讨论。"},
		{name: "duplicate-sections", raw: "# 发布动态\n你好。\n# 发布动态\n你好。\n# 发布动态\n今天也要好好生活。", want: "你好。\n\n今天也要好好生活。"},
		{name: "malformed-envelope", raw: `{"text":"# 发布动态\n你好。\n# 认知\nprivate"`, want: "你好。"},
		{name: "no-media", raw: "# 发布动态\n今天也要好好生活。\n\n[自拍照：站在窗边微笑]\n# 事务记录\nprivate", want: "今天也要好好生活。"},
		{name: "inline-placeholder", raw: "早安。[自拍照:窗边]今天也要开心。", want: "早安。今天也要开心。"},
		{name: "multiple-placeholders", raw: "[自拍照：窗边]\n\n你好。\n\n【自拍照：公园】", want: "你好。"},
		{name: "attached-media", raw: "# 发布动态\n今天也要好好生活。\n\n[自拍照：站在窗边微笑]", want: "今天也要好好生活。\n\n[自拍照：站在窗边微笑]", hasMedia: true},
		{name: "ordinary-markdown", raw: "# 茶的选择\n\n[乌龙茶](https://example.invalid/tea)很香。", want: "# 茶的选择\n\n[乌龙茶](https://example.invalid/tea)很香。"},
		{name: "private-only", raw: "# 认知\nprivate\n# 事务记录\nprivate"},
		{name: "empty-heading", raw: "# 发布动态\n# 事务记录\nprivate"},
		{name: "placeholder-only", raw: "# 发布动态\n[自拍照：窗边]"},
	} {
		t.Run(sample.name, func(t *testing.T) {
			body, err := momentPublicationText(sample.raw, sample.hasMedia)
			if sample.want == "" {
				if !errors.Is(err, ErrInvalidArguments) || body != "" {
					t.Fatalf("bodyless output became a Moment: %q, %v", body, err)
				}
				return
			}
			if err != nil || body != sample.want {
				t.Fatalf("Moment body = %q, want %q; error=%v", body, sample.want, err)
			}
			if again, err := momentPublicationText(body, sample.hasMedia); err != nil || again != body {
				t.Fatalf("Moment filter is not idempotent: %q, %v", again, err)
			}
		})
	}
}
