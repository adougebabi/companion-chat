package core

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// FormalAgentID is the stable product identity of one complete model task.
// Shared prompt fragments, context slots and embedding operations are not
// Agents; each entry below owns one task prompt/context/model/tool/output
// contract and is run through the common Eino Runner.
type FormalAgentID string

const (
	FormalAgentConversationCognition FormalAgentID = "conversation_cognition"
	FormalAgentWakeUp                FormalAgentID = "wake_up"
	FormalAgentTakeoverJudge         FormalAgentID = "takeover_judge"
	FormalAgentTakeoverReply         FormalAgentID = "takeover_reply"
	FormalAgentInitialization        FormalAgentID = "initialization"
	FormalAgentPersonaCompilation    FormalAgentID = "persona_compilation"
	FormalAgentMediaPrompt           FormalAgentID = "media_prompt"
	FormalAgentMediaQuality          FormalAgentID = "media_quality"
	FormalAgentVisualIdentityVision  FormalAgentID = "visual_identity_vision"
	FormalAgentVisualIdentityPatch   FormalAgentID = "visual_identity_patch"
	FormalAgentVisualIdentity        FormalAgentID = "visual_identity"
	FormalAgentConversationSummary   FormalAgentID = "conversation_summary"
	FormalAgentScheduleGeneration    FormalAgentID = "schedule_generation"
	FormalAgentNativeCognition       FormalAgentID = "native_cognition"
	FormalAgentDailyReview           FormalAgentID = "daily_review"
	FormalAgentPersistentSwitch      FormalAgentID = "persistent_switch"
	FormalAgentReflection            FormalAgentID = "reflection"
	FormalAgentScheduleReplan        FormalAgentID = "schedule_replan"
)

type FormalAgentOutputKind string

const (
	FormalAgentOutputStructured FormalAgentOutputKind = "structured"
	FormalAgentOutputText       FormalAgentOutputKind = "text"
)

// FormalAgentDefinition is deliberately declarative. Business prompt and
// context assembly stay in the task's typed Run entry; the generic Runner does
// not switch on Agent names to implement business behavior.
type FormalAgentDefinition struct {
	ID              FormalAgentID
	Name            string
	Description     string
	Role            string
	Scenario        string
	OutputKind      FormalAgentOutputKind
	MaxIterations   int
	EnableStreaming bool
	DefaultSurface  CapabilitySurface
}

