package core

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const goalEvaluationPolicyVersion = "goal.evaluation.v2"

type GoalCriterion struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type GoalStage struct {
	DependencyIDs   []string        `json:"dependency_ids"`
	ID              string          `json:"id"`
	FluctlightID    string          `json:"fluctlight_id"`
	GoalID          string          `json:"goal_id"`
	ProfileID       string          `json:"profile_id"`
	Purpose         string          `json:"purpose"`
	Strategy        string          `json:"strategy"`
	EntryBasis      string          `json:"entry_basis"`
	ExitBasis       string          `json:"exit_basis"`
	Criteria        []GoalCriterion `json:"criteria"`
	CriteriaVersion int             `json:"criteria_version"`
	Status          string          `json:"status"`
	Revision        int             `json:"revision"`
	Reason          string          `json:"reason"`
}

type GoalCommitment struct {
	ID                   string          `json:"id"`
	FluctlightID         string          `json:"fluctlight_id"`
	GoalID               string          `json:"goal_id"`
	StageID              string          `json:"stage_id,omitempty"`
	ProfileID            string          `json:"profile_id"`
	ExpectedResult       string          `json:"expected_result"`
	Criteria             []GoalCriterion `json:"criteria"`
	CriteriaVersion      int             `json:"criteria_version"`
	WindowStart          *time.Time      `json:"window_start,omitempty"`
	WindowEnd            *time.Time      `json:"window_end,omitempty"`
	OpportunityCondition string          `json:"opportunity_condition"`
	Blocker              string          `json:"blocker"`
	Status               string          `json:"status"`
	Revision             int             `json:"revision"`
}

type GoalSource struct {
	Ref               string         `json:"ref"`
	EventID           int64          `json:"event_id"`
	Kind              string         `json:"kind"`
	ID                string         `json:"id"`
	Version           string         `json:"version"`
	FluctlightID      string         `json:"fluctlight_id"`
	ProfileID         string         `json:"profile_id"`
	ConversationID    string         `json:"conversation_id,omitempty"`
	SubjectActorID    string         `json:"subject_actor_id"`
	OccurredAt        time.Time      `json:"occurred_at"`
	RecordedAt        time.Time      `json:"recorded_at"`
	Valid             bool           `json:"valid"`
	CanSupportSuccess bool           `json:"can_support_success"`
	GoalIDs           []string       `json:"goal_ids,omitempty"`
	Data              map[string]any `json:"data"`
}

type GoalCriterionJudgment struct {
	Kind         string   `json:"kind"`
	CriterionID  string   `json:"criterion_id"`
	Verdict      string   `json:"verdict"`
	EvidenceRefs []string `json:"evidence_refs"`
	Discourse    string   `json:"discourse"`
	Subject      string   `json:"subject"`
	Reason       string   `json:"reason"`
}

type GoalEvaluationCandidate struct {
	Review                   *GoalReviewDecision           `json:"review,omitempty"`
	GoalID                   string                        `json:"goal_id"`
	ExpectedRevision         int                           `json:"expected_revision"`
	CriteriaVersion          int                           `json:"criteria_version"`
	Judgments                []GoalCriterionJudgment       `json:"judgments"`
	Impact                   string                        `json:"impact"`
	Blocker                  string                        `json:"blocker"`
	WaitCondition            string                        `json:"wait_condition"`
	NextStep                 string                        `json:"next_step"`
	NextReviewAt             *time.Time                    `json:"next_review_at,omitempty"`
	StageEvaluation          *GoalObjectEvaluation         `json:"stage_evaluation,omitempty"`
	CommitmentEvaluations    []GoalObjectEvaluation        `json:"commitment_evaluations,omitempty"`
	RelationshipConfirmation *GoalRelationshipConfirmation `json:"relationship_confirmation,omitempty"`
	ResidualMotivation       string                        `json:"residual_motivation"`
	Followup                 *GoalFollowupCandidate        `json:"followup,omitempty"`
}

type GoalObjectEvaluation struct {
	ID               string                  `json:"id"`
	ExpectedRevision int                     `json:"expected_revision"`
	CriteriaVersion  int                     `json:"criteria_version"`
	Judgments        []GoalCriterionJudgment `json:"judgments"`
	Completed        bool                    `json:"completed"`
	Reason           string                  `json:"reason"`
}

type GoalRelationshipConfirmation struct {
	relationshipID       string
	relationshipRevision int
	TargetActorID        string   `json:"target_actor_id"`
	EvidenceRefs         []string `json:"evidence_refs"`
	Label                string   `json:"label"`
}

