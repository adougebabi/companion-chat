package core

import "time"

type goalEvaluationProviderPacket struct {
	StableDefinitions map[string]any `json:"stable_definitions"`
	Current           map[string]any `json:"current"`
}

type goalEvaluationWireOutput struct {
	Evaluations []goalEvaluationWireCandidate `json:"evaluations"`
	Plans       []goalEvaluationWirePlan      `json:"plans"`
}

type goalEvaluationWireJudgment struct {
	CriterionRef        string   `json:"criterion_ref"`
	CriterionQuote      string   `json:"criterion_quote"`
	OptionalImprovement string   `json:"optional_improvement"`
	Verdict             string   `json:"verdict"`
	Kind                string   `json:"kind"`
	Subject             string   `json:"subject"`
	Discourse           string   `json:"discourse"`
	EvidenceRefs        []string `json:"evidence_refs"`
	Reason              string   `json:"reason"`
}

type goalEvaluationWireCandidate struct {
	GoalRef                  string                                      `json:"goal_ref"`
	Judgments                []goalEvaluationWireJudgment                `json:"judgments"`
	Impact                   string                                      `json:"impact"`
	Blocker                  string                                      `json:"blocker"`
	WaitCondition            string                                      `json:"wait_condition"`
	NextStep                 string                                      `json:"next_step"`
	NextReviewAt             *time.Time                                  `json:"next_review_at,omitempty"`
	StageEvaluation          *goalEvaluationWireObjectEvaluation         `json:"stage_evaluation,omitempty"`
	CommitmentEvaluations    []goalEvaluationWireObjectEvaluation        `json:"commitment_evaluations,omitempty"`
	RelationshipConfirmation *goalEvaluationWireRelationshipConfirmation `json:"relationship_confirmation,omitempty"`
	Review                   *goalEvaluationWireReview                   `json:"review,omitempty"`
	ResidualMotivation       string                                      `json:"residual_motivation"`
	Followup                 *GoalFollowupCandidate                      `json:"followup,omitempty"`
}

type goalEvaluationWireObjectEvaluation struct {
	ObjectRef string                       `json:"object_ref"`
	Judgments []goalEvaluationWireJudgment `json:"judgments"`
	Completed bool                         `json:"completed"`
	Reason    string                       `json:"reason"`
}

type goalEvaluationWireRelationshipConfirmation struct {
	TargetActorRef string   `json:"target_actor_ref"`
	EvidenceRefs   []string `json:"evidence_refs"`
	Label          string   `json:"label"`
}

type goalEvaluationWireReview struct {
	ReasonCategory      string   `json:"reason_category"`
	Decision            string   `json:"decision"`
	Explanation         string   `json:"explanation"`
	EvidenceRefs        []string `json:"evidence_refs"`
	StageRef            string   `json:"stage_ref,omitempty"`
	FeasibleAlternative string   `json:"feasible_alternative"`
}

type goalEvaluationWirePlan struct {
	GoalRef       string                            `json:"goal_ref"`
	Reason        string                            `json:"reason"`
	NextStep      string                            `json:"next_step"`
	WaitCondition string                            `json:"wait_condition"`
	NextReviewAt  *time.Time                        `json:"next_review_at,omitempty"`
	Stage         *goalEvaluationWireStagePlan      `json:"stage,omitempty"`
	Commitment    *goalEvaluationWireCommitmentPlan `json:"commitment,omitempty"`
}

type goalEvaluationWireStagePlan struct {
	DependencyRefs []string `json:"dependency_refs,omitempty"`
	Operation      string   `json:"operation"`
	ObjectRef      string   `json:"object_ref,omitempty"`
	Purpose        string   `json:"purpose"`
	Strategy       string   `json:"strategy"`
	EntryBasis     string   `json:"entry_basis"`
	ExitBasis      string   `json:"exit_basis"`
	Criteria       []string `json:"criteria"`
	Reason         string   `json:"reason"`
}

type goalEvaluationWireCommitmentPlan struct {
	Operation            string     `json:"operation,omitempty"`
	Reason               string     `json:"reason,omitempty"`
	ObjectRef            string     `json:"object_ref,omitempty"`
	ExpectedResult       string     `json:"expected_result"`
	Criteria             []string   `json:"criteria"`
	WindowStart          *time.Time `json:"window_start,omitempty"`
	WindowEnd            *time.Time `json:"window_end,omitempty"`
	OpportunityCondition string     `json:"opportunity_condition"`
	Blocker              string     `json:"blocker"`
}
