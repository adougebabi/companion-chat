package core

import (
	"maps"
	"strings"
)

const imageNoTextInstruction = "图片中禁止任何文字、伪文字、字母、数字、标题、标签、字幕、签名、徽标和水印；所有资料区保持空白或仅用图形和色块表达。 No text, pseudo-text, letters, numbers, captions, labels, signatures, logos or watermarks."
const imageNoTextNegativePrompt = "text, pseudo-text, gibberish, letters, numbers, typography, titles, labels, captions, subtitles, signatures, logos, watermarks, 文字, 伪文字, 乱码, 字母, 数字, 标题, 标签, 字幕, 签名, 徽标, 水印"

func imagePromptWithoutText(value string) string {
	return appendImagePromptRule(value, imageNoTextInstruction, "\n\n")
}

func appendImagePromptRule(value, rule, separator string) string {
	if strings.Contains(value, rule) {
		return value
	}
	if value == "" {
		return rule
	}
	return value + separator + rule
}

// imageWorkflowWithoutText applies the explicit product rendering policy at the
// common physical submission boundary. Model-owned scene facts, reference
// images, numeric renderer controls and graph edges remain intact. Negative
// conditioning is found through the declared ComfyUI negative input edges,
// rather than guessed from node names or prompt prose.
func imageWorkflowWithoutText(workflow map[string]any) map[string]any {
	result := maps.Clone(workflow)
	for id, raw := range result {
		if node, ok := raw.(map[string]any); ok {
			node = maps.Clone(node)
			if inputs, ok := node["inputs"].(map[string]any); ok {
				node["inputs"] = maps.Clone(inputs)
			}
			result[id] = node
		}
	}
	positiveNodes, negativeNodes := map[string]bool{}, map[string]bool{}
	var markConditioning func(any, map[string]bool)
	markConditioning = func(value any, nodes map[string]bool) {
		edge := arrayValue(value)
		if len(edge) != 2 {
			return
		}
		id, ok := edge[0].(string)
		if !ok || nodes[id] || result[id] == nil {
			return
		}
		nodes[id] = true
		for _, input := range mapValue(mapValue(result[id])["inputs"]) {
			markConditioning(input, nodes)
		}
	}
	for _, raw := range result {
		inputs := mapValue(mapValue(raw)["inputs"])
		markConditioning(inputs["positive"], positiveNodes)
		markConditioning(inputs["positive_prompt"], positiveNodes)
		markConditioning(inputs["negative"], negativeNodes)
		markConditioning(inputs["negative_prompt"], negativeNodes)
	}
	for id, raw := range result {
		inputs := mapValue(mapValue(raw)["inputs"])
		for _, key := range []string{"text", "text_g", "text_l", "prompt", "positive_prompt", "negative_prompt", "negative"} {
			text, ok := inputs[key].(string)
			if !ok {
				continue
			}
			// Flux and similar workflows derive zeroed negative conditioning
			// from the positive encoder. Its shared text still renders the
			// positive scene, so never append exclusion tokens there.
			if negativeNodes[id] && !positiveNodes[id] || key == "negative_prompt" || key == "negative" {
				inputs[key] = appendImagePromptRule(text, imageNoTextNegativePrompt, ", ")
			} else {
				inputs[key] = imagePromptWithoutText(text)
			}
		}
	}
	return result
}
