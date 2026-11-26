package core

import (
	"testing"
)

func TestDirectMediaContextPrefilterIgnoresOlderUnrelatedTopic(t *testing.T) {
	history := []map[string]any{
		{"kind": "user", "text": "刚才那张照片怎么样？"},
		{"kind": "assistant", "text": "挺好看的。"},
		{"kind": "user", "text": "今天工作怎么样？"},
		{"kind": "assistant", "text": "今天已经下班了。"},
		{"kind": "user", "text": "弄好了没？"},
	}
	if directMediaDeliveryCheckNeeded("已经贴到聊天里了。", "弄好了没？", history) {
		t.Fatal("an older unrelated media topic triggered a new media check")
	}
}