var formalAgentRegistry = map[FormalAgentID]FormalAgentDefinition{
	FormalAgentConversationCognition: {ID: FormalAgentConversationCognition, Name: "fluctlight-conversation-cognition", Description: "Assess one conversation turn and use authorized capabilities before returning the final turn contract.", Role: "cognitive_assessment", Scenario: "cognitive_assessment", OutputKind: FormalAgentOutputStructured, DefaultSurface: CapabilitySurfaceConversation},
	FormalAgentWakeUp:                {ID: FormalAgentWakeUp, Name: "fluctlight-wake-up", Description: "Assess one durable wake-up fact and use authorized background capabilities before returning the final wake-up contract.", Role: "cognitive_assessment", Scenario: "wake_up", OutputKind: FormalAgentOutputStructured, DefaultSurface: CapabilitySurfaceWakeUp},
	FormalAgentTakeoverJudge:         {ID: FormalAgentTakeoverJudge, Name: "fluctlight-takeover-judge", Description: "Judge the bounded takeover rule without tools and return the verdict contract.", Role: takeoverJudgeRole, Scenario: "takeover_judge", OutputKind: FormalAgentOutputStructured, DefaultSurface: CapabilitySurfaceConversation},
	FormalAgentTakeoverReply:         {ID: FormalAgentTakeoverReply, Name: "fluctlight-takeover-reply", Description: "Produce the selected takeover persona response and use authorized conversation capabilities before returning the final turn contract.", Role: "cognitive_assessment", Scenario: "takeover_reply", OutputKind: FormalAgentOutputStructured, DefaultSurface: CapabilitySurfaceConversation},
	FormalAgentInitialization:        {ID: FormalAgentInitialization, Name: "fluctlight-initialization", Description: "Create the complete initial identity and persona projection from the Owner description.", Role: "initialization", Scenario: "initialization", OutputKind: FormalAgentOutputStructured},
	FormalAgentPersonaCompilation:    {ID: FormalAgentPersonaCompilation, Name: "fluctlight-persona-compilation", Description: "Compile one validated complete persona profile into a concise, source-linked Working Persona without tools.", Role: "initialization", Scenario: "persona_compilation", OutputKind: FormalAgentOutputStructured},
	FormalAgentMediaPrompt:           {ID: FormalAgentMediaPrompt, Name: "fluctlight-media-prompt", Description: "Convert a frozen media intent into the final renderer prompt.", Role: "media_prompt", Scenario: "media_prompt", OutputKind: FormalAgentOutputText},
	FormalAgentMediaQuality:          {ID: FormalAgentMediaQuality, Name: "fluctlight-media-quality", Description: "Inspect the generated media bytes and return the bounded quality decision.", Role: "media_prompt", Scenario: "media_quality_acceptance", OutputKind: FormalAgentOutputStructured},
	FormalAgentVisualIdentityVision:  {ID: FormalAgentVisualIdentityVision, Name: "fluctlight-visual-identity-vision", Description: "Inspect the real candidate image and return bounded identity observations.", Role: "visual_identity_vision", Scenario: "visual_identity_vision", OutputKind: FormalAgentOutputStructured},
	FormalAgentVisualIdentityPatch:   {ID: FormalAgentVisualIdentityPatch, Name: "fluctlight-visual-identity-patch", Description: "Review visual observations and return the accepted or regenerate patch contract.", Role: "visual_identity_patch", Scenario: "visual_identity_patch", OutputKind: FormalAgentOutputStructured},
	FormalAgentVisualIdentity:        {ID: FormalAgentVisualIdentity, Name: "fluctlight-visual-identity", Description: "Own the durable candidate generation, real-image review, canonical promotion, and character-sheet completion task.", Role: "visual_identity_vision", Scenario: "visual_identity_agent", OutputKind: FormalAgentOutputStructured, DefaultSurface: CapabilitySurfaceVisualIdentity},
	FormalAgentConversationSummary:   {ID: FormalAgentConversationSummary, Name: "fluctlight-conversation-summary", Description: "Summarize an explicit conversation source range into the final summary projection.", Role: "reflection", Scenario: "conversation_summary", OutputKind: FormalAgentOutputStructured},
	FormalAgentScheduleGeneration:    {ID: FormalAgentScheduleGeneration, Name: "fluctlight-schedule-generation", Description: "Generate the complete local-day schedule contract from authorized identity and life facts.", Role: "cognitive_assessment", Scenario: "schedule_generation", OutputKind: FormalAgentOutputStructured},
	FormalAgentNativeCognition:       {ID: FormalAgentNativeCognition, Name: "fluctlight-native-cognition", Description: "Assess one native life fact and use authorized native capabilities before returning the final cognition contract.", Role: "cognitive_assessment", Scenario: "native_cognition", OutputKind: FormalAgentOutputStructured, DefaultSurface: CapabilitySurfaceNativeCognition},
	FormalAgentDailyReview:           {ID: FormalAgentDailyReview, Name: "fluctlight-daily-review", Description: "Review one local day and use authorized autonomy capabilities before returning the final review contract.", Role: "cognitive_assessment", Scenario: "daily_review", OutputKind: FormalAgentOutputStructured, DefaultSurface: CapabilitySurfaceAutonomy},
	FormalAgentPersistentSwitch:      {ID: FormalAgentPersistentSwitch, Name: "fluctlight-persistent-switch", Description: "Assess the declared persistent persona switch rules without tools.", Role: "cognitive_assessment", Scenario: "persistent_switch", OutputKind: FormalAgentOutputStructured, DefaultSurface: CapabilitySurfaceConversation},
	FormalAgentReflection:            {ID: FormalAgentReflection, Name: "fluctlight-reflection", Description: "Review a bounded evidence window and return the final reflection proposal without tools.", Role: "reflection", Scenario: "reflection", OutputKind: FormalAgentOutputStructured},
	FormalAgentScheduleReplan:        {ID: FormalAgentScheduleReplan, Name: "fluctlight-schedule-replan", Description: "Create a complete replacement schedule from the frozen replan intent and current life facts.", Role: "cognitive_assessment", Scenario: "schedule_replan_planner", OutputKind: FormalAgentOutputStructured},
}

