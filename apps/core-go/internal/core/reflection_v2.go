package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

const (
	reflectionProposalV2SchemaVersion = "fluctlight.reflection-proposal.v2"
	evolutionContextSchemaVersion     = "fluctlight.evolution-context.v2"
	reflectionPolicyVersion           = "reflection.policy.v2"
)

type EvolutionDomain string

const (
	EvolutionMemory         EvolutionDomain = "memory"
	EvolutionRelationship   EvolutionDomain = "relationship"
	EvolutionGoal           EvolutionDomain = "goal"
	EvolutionIntention      EvolutionDomain = "intention"
	EvolutionAffectProfile  EvolutionDomain = "affect_profile"
	EvolutionDrive          EvolutionDomain = "drive"
	EvolutionPreference     EvolutionDomain = "preference"
	EvolutionTrigger        EvolutionDomain = "trigger"
	EvolutionDevelopingSelf EvolutionDomain = "developing_self"
	EvolutionPersonality    EvolutionDomain = "personality_overlay"
	EvolutionBehaviorPolicy EvolutionDomain = "behavior_policy_overlay"
)

type ReflectionMemoryCandidateV2 struct {
	Operation             string   `json:"operation"`
	TargetRef             string   `json:"target_ref,omitempty"`
	MergeRefs             []string `json:"merge_refs,omitempty"`
	Type                  string   `json:"type,omitempty"`
	Content               string   `json:"content,omitempty"`
	Confidence            float64  `json:"confidence"`
	Importance            float64  `json:"importance,omitempty"`
	EmotionalSignificance float64  `json:"emotional_significance,omitempty"`
	EvidenceRefs          []string `json:"evidence_refs"`
	SemanticReason        string   `json:"semantic_reason"`
}

type ReflectionRelationshipObservationV2 struct {
	TargetRef      string   `json:"target_ref"`
	Observation    string   `json:"observation"`
	Direction      string   `json:"direction"`
	Strength       float64  `json:"strength"`
	Confidence     float64  `json:"confidence"`
	EvidenceRefs   []string `json:"evidence_refs"`
	SemanticReason string   `json:"semantic_reason"`
}

type ReflectionGoalCandidateV2 struct {
	Operation        string   `json:"operation"`
	TargetRef        string   `json:"target_ref,omitempty"`
	DesiredOutcome   string   `json:"desired_outcome,omitempty"`
	SuccessCriteria  []string `json:"success_criteria,omitempty"`
	Motivation       string   `json:"motivation,omitempty"`
	Direction        string   `json:"direction,omitempty"`
	Strength         float64  `json:"strength"`
	Confidence       float64  `json:"confidence"`
	OutcomeRefs      []string `json:"outcome_refs,omitempty"`
	CriterionIndexes []int    `json:"criterion_indexes,omitempty"`
	Complete         bool     `json:"complete,omitempty"`
	EvidenceRefs     []string `json:"evidence_refs"`
	SemanticReason   string   `json:"semantic_reason"`
}

type ReflectionIntentionCandidateV2 struct {
	Operation             string                 `json:"operation"`
	TargetRef             string                 `json:"target_ref,omitempty"`
	GoalRef               string                 `json:"goal_ref"`
	ActionIntent          string                 `json:"action_intent,omitempty"`
	ExpectedOutcome       string                 `json:"expected_outcome,omitempty"`
	CapabilityConstraints []string               `json:"capability_constraints,omitempty"`
	TypedTrigger          *TypedIntentionTrigger `json:"typed_trigger,omitempty"`
	PreferredTime         *time.Time             `json:"preferred_time,omitempty"`
	Expiration            *time.Time             `json:"expiration,omitempty"`
	Confidence            float64                `json:"confidence"`
	EvidenceRefs          []string               `json:"evidence_refs"`
	SemanticReason        string                 `json:"semantic_reason"`
}

type ReflectionEmotionalSummaryV2 struct {
	DominantPatterns []string `json:"dominant_patterns"`
	Triggers         []string `json:"triggers"`
	RecoveryPatterns []string `json:"recovery_patterns"`
	Conflicts        []string `json:"conflicts"`
	EvidenceRefs     []string `json:"evidence_refs"`
}