type GoalFollowupCandidate struct {
	DesiredOutcome  string   `json:"desired_outcome"`
	SuccessCriteria []string `json:"success_criteria"`
	Motivation      string   `json:"motivation"`
}

type GoalPlanCandidate struct {
	GoalID           string              `json:"goal_id"`
	ExpectedRevision int                 `json:"expected_revision"`
	CriteriaVersion  int                 `json:"criteria_version"`
	Reason           string              `json:"reason"`
	NextStep         string              `json:"next_step"`
	WaitCondition    string              `json:"wait_condition"`
	NextReviewAt     *time.Time          `json:"next_review_at,omitempty"`
	Stage            *GoalStagePlan      `json:"stage,omitempty"`
	Commitment       *GoalCommitmentPlan `json:"commitment,omitempty"`
}

type GoalStagePlan struct {
	DependencyIDs    []string `json:"dependency_ids,omitempty"`
	Operation        string   `json:"operation"`
	ID               string   `json:"id,omitempty"`
	ExpectedRevision int      `json:"expected_revision"`
	Purpose          string   `json:"purpose"`
	Strategy         string   `json:"strategy"`
	EntryBasis       string   `json:"entry_basis"`
	ExitBasis        string   `json:"exit_basis"`
	Criteria         []string `json:"criteria"`
	Reason           string   `json:"reason"`
}

type GoalCommitmentPlan struct {
	Operation            string     `json:"operation,omitempty"`
	Reason               string     `json:"reason,omitempty"`
	ID                   string     `json:"id,omitempty"`
	ExpectedRevision     int        `json:"expected_revision"`
	ExpectedResult       string     `json:"expected_result"`
	Criteria             []string   `json:"criteria"`
	WindowStart          *time.Time `json:"window_start,omitempty"`
	WindowEnd            *time.Time `json:"window_end,omitempty"`
	OpportunityCondition string     `json:"opportunity_condition"`
	Blocker              string     `json:"blocker"`
}

type GoalEvaluationTaskOutput struct {
	Evaluations []GoalEvaluationCandidate `json:"evaluations"`
	Plans       []GoalPlanCandidate       `json:"plans"`
}

// Counter-evidence shares the same owner, persona and binding boundaries as
// success evidence. Withdrawn sources can explain regression but cannot prove success.
func validateGoalSourceScope(goal GoalAuthority, ref string, sources map[string]GoalSource) error {
	source, ok := sources[ref]
	if !ok || source.FluctlightID != goal.FluctlightID {
		return errors.New("goal_judgment_source_invalid")
	}
	if goal.ProfileID != "" && source.ProfileID != "" && source.ProfileID != goal.ProfileID {
		return errors.New("goal_judgment_profile_invalid")
	}
	if source.Kind == "message" && goal.ProfileID != "" && source.ProfileID == "" && stringValue(source.Data["message_kind"]) != "user" {
		return errors.New("goal_message_profile_unknown")
	}
	if len(source.GoalIDs) > 0 && !containsString(source.GoalIDs, goal.EntityID) {
		return errors.New("goal_judgment_binding_invalid")
	}
	return nil
}

