package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestR20ProviderRoleLookupPreservesExactMissingBindingCause(t *testing.T) {
	cause := fmt.Errorf("lookup: %w", pgx.ErrNoRows)
	err := providerAssignmentLookupError("generic_llm", cause)
	var unassigned *providerRoleUnassignedError
	if !errors.As(err, &unassigned) || unassigned.role != "generic_llm" || unassigned.cause != cause || !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("role lookup lost typed identity/cause: %v", err)
	}
	if want := "provider role generic_llm unavailable: " + cause.Error(); err.Error() != want {
		t.Fatalf("diagnostic message changed: got %q, want %q", err, want)
	}
	if code, reason := wardrobePreparationFailure(fmt.Errorf("agent failed: %w", err)); code != "wardrobe_provider_unconfigured" || !strings.Contains(reason, "模型用途") {
		t.Fatalf("missing binding not actionable: %s %s", code, reason)
	}
	connection := errors.New("connection refused")
	err = providerAssignmentLookupError("generic_llm", connection)
	if errors.As(err, &unassigned) || !errors.Is(err, connection) {
		t.Fatalf("ordinary DB failure misclassified or cause lost: %v", err)
	}
	if code, _ := wardrobePreparationFailure(err); code != "wardrobe_provider_assignment_unavailable" {
		t.Fatalf("ordinary DB failure classified as configuration: %s", code)
	}
}

func TestR20ProviderLookupDeadlineIsNotModelTimeout(t *testing.T) {
	err := providerAssignmentLookupError("generic_llm", context.DeadlineExceeded)
	code, reason := wardrobePreparationFailure(err)
	if code != "wardrobe_provider_assignment_unavailable" || !errors.Is(err, context.DeadlineExceeded) || strings.Contains(reason, "模型响应超时") {
		t.Fatalf("lookup deadline became model failure: %s %s %v", code, reason, err)
	}
}

func TestR20WardrobeContextFailureKeepsStageWithoutGuessingCause(t *testing.T) {
	for stage, step := range map[string]string{
		"resource": "角色资料", "effective_profile": "当前人格", "persona_detail": "人格详情",
		"life_snapshot": "生活状态", "wardrobe_items": "衣物目录", "wardrobe_revision": "衣柜版本", "wardrobe_plans": "穿搭方案",
	} {
		t.Run(stage, func(t *testing.T) {
			cause := fmt.Errorf("read failed: %w", pgx.ErrNoRows)
			err := wardrobeContextReadFailure(stage, cause)
			var typed *wardrobePreparationContextReadError
			if !errors.As(fmt.Errorf("preparation: %w", err), &typed) || typed.stage != stage || !errors.Is(err, pgx.ErrNoRows) || errors.Unwrap(err) != cause {
				t.Fatalf("context stage/cause lost: %v", err)
			}
			code, reason := wardrobePreparationFailure(err)
			if code != "wardrobe_preparation_context_failed" || !strings.Contains(reason, step) || !strings.Contains(reason, "调用模型前") {
				t.Fatalf("actual stage missing: %s %s", code, reason)
			}
			for _, incorrect := range []string{"未分配", "模型响应", "sql:", "no rows"} {
				if strings.Contains(reason, incorrect) {
					t.Fatalf("product reason guesses or exposes raw cause: %s", reason)
				}
			}
		})
	}
	// A storage deadline before model I/O cannot establish a model timeout.
	if code, _ := wardrobePreparationFailure(wardrobeContextReadFailure("life_snapshot", context.DeadlineExceeded)); code != "wardrobe_preparation_context_failed" {
		t.Fatalf("context deadline blamed on model: %s", code)
	}
}

func TestR20WardrobeUnknownAndExistingFailuresRemainDistinct(t *testing.T) {
	for _, err := range []error{pgx.ErrNoRows, errors.New("sql: no rows in result set"), errors.New("provider role generic_llm preflight failed"), errors.New("unexpected preparation error")} {
		code, reason := wardrobePreparationFailure(err)
		if code != "wardrobe_preparation_failed" || strings.Contains(reason, "检查模型") || strings.Contains(reason, "未分配") || strings.Contains(reason, "sql:") {
			t.Fatalf("unknown/historical failure guessed: %v => %s %s", err, code, reason)
		}
	}
	for _, test := range []struct {
		err  error
		code string
	}{
		{context.Canceled, "wardrobe_preparation_cancelled"},
		{wardrobeContextReadFailure("resource", context.Canceled), "wardrobe_preparation_cancelled"},
		{context.DeadlineExceeded, "wardrobe_provider_timeout"},
		{&structuredResponseTruncatedError{role: "cognitive_assessment"}, "wardrobe_response_truncated"},
		{errors.New("wardrobe_initial_plans_missing"), "wardrobe_plan_unavailable"},
		{errors.New("adk_final_output_invalid: required field"), "wardrobe_response_schema_invalid"},
		{errors.New("structured_response_invalid_json"), "wardrobe_response_invalid_json"},
	} {
		if code, _ := wardrobePreparationFailure(test.err); code != test.code {
			t.Fatalf("existing failure changed: %v => %s, want %s", test.err, code, test.code)
		}
	}
}