type ReflectionAffectCandidateV2 struct {
	Target         string   `json:"target"`
	Direction      string   `json:"direction"`
	Strength       float64  `json:"strength"`
	Confidence     float64  `json:"confidence"`
	EvidenceRefs   []string `json:"evidence_refs"`
	SemanticReason string   `json:"semantic_reason"`
}

type ReflectionSlotCandidateV2 struct {
	Operation      string   `json:"operation"`
	TargetRef      string   `json:"target_ref,omitempty"`
	Key            string   `json:"key"`
	SemanticValue  string   `json:"semantic_value"`
	Direction      string   `json:"direction"`
	Strength       float64  `json:"strength"`
	Confidence     float64  `json:"confidence"`
	EvidenceRefs   []string `json:"evidence_refs"`
	SemanticReason string   `json:"semantic_reason"`
}

type ReflectionDevelopingSelfCandidateV2 struct {
	Operation      string   `json:"operation"`
	TargetRef      string   `json:"target_ref,omitempty"`
	Category       string   `json:"category"`
	Claim          string   `json:"claim"`
	Confidence     float64  `json:"confidence"`
	EvidenceRefs   []string `json:"evidence_refs"`
	SemanticReason string   `json:"semantic_reason"`
}

type ReflectionOverlayCandidateV2 struct {
	FieldPath      string   `json:"field_path"`
	Direction      string   `json:"direction"`
	SemanticValue  string   `json:"semantic_value,omitempty"`
	Strength       float64  `json:"strength"`
	Confidence     float64  `json:"confidence"`
	EvidenceRefs   []string `json:"evidence_refs"`
	SemanticReason string   `json:"semantic_reason"`
}

type ReflectionProposalV2 struct {
	SchemaVersion                     string                                `json:"schema_version"`
	Summary                           string                                `json:"summary"`
	MemoryCandidates                  []ReflectionMemoryCandidateV2         `json:"memory_candidates"`
	RelationshipObservations          []ReflectionRelationshipObservationV2 `json:"relationship_observations"`
	GoalCandidates                    []ReflectionGoalCandidateV2           `json:"goal_candidates"`
	IntentionCandidates               []ReflectionIntentionCandidateV2      `json:"intention_candidates"`
	EmotionalSummary                  ReflectionEmotionalSummaryV2          `json:"emotional_summary"`
	AffectRecalibrationCandidates     []ReflectionAffectCandidateV2         `json:"affect_recalibration_candidates"`
	DriveCandidates                   []ReflectionSlotCandidateV2           `json:"drive_candidates"`
	PreferenceCandidates              []ReflectionSlotCandidateV2           `json:"preference_candidates"`
	TriggerCandidates                 []ReflectionSlotCandidateV2           `json:"trigger_candidates"`
	DevelopingSelfCandidates          []ReflectionDevelopingSelfCandidateV2 `json:"developing_self_candidates"`
	PersonalityEvolutionCandidates    []ReflectionOverlayCandidateV2        `json:"personality_evolution_candidates"`
	BehaviorPolicyEvolutionCandidates []ReflectionOverlayCandidateV2        `json:"behavior_policy_evolution_candidates"`
}

