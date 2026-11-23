package core

import "testing"

func TestChatMediaIntentFalsePositiveRegression(t *testing.T) {
	for _, text := range []string{"想分享给你看。", "我想发张照片给你。", "是张配色图，想分享给你看。"} {
		if !directExtractedClaimExplicitlyNonDelivery(text) {
			t.Errorf("non-delivery intent rejected: %s", text)
		}
	}
}
func TestChatTextNarrationIsNotImageDelivery(t *testing.T) {
	if !directClearlyNoDeliveryClaim("（发完后，看着对话框，又补了一句。）") {
		t.Fatal("text-message narration rejected as image delivery")
	}
}

func TestChatMediaNarrationCannotHideARealDeliveryClaim(t *testing.T) {
	for _, text := range []string{"（已经发给你了，看着对话框。）", "（发完照片后，看着对话框。）", "我想分享给你看，照片已经发给你了。", "我想我已经把照片发给你了。", "这是我投递给你的照片。", "这是递交给你的照片。"} {
		if directClearlyNoDeliveryClaim(text) {
			t.Errorf("unsupported delivery escaped: %s", text)
		}
	}
}
