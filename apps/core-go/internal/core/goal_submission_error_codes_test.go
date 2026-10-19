package core

import "testing"

func TestGoalSubmissionBusinessCodesPreserveStaleRecoveryAndSpecificWarnings(t *testing.T) {
	for _, tc := range []struct {
		cause error
		code  string
		stale bool
	}{
		{ErrConflict, "conflict", true},
		{ErrFoundationRevisionStale, "foundation_revision_stale", true},
		{ErrLifeContextStale, "life_context_stale", true},
		{newCapabilityError("goal_dependency_unsatisfied", false, ErrConflict), "goal_dependency_unsatisfied", false},
	} {
		code := goalSubmissionBusinessCode(tc.cause)
		if code != tc.code || goalEvaluationStaleSubmissionError(code) != tc.stale {
			t.Fatalf("submission recovery classification: cause=%v code=%q stale=%v", tc.cause, code, goalEvaluationStaleSubmissionError(code))
		}
	}
}