func FormalAgentDefinitions() []FormalAgentDefinition {
	result := make([]FormalAgentDefinition, 0, len(formalAgentRegistry))
	for _, definition := range formalAgentRegistry {
		result = append(result, definition)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func FormalAgentDefinitionByID(id FormalAgentID) (FormalAgentDefinition, bool) {
	definition, ok := formalAgentRegistry[id]
	return definition, ok
}

func formalAgentForSchema(schemaName string) (FormalAgentID, bool) {
	switch strings.TrimSpace(schemaName) {
	case "conversation_turn_response":
		return FormalAgentConversationCognition, true
	case "wake_up_response":
		return FormalAgentWakeUp, true
	case takeoverJudgeSchemaName:
		return FormalAgentTakeoverJudge, true
	case takeoverReplySchemaName:
		return FormalAgentTakeoverReply, true
	case "initialization_response":
		return FormalAgentInitialization, true
	case "persona_compilation_response":
		return FormalAgentPersonaCompilation, true
	case "media_prompt_text":
		return FormalAgentMediaPrompt, true
	case "media_quality_acceptance_response":
		return FormalAgentMediaQuality, true
	case "visual_identity_vision_response":
		return FormalAgentVisualIdentityVision, true
	case "visual_identity_patch_response":
		return FormalAgentVisualIdentityPatch, true
	case "visual_identity_agent_response":
		return FormalAgentVisualIdentity, true
	case "conversation_summary_v1":
		return FormalAgentConversationSummary, true
	case "schedule_response":
		return FormalAgentScheduleGeneration, true
	case "native_cognition_response":
		return FormalAgentNativeCognition, true
	case "daily_review_response":
		return FormalAgentDailyReview, true
	case persistentSwitchAssessmentSchemaName:
		return FormalAgentPersistentSwitch, true
	case "reflection_proposal_v2":
		return FormalAgentReflection, true
	case "schedule_replan_plan":
		return FormalAgentScheduleReplan, true
	default:
		return "", false
	}
}

type formalAgentContextKey struct{}

func withFormalAgentDefinition(ctx context.Context, definition FormalAgentDefinition) context.Context {
	return context.WithValue(ctx, formalAgentContextKey{}, definition)
}

func formalAgentDefinitionFromContext(ctx context.Context) (FormalAgentDefinition, bool) {
	definition, ok := ctx.Value(formalAgentContextKey{}).(FormalAgentDefinition)
	return definition, ok
}

// FormalAgentRunInput is the already-assembled typed task input accepted by
// the common Runner. Typed Run* methods remain responsible for assembling it.
type FormalAgentRunInput struct {
	Prompt          PromptAssemblyResult
	Definitions     []CapabilityDefinition
	SchemaName      string
	EnableThinking  bool
	EnableStreaming bool
	Capability      *ADKCapabilityRequest
}

func (a *App) RunFormalAgent(ctx context.Context, id FormalAgentID, input FormalAgentRunInput) (ADKStructuredTaskResult, error) {
	definition, ok := FormalAgentDefinitionByID(id)
	if !ok {
		return ADKStructuredTaskResult{}, fmt.Errorf("formal_agent_not_registered: %s", id)
	}
	if strings.TrimSpace(input.SchemaName) == "" {
		return ADKStructuredTaskResult{}, errors.New("formal_agent_output_contract_required")
	}
	definition.EnableStreaming = input.EnableStreaming
	return a.RunADKStructuredTask(withFormalAgentDefinition(ctx, definition), ADKStructuredTaskInput{
		AgentID: id, Role: definition.Role, Scenario: definition.Scenario,
		Prompt: input.Prompt, Definitions: input.Definitions, SchemaName: input.SchemaName,
		EnableThinking: input.EnableThinking, EnableStreaming: input.EnableStreaming,
		TextOutput: definition.OutputKind == FormalAgentOutputText, Capability: input.Capability,
	})
}

// RunFormalAgentStream is the production streaming entry. It uses the same
// registry, Eino Agent and Runner; only RunnerConfig.EnableStreaming changes,
// so tool execution and final decoding cannot drift into a second path.
func (a *App) RunFormalAgentStream(ctx context.Context, id FormalAgentID, input FormalAgentRunInput) (ADKStructuredTaskResult, error) {
	input.EnableStreaming = true
	return a.RunFormalAgent(ctx, id, input)
}