func validateGoalSourceForJudgment(goal GoalAuthority, judgment GoalCriterionJudgment, sources map[string]GoalSource) error {
	if len(judgment.EvidenceRefs) == 0 || len(judgment.EvidenceRefs) > 16 {
		return errors.New("goal_judgment_evidence_required")
	}
	if judgment.Discourse != "assertion" && judgment.Discourse != "domain_fact" {
		return errors.New("goal_judgment_not_actual_event")
	}
	if judgment.Kind == "" {
		judgment.Kind = "semantic"
	}
	switch judgment.Kind {
	case "communication", "relationship", "acquisition", "information", "semantic":
	default:
		return errors.New("goal_judgment_kind_invalid")
	}
	speakers := map[string]bool{}
	conversations := map[string]bool{}

	for _, ref := range judgment.EvidenceRefs {
		source, ok := sources[ref]
		if !ok || !source.Valid || !source.CanSupportSuccess || source.FluctlightID != goal.FluctlightID {
			return errors.New("goal_judgment_source_invalid")
		}
		if err := validateGoalSourceScope(goal, ref, sources); err != nil {
			return err
		}
		if source.Kind == "message" && stringValue(source.Data["message_kind"]) == "assistant" && judgment.Kind != "communication" && judgment.Kind != "relationship" {
			return errors.New("goal_judgment_self_report_not_business_fact")
		}
		if goal.DeadlinePolicy == "hard" && goal.Deadline != nil && source.OccurredAt.After(*goal.Deadline) {
			return errors.New("goal_judgment_after_hard_deadline")
		}
		if (judgment.Kind == "communication" || judgment.Kind == "relationship") && source.Kind != "message" {
			return errors.New("goal_judgment_conversation_authority_required")
		}
		if judgment.Kind == "acquisition" && source.Kind != "item" && (source.Kind != "outcome" || len(arrayValue(source.Data["verified_item_ids"])) == 0) {
			return errors.New("goal_judgment_acquisition_authority_required")
		}
		if source.Kind == "message" {
			speakers[source.SubjectActorID] = true
			conversations[source.ConversationID] = true
			if goal.TargetActorID != "" && (judgment.Kind == "communication" || judgment.Kind == "relationship") && !containsString(decisionServiceRefValues(source.Data["participants"]), goal.TargetActorID) {
				return errors.New("goal_judgment_recipient_mismatch")
			}
		}
		if source.Kind == "actor_fact" && judgment.Subject == "actor_self" && source.SubjectActorID != goal.FluctlightID {
			return errors.New("goal_judgment_actor_mismatch")
		}

	}
	if judgment.Kind == "relationship" && judgment.Subject != "both" {
		return errors.New("goal_judgment_mutual_evidence_required")
	}
	switch judgment.Subject {
	case "actor_self":
		if len(speakers) > 0 && !speakers[goal.FluctlightID] {
			return errors.New("goal_judgment_actor_mismatch")
		}
	case "target_actor":
		if goal.TargetActorID == "" || !speakers[goal.TargetActorID] {
			return errors.New("goal_judgment_actor_mismatch")
		}
	case "both":
		if len(conversations) != 1 {
			return errors.New("goal_judgment_conversation_mismatch")
		}
		if goal.TargetActorID == "" || !speakers[goal.FluctlightID] || !speakers[goal.TargetActorID] {
			return errors.New("goal_judgment_mutual_evidence_required")
		}
	case "domain":
		for _, ref := range judgment.EvidenceRefs {
			if sources[ref].Kind == "message" {
				return errors.New("goal_judgment_domain_authority_required")
			}
		}
	default:
		return errors.New("goal_judgment_subject_invalid")
	}
	return nil
}

func validatedGoalJudgments(goal GoalAuthority, proposed []GoalCriterionJudgment, sources map[string]GoalSource) ([]GoalCriterionJudgment, bool, float64, error) {
	ids := goalCriterionIDs(goal)
	byID := map[string]GoalCriterionJudgment{}
	for _, j := range proposed {
		if !containsString(ids, j.CriterionID) || byID[j.CriterionID].CriterionID != "" || len([]rune(j.Reason)) > 1000 {
			return nil, false, 0, errors.New("goal_judgment_criterion_invalid")
		}
		switch j.Verdict {
		case "satisfied":
			if err := validateGoalSourceForJudgment(goal, j, sources); err != nil {
				return nil, false, 0, err
			}
		case "not_satisfied", "unknown":
			if len(j.EvidenceRefs) > 16 {
				return nil, false, 0, errors.New("goal_judgment_evidence_limit")
			}
			for _, ref := range j.EvidenceRefs {
				if err := validateGoalSourceScope(goal, ref, sources); err != nil {
					return nil, false, 0, err
				}
			}
		default:
			return nil, false, 0, errors.New("goal_judgment_verdict_invalid")
		}
		byID[j.CriterionID] = j
	}
	result := make([]GoalCriterionJudgment, 0, len(ids))
	satisfied := 0
	for _, id := range ids {
		j, ok := byID[id]
		if !ok {
			j = GoalCriterionJudgment{CriterionID: id, Verdict: "unknown", EvidenceRefs: []string{}, Discourse: "uncertain", Subject: "actor_self", Reason: "no applicable evidence"}
		}
		if j.Verdict == "satisfied" {
			satisfied++
		}
		result = append(result, j)
	}
	if len(ids) == 0 {
		return result, false, 0, nil
	}
	policy := nonNilGoalCriteriaPolicy(goal.CriteriaPolicy)
	mode := firstString(policy["mode"], "all")
	if mode == "any" {
		if satisfied > 0 {
			return result, true, 1, nil
		}
		return result, false, 0, nil
	}
	if mode != "all" {
		return nil, false, 0, errors.New("goal_criteria_policy_invalid")
	}
	optional := decisionServiceRefValues(policy["optional_ids"])
	required, requiredSatisfied := 0, 0
	for _, id := range optional {
		if !containsString(ids, id) {
			return nil, false, 0, errors.New("goal_optional_criterion_invalid")
		}
	}
	for _, j := range result {
		if !containsString(optional, j.CriterionID) {
			required++
			if j.Verdict == "satisfied" {
				requiredSatisfied++
			}
		}
	}
	if required == 0 {
		return nil, false, 0, errors.New("goal_required_criteria_missing")
	}
	return result, requiredSatisfied == required, float64(requiredSatisfied) / float64(required), nil
}

