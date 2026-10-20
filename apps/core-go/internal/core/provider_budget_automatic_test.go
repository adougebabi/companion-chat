package core

import "testing"

func TestAutomaticProviderInputBudgetUsesActualWindowAndPolicy(t *testing.T) {
	for _, c := range []struct {
		window, output int
		policy         string
		want           int
	}{{24576, 4069, promptBudgetPolicyVersionV1, 16411}, {24576, 4096, promptBudgetPolicyVersionV1, 16384}, {16384, 4096, promptBudgetPolicyVersionV2, 11776}} {
		got, err := automaticProviderInputBudget(c.window, c.output, c.policy)
		if err != nil || got != c.want {
			t.Fatalf("%+v got=%d err=%v", c, got, err)
		}
	}
	for _, c := range []struct {
		window, output int
		policy         string
	}{{8192, 4096, promptBudgetPolicyVersionV1}, {-1, 4096, promptBudgetPolicyVersionV1}, {16384, 0, promptBudgetPolicyVersionV1}, {16384, 4096, "unknown"}} {
		if _, err := automaticProviderInputBudget(c.window, c.output, c.policy); err == nil {
			t.Fatalf("accepted invalid budget %+v", c)
		}
	}
}