func DecodeReflectionProposalV2(raw []byte) (ReflectionProposalV2, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var proposal ReflectionProposalV2
	if err := decoder.Decode(&proposal); err != nil {
		return ReflectionProposalV2{}, fmt.Errorf("reflection_v2_schema_invalid: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ReflectionProposalV2{}, errors.New("reflection_v2_trailing_payload")
	}
	if proposal.SchemaVersion != reflectionProposalV2SchemaVersion || strings.TrimSpace(proposal.Summary) == "" || len([]rune(proposal.Summary)) > 4000 {
		return ReflectionProposalV2{}, errors.New("reflection_v2_header_invalid")
	}
	return proposal, nil
}

type EvolutionEvidence struct {
	Ref        string    `json:"ref"`
	Kind       string    `json:"kind"`
	Summary    string    `json:"summary"`
	Sequence   int       `json:"sequence"`
	OccurredAt time.Time `json:"occurred_at"`
}

type EvolutionContext struct {
	SchemaVersion  string                `json:"schema_version"`
	ProviderRole   string                `json:"provider_role"`
	FluctlightID   string                `json:"fluctlight_id"`
	SourceWindow   string                `json:"source_window"`
	FromSequence   int                   `json:"from_sequence"`
	ToSequence     int                   `json:"to_sequence"`
	Watermark      int                   `json:"watermark"`
	Evidence       []EvolutionEvidence   `json:"evidence"`
	Outcomes       []ActionOutcome       `json:"outcomes"`
	ReferenceIndex ContextReferenceIndex `json:"reference_index"`
	BaseRevisions  map[string]int        `json:"base_revisions"`
}

func BuildEvolutionContext(input EvolutionContext) (EvolutionContext, error) {
	if input.ProviderRole != "reflection" || strings.TrimSpace(input.FluctlightID) == "" || strings.TrimSpace(input.SourceWindow) == "" || input.FromSequence < 1 || input.ToSequence < input.FromSequence || input.Watermark != input.FromSequence-1 {
		return EvolutionContext{}, errors.New("evolution_context_identity_invalid")
	}
	if len(input.Evidence) == 0 || len(input.Evidence) > 256 || len(input.Outcomes) > 128 || len(input.BaseRevisions) == 0 {
		return EvolutionContext{}, errors.New("evolution_context_bounds_invalid")
	}
	if err := input.ReferenceIndex.Validate(); err != nil || input.ReferenceIndex.FluctlightID != input.FluctlightID {
		return EvolutionContext{}, errors.New("evolution_context_reference_index_invalid")
	}
	seen := map[string]struct{}{}
	for _, evidence := range input.Evidence {
		if strings.TrimSpace(evidence.Ref) == "" || strings.TrimSpace(evidence.Kind) == "" || len([]rune(evidence.Kind)) > 128 || strings.TrimSpace(evidence.Summary) == "" || len([]rune(evidence.Summary)) > 2000 || evidence.Sequence < input.FromSequence || evidence.Sequence > input.ToSequence || evidence.OccurredAt.IsZero() {
			return EvolutionContext{}, errors.New("evolution_context_evidence_invalid")
		}
		if _, duplicate := seen[evidence.Ref]; duplicate {
			return EvolutionContext{}, errors.New("evolution_context_evidence_duplicate")
		}
		seen[evidence.Ref] = struct{}{}
	}
	for _, outcome := range input.Outcomes {
		if err := outcome.Validate(); err != nil || outcome.FluctlightID != input.FluctlightID {
			return EvolutionContext{}, errors.New("evolution_context_outcome_invalid")
		}
	}
	result := input
	result.SchemaVersion = evolutionContextSchemaVersion
	result.Evidence = append([]EvolutionEvidence(nil), input.Evidence...)
	result.Outcomes = append([]ActionOutcome(nil), input.Outcomes...)
	result.BaseRevisions = cloneRevisionMap(input.BaseRevisions)
	return result, nil
}

type EvolutionDisposition string

const (
	EvolutionAccepted EvolutionDisposition = "accepted"
	EvolutionRejected EvolutionDisposition = "rejected"
	EvolutionDeferred EvolutionDisposition = "deferred"
	EvolutionNoChange EvolutionDisposition = "no_change"
)

type EvolutionCandidatePlan struct {
	CandidateID  string               `json:"candidate_id"`
	Domain       EvolutionDomain      `json:"domain"`
	Index        int                  `json:"index"`
	Operation    string               `json:"operation"`
	TargetRef    string               `json:"target_ref,omitempty"`
	FieldPath    string               `json:"field_path,omitempty"`
	Direction    string               `json:"direction,omitempty"`
	Strength     float64              `json:"strength"`
	Confidence   float64              `json:"confidence"`
	EvidenceRefs []string             `json:"evidence_refs"`
	Disposition  EvolutionDisposition `json:"disposition"`
	ReasonCode   string               `json:"reason_code"`
}

type ReflectionPolicyV2 struct {
	MinConfidence     float64
	SlowMinConfidence float64
	SlowMinEvidence   int
	Cooldowns         map[string]time.Time
	SupportedDomains  map[EvolutionDomain]bool
}

type ReflectionEvolutionPlan struct {
	ProposalID        string                   `json:"proposal_id"`
	FluctlightID      string                   `json:"fluctlight_id"`
	SourceWindow      string                   `json:"source_window"`
	BaseWatermark     int                      `json:"base_watermark"`
	ToSequence        int                      `json:"to_sequence"`
	ExpectedRevisions map[string]int           `json:"expected_revisions"`
	Candidates        []EvolutionCandidatePlan `json:"candidates"`
	PolicyVersion     string                   `json:"policy_version"`
}

type reflectionCandidateInput struct {
	domain       EvolutionDomain
	index        int
	operation    string
	targetRef    string
	fieldPath    string
	direction    string
	strength     float64
	confidence   float64
	evidenceRefs []string
	requiredKind ContextReferenceKind
}

func CompileReflectionPlan(proposal ReflectionProposalV2, context EvolutionContext, policy ReflectionPolicyV2, now time.Time) (ReflectionEvolutionPlan, error) {
	if proposal.SchemaVersion != reflectionProposalV2SchemaVersion || context.SchemaVersion != evolutionContextSchemaVersion || context.ProviderRole != "reflection" || now.IsZero() {
		return ReflectionEvolutionPlan{}, errors.New("reflection_plan_input_invalid")
	}
	if err := validateReflectionProposalReferences(proposal, context); err != nil {
		return ReflectionEvolutionPlan{}, err
	}
	allowedEvidence := map[string]struct{}{}
	for _, evidence := range context.Evidence {
		allowedEvidence[evidence.Ref] = struct{}{}
	}
	for _, outcome := range context.Outcomes {
		allowedEvidence[outcome.ID] = struct{}{}
		for _, ref := range outcome.EvidenceRefs {
			allowedEvidence[ref] = struct{}{}
		}
	}
	for ref := range context.ReferenceIndex.ByRef {
		allowedEvidence[ref] = struct{}{}
	}
	inputs := collectReflectionCandidates(proposal)
	if len(inputs) > 256 {
		return ReflectionEvolutionPlan{}, errors.New("reflection_candidates_too_many")
	}
	if policy.MinConfidence <= 0 {
		policy.MinConfidence = 0.65
	}
	if policy.SlowMinConfidence <= 0 {
		policy.SlowMinConfidence = 0.8
	}
	if policy.SlowMinEvidence <= 0 {
		policy.SlowMinEvidence = 2
	}
	plans := make([]EvolutionCandidatePlan, 0, len(inputs))
	for _, input := range inputs {
		if strings.TrimSpace(input.operation) == "" || !unitFinite(input.confidence) || !unitFinite(input.strength) || len(input.evidenceRefs) == 0 || len(input.evidenceRefs) > 64 {
			return ReflectionEvolutionPlan{}, fmt.Errorf("reflection_%s_candidate_%d_invalid", input.domain, input.index)
		}
		for _, ref := range input.evidenceRefs {
			if _, ok := allowedEvidence[ref]; !ok {
				return ReflectionEvolutionPlan{}, fmt.Errorf("reflection_%s_candidate_%d_foreign_evidence", input.domain, input.index)
			}
		}
		if input.targetRef != "" {
			entry, ok := context.ReferenceIndex.ByRef[input.targetRef]
			if !ok || (input.requiredKind != "" && entry.Kind != input.requiredKind) {
				return ReflectionEvolutionPlan{}, fmt.Errorf("reflection_%s_candidate_%d_foreign_target", input.domain, input.index)
			}
		}
		if _, exists := context.BaseRevisions[string(input.domain)]; !exists {
			return ReflectionEvolutionPlan{}, fmt.Errorf("reflection_%s_base_revision_missing", input.domain)
		}
		disposition, reason := EvolutionAccepted, "policy_accepted"
		threshold := policy.MinConfidence
		slow := input.domain == EvolutionPersonality || input.domain == EvolutionBehaviorPolicy || input.domain == EvolutionAffectProfile || input.domain == EvolutionDevelopingSelf
		if slow {
			threshold = policy.SlowMinConfidence
		}
		if policy.SupportedDomains != nil && !policy.SupportedDomains[input.domain] {
			disposition, reason = EvolutionDeferred, "domain_reducer_unavailable"
		} else if input.confidence < threshold {
			disposition, reason = EvolutionDeferred, "confidence_below_threshold"
		} else if slow && len(mergeStableRefs(input.evidenceRefs)) < policy.SlowMinEvidence {
			disposition, reason = EvolutionDeferred, "cross_window_evidence_required"
		} else if until, ok := policy.Cooldowns[string(input.domain)+":"+input.fieldPath]; ok && now.Before(until) {
			disposition, reason = EvolutionDeferred, "cooldown_active"
		} else if input.direction == "maintain" || input.operation == "no_change" {
			disposition, reason = EvolutionNoChange, "semantic_no_change"
		}
		candidateID := "evolution_candidate_" + stableDigest(context.SourceWindow+"\x1f"+string(input.domain)+"\x1f"+fmt.Sprint(input.index))
		plans = append(plans, EvolutionCandidatePlan{CandidateID: candidateID, Domain: input.domain, Index: input.index, Operation: input.operation, TargetRef: input.targetRef, FieldPath: input.fieldPath, Direction: input.direction, Strength: input.strength, Confidence: input.confidence, EvidenceRefs: append([]string(nil), input.evidenceRefs...), Disposition: disposition, ReasonCode: reason})
	}
	proposalID := "reflection_proposal_" + stableDigest(context.SourceWindow+"\x1f"+jsonString(proposal))
	return ReflectionEvolutionPlan{ProposalID: proposalID, FluctlightID: context.FluctlightID, SourceWindow: context.SourceWindow, BaseWatermark: context.Watermark, ToSequence: context.ToSequence, ExpectedRevisions: cloneRevisionMap(context.BaseRevisions), Candidates: plans, PolicyVersion: reflectionPolicyVersion}, nil
}

func validateReflectionProposalReferences(proposal ReflectionProposalV2, context EvolutionContext) error {
	outcomeIDs := make(map[string]struct{}, len(context.Outcomes))
	for _, outcome := range context.Outcomes {
		outcomeIDs[outcome.ID] = struct{}{}
	}
	for index, candidate := range proposal.GoalCandidates {
		for _, ref := range candidate.OutcomeRefs {
			entry, ok := context.ReferenceIndex.ByRef[ref]
			if !ok || entry.Kind != ContextReferenceOutcome {
				return fmt.Errorf("reflection_goal_candidate_%d_foreign_outcome", index)
			}
			if _, ok := outcomeIDs[entry.EntityID]; !ok {
				return fmt.Errorf("reflection_goal_candidate_%d_outcome_missing", index)
			}
		}
	}
	for index, candidate := range proposal.IntentionCandidates {
		goalEntry, ok := context.ReferenceIndex.ByRef[candidate.GoalRef]
		if !ok || goalEntry.Kind != ContextReferenceGoal {
			return fmt.Errorf("reflection_intention_candidate_%d_foreign_goal", index)
		}
		if candidate.TargetRef != "" {
			target, ok := context.ReferenceIndex.ByRef[candidate.TargetRef]
			if !ok || target.Kind != ContextReferenceIntention || strings.TrimSpace(stringValue(decodeObject(target.Snapshot)["goal_ref"])) != candidate.GoalRef || strings.TrimSpace(stringValue(decodeObject(target.Snapshot)["goal_id"])) != goalEntry.EntityID {
				return fmt.Errorf("reflection_intention_candidate_%d_goal_rebind_forbidden", index)
			}
		}
		if candidate.TypedTrigger != nil && candidate.TypedTrigger.EventRef != "" {
			entry, ok := context.ReferenceIndex.ByRef[candidate.TypedTrigger.EventRef]
			if !ok {
				return fmt.Errorf("reflection_intention_candidate_%d_foreign_event", index)
			}
			switch entry.Kind {
			case ContextReferenceScene, ContextReferenceLifeContext, ContextReferenceSchedule, ContextReferenceScheduleItem, ContextReferencePresence, ContextReferenceOutcome:
			default:
				return fmt.Errorf("reflection_intention_candidate_%d_event_kind_invalid", index)
			}
		}
	}
	return nil
}

func collectReflectionCandidates(proposal ReflectionProposalV2) []reflectionCandidateInput {
	result := make([]reflectionCandidateInput, 0)
	for index, item := range proposal.MemoryCandidates {
		confidence, strength := item.Confidence, item.Importance
		if MemoryOperation(item.Operation) == MemoryConfirm || MemoryOperation(item.Operation) == MemoryDeprecate {
			confidence, strength = 1, 1
		}
		result = append(result, reflectionCandidateInput{domain: EvolutionMemory, index: index, operation: item.Operation, targetRef: item.TargetRef, confidence: confidence, strength: strength, evidenceRefs: item.EvidenceRefs, requiredKind: ContextReferenceMemory})
	}
	for index, item := range proposal.RelationshipObservations {
		result = append(result, reflectionCandidateInput{domain: EvolutionRelationship, index: index, operation: "observe", targetRef: item.TargetRef, direction: item.Direction, confidence: item.Confidence, strength: item.Strength, evidenceRefs: item.EvidenceRefs, requiredKind: ContextReferenceRelationship})
	}
	for index, item := range proposal.GoalCandidates {
		result = append(result, reflectionCandidateInput{domain: EvolutionGoal, index: index, operation: item.Operation, targetRef: item.TargetRef, direction: item.Direction, confidence: item.Confidence, strength: item.Strength, evidenceRefs: mergeStableRefs(item.EvidenceRefs, item.OutcomeRefs), requiredKind: ContextReferenceGoal})
	}
	for index, item := range proposal.IntentionCandidates {
		result = append(result, reflectionCandidateInput{domain: EvolutionIntention, index: index, operation: item.Operation, targetRef: item.TargetRef, confidence: item.Confidence, strength: item.Confidence, evidenceRefs: item.EvidenceRefs, requiredKind: ContextReferenceIntention})
	}
	if len(proposal.EmotionalSummary.EvidenceRefs) > 0 {
		result = append(result, reflectionCandidateInput{domain: EvolutionAffectProfile, index: 0, operation: "summarize", confidence: 1, strength: 1, evidenceRefs: proposal.EmotionalSummary.EvidenceRefs})
	}
	for index, item := range proposal.AffectRecalibrationCandidates {
		result = append(result, reflectionCandidateInput{domain: EvolutionAffectProfile, index: index + 1, operation: "recalibrate", fieldPath: item.Target, direction: item.Direction, confidence: item.Confidence, strength: item.Strength, evidenceRefs: item.EvidenceRefs})
	}
	appendSlots := func(domain EvolutionDomain, items []ReflectionSlotCandidateV2) {
		for index, item := range items {
			requiredKind := map[EvolutionDomain]ContextReferenceKind{EvolutionDrive: ContextReferenceDrive, EvolutionPreference: ContextReferencePreference, EvolutionTrigger: ContextReferenceTrigger}[domain]
			result = append(result, reflectionCandidateInput{domain: domain, index: index, operation: item.Operation, targetRef: item.TargetRef, fieldPath: item.Key, direction: item.Direction, confidence: item.Confidence, strength: item.Strength, evidenceRefs: item.EvidenceRefs, requiredKind: requiredKind})
		}
	}
	appendSlots(EvolutionDrive, proposal.DriveCandidates)
	appendSlots(EvolutionPreference, proposal.PreferenceCandidates)
	appendSlots(EvolutionTrigger, proposal.TriggerCandidates)
	for index, item := range proposal.DevelopingSelfCandidates {
		result = append(result, reflectionCandidateInput{domain: EvolutionDevelopingSelf, index: index, operation: item.Operation, targetRef: item.TargetRef, fieldPath: item.Category, confidence: item.Confidence, strength: item.Confidence, evidenceRefs: item.EvidenceRefs, requiredKind: ContextReferenceDevelopingSelf})
	}
	appendOverlays := func(domain EvolutionDomain, items []ReflectionOverlayCandidateV2) {
		for index, item := range items {
			result = append(result, reflectionCandidateInput{domain: domain, index: index, operation: "evolve", fieldPath: item.FieldPath, direction: item.Direction, confidence: item.Confidence, strength: item.Strength, evidenceRefs: item.EvidenceRefs})
		}
	}
	appendOverlays(EvolutionPersonality, proposal.PersonalityEvolutionCandidates)
	appendOverlays(EvolutionBehaviorPolicy, proposal.BehaviorPolicyEvolutionCandidates)
	return result
}

type EvolutionState struct {
	FluctlightID string         `json:"fluctlight_id"`
	Watermark    int            `json:"watermark"`
	Revisions    map[string]int `json:"revisions"`
}

type ReflectionDispositionCounts struct {
	Applied  int `json:"applied"`
	Rejected int `json:"rejected"`
	Deferred int `json:"deferred"`
	NoChange int `json:"no_change"`
}

type ReflectionApplyResult struct {
	Status       string                                          `json:"status"`
	ProposalID   string                                          `json:"proposal_id"`
	SourceWindow string                                          `json:"source_window"`
	Watermark    int                                             `json:"watermark"`
	Counts       map[EvolutionDomain]ReflectionDispositionCounts `json:"counts"`
	ChangedRefs  []string                                        `json:"changed_refs"`
	Revisions    map[string]int                                  `json:"revisions"`
	ReasonCodes  []string                                        `json:"reason_codes"`
}

type EvolutionDomainApplier func(EvolutionCandidatePlan) error

func ApplyReflectionPlan(state EvolutionState, plan ReflectionEvolutionPlan, appliers map[EvolutionDomain]EvolutionDomainApplier) (EvolutionState, ReflectionApplyResult, error) {
	if state.FluctlightID == "" || state.FluctlightID != plan.FluctlightID || state.Watermark != plan.BaseWatermark || plan.ToSequence <= plan.BaseWatermark || plan.PolicyVersion != reflectionPolicyVersion {
		return state, ReflectionApplyResult{}, errors.New("reflection_apply_window_stale")
	}
	for domain, expected := range plan.ExpectedRevisions {
		if state.Revisions[domain] != expected {
			return state, ReflectionApplyResult{}, fmt.Errorf("reflection_apply_%s_revision_stale", domain)
		}
	}
	working := EvolutionState{FluctlightID: state.FluctlightID, Watermark: state.Watermark, Revisions: cloneRevisionMap(state.Revisions)}
	result := ReflectionApplyResult{Status: "no_change", ProposalID: plan.ProposalID, SourceWindow: plan.SourceWindow, Watermark: state.Watermark, Counts: map[EvolutionDomain]ReflectionDispositionCounts{}, Revisions: cloneRevisionMap(state.Revisions)}
	reasons := map[string]struct{}{}
	for _, candidate := range plan.Candidates {
		counts := result.Counts[candidate.Domain]
		switch candidate.Disposition {
		case EvolutionAccepted:
			if applier := appliers[candidate.Domain]; applier != nil {
				if err := applier(candidate); err != nil {
					return state, ReflectionApplyResult{}, fmt.Errorf("reflection_apply_%s_failed: %w", candidate.Domain, err)
				}
			}
			working.Revisions[string(candidate.Domain)]++
			counts.Applied++
			result.Status = "applied"
			if candidate.TargetRef == "" || !contextReferencePattern.MatchString(candidate.TargetRef) {
				return state, ReflectionApplyResult{}, errors.New("reflection_changed_ref_missing")
			}
			result.ChangedRefs = append(result.ChangedRefs, candidate.TargetRef)
		case EvolutionRejected:
			counts.Rejected++
		case EvolutionDeferred:
			counts.Deferred++
		case EvolutionNoChange:
			counts.NoChange++
		default:
			return state, ReflectionApplyResult{}, errors.New("reflection_disposition_invalid")
		}
		result.Counts[candidate.Domain] = counts
		reasons[candidate.ReasonCode] = struct{}{}
	}
	working.Watermark = plan.ToSequence
	result.Watermark = plan.ToSequence
	result.Revisions = cloneRevisionMap(working.Revisions)
	for reason := range reasons {
		result.ReasonCodes = append(result.ReasonCodes, reason)
	}
	sort.Strings(result.ChangedRefs)
	sort.Strings(result.ReasonCodes)
	return working, result, nil
}

func cloneRevisionMap(source map[string]int) map[string]int {
	result := make(map[string]int, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
