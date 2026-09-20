package core

import "github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/prompt"

// Provider instructions are role-specific. The JSON Schema and native tools
// carry protocol detail; the system message states semantic authority and the
// decision boundary the model must follow.
const (
	providerLanguageRule              = prompt.ProviderLanguageRule
	mediaPromptInstruction            = prompt.MediaPromptInstruction
	mediaQualityAcceptanceInstruction = prompt.MediaQualityAcceptanceInstruction
	providerContextAuthorityRule      = prompt.ProviderContextAuthorityRule
	reflectionV2Instruction           = prompt.ReflectionV2Instruction
	nativeCognitionInstruction        = prompt.NativeCognitionInstruction
	actionRealizationInstruction      = prompt.ActionRealizationInstruction
)