// This is the sole semantic completion rule. Sources are Core-read records;
// structured judgments are candidates, never a substitute for provenance.
func ApplyGoalEvaluation(goal GoalAuthority, candidate GoalEvaluationCandidate, sources map[string]GoalSource, at time.Time) (GoalAuthority, GoalGovernanceRecord, []GoalCriterionJudgment, error) {
	if candidate.GoalID != goal.EntityID || candidate.ExpectedRevision != goal.Revision || candidate.CriteriaVersion != effectiveGoalCriteriaVersion(goal) {
		return GoalAuthority{}, GoalGovernanceRecord{}, nil, errors.New("goal_evaluation_version_conflict")
	}
	judgments, complete, progress, err := validatedGoalJudgments(goal, candidate.Judgments, sources)
	if err != nil {
		return GoalAuthority{}, GoalGovernanceRecord{}, nil, err
	}
	switch candidate.Impact {
	case "progressed", "no_change", "blocked", "regressed", "needs_evidence", "completed":
	default:
		return GoalAuthority{}, GoalGovernanceRecord{}, nil, errors.New("goal_evaluation_impact_invalid")
	}
	if candidate.Impact == "no_change" {
		known := false
		for _, judgment := range judgments {
			known = known || judgment.Verdict != "unknown"
		}
		if !known {
			return GoalAuthority{}, GoalGovernanceRecord{}, nil, errors.New("goal_evaluation_unknown_requires_evidence")
		}
	}
	if candidate.Impact == "completed" && !complete {
		return GoalAuthority{}, GoalGovernanceRecord{}, nil, errors.New("goal_completion_criteria_incomplete")
	}
	// A fully satisfied assessment must request settlement explicitly. Reject
	// contradictory candidates rather than memoizing an active Goal at 100%.
	if complete && candidate.Impact != "completed" {
		return GoalAuthority{}, GoalGovernanceRecord{}, nil, errors.New("goal_evaluation_completion_impact_mismatch")
	}
	next := goal
	operation := GoalUpdate
	if goal.Status == GoalActive {
		next.Progress = progress
		if candidate.Impact == "completed" {
			next.Status = GoalCompleted
			operation = GoalComplete
		}
	}
	refs := []string{}
	for _, j := range judgments {
		refs = mergeStableRefs(refs, j.EvidenceRefs)
	}
	if len(refs) == 0 {
		refs = goal.EvidenceRefs
	}
	next.EvidenceRefs = mergeStableRefs(goal.EvidenceRefs, refs)
	next.Revision++
	if err := next.Validate(); err != nil {
		return GoalAuthority{}, GoalGovernanceRecord{}, nil, err
	}
	return next, GoalGovernanceRecord{GoalRef: goal.Ref, Operation: operation, FromStatus: goal.Status, ToStatus: next.Status, BaseRevision: goal.Revision, Revision: next.Revision, EvidenceRefs: refs, Reason: "versioned Goal evaluation: " + candidate.Impact, PolicyVersion: goalEvaluationPolicyVersion, OccurredAt: at.UTC()}, judgments, nil
}

func validateGoalCriteria(criteria []GoalCriterion) error {
	if len(criteria) == 0 || len(criteria) > 16 {
		return errors.New("goal_subobject_criteria_invalid")
	}
	seen := map[string]bool{}
	for _, c := range criteria {
		if c.ID == "" || seen[c.ID] || strings.TrimSpace(c.Text) == "" || len([]rune(c.Text)) > 500 {
			return errors.New("goal_subobject_criteria_invalid")
		}
		seen[c.ID] = true
	}
	return nil
}

func newGoalObjectCriteria(id string, version int, texts []string) []GoalCriterion {
	result := make([]GoalCriterion, len(texts))
	for i, text := range texts {
		result[i] = GoalCriterion{ID: "criterion_" + stableDigest(id+"\x1f"+fmt.Sprint(version)+"\x1f"+fmt.Sprint(i)), Text: text}
	}
	return result
}
