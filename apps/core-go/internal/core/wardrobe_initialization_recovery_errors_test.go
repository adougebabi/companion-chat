package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestWardrobeInitializationRecoveryPreviousErrorUsesUserMessage(t *testing.T) {
	for _, test := range []struct {
		name, cause, want string
	}{
		{
			name:  "provider_error",
			cause: `provider request failed: failed to create chat completion: Post "http://100.80.75.9:11234/v1/chat/completions": provider HTTP 400: {"error":{"message":"Prompt (1357 tokens) requires ~1186MB GPU memory but only ~336MB is available and the hot prefix cache holds ~0MB more. Reduce prompt size, lower --ctx-size, or use a smaller model.","type":"invalid_request_error","code":400}}`,
			want:  "初始衣柜准备未完成，原资料和已完成步骤仍保留。请查看原任务中的诊断记录，确认原因后恢复。",
		},
		{
			name:  "missing_initial_plans",
			cause: "wardrobe_initial_plans_missing",
			want:  "模型没有提供可执行的初始穿搭方案，衣柜尚未准备完成。可补充已有的穿衣偏好或登记可用衣物后恢复；未知的身体资料无需编造。",
		},
		{
			name:  "provider_timeout",
			cause: `provider request failed: Post "http://100.80.75.9:11234/v1/chat/completions": context deadline exceeded`,
			want:  "初始衣柜模型响应超时，尚未确认准备完成。可以检查模型用途中的超时设置后恢复。",
		},
	} {
		for _, status := range []string{"failed", "dead_letter"} {
			t.Run(test.name+"/"+status, func(t *testing.T) {
				tx := &wardrobeRecoveryErrorTx{cause: test.cause, status: status}
				ctx := context.WithValue(context.Background(), surfaceDBTxKey{}, tx)
				out, err := (&App{}).RecoverInitialWardrobe(ctx, "owner", "fl-1", map[string]any{"reason": "恢复准备", "idempotency_key": "recovery"})
				if err != nil {
					t.Fatal(err)
				}
				if out["previous_error"] != test.want {
					t.Fatalf("previous_error = %q, want %q", out["previous_error"], test.want)
				}
				if out["restarted"] != true || out["status"] != "pending" || out["intent_id"] != "wardrobe_initialize:fl-1" || out["workflow_id"] != "wardrobe_initialize:fl-1" {
					t.Fatalf("recovery response lost stable identity: %#v", out)
				}
				body, err := json.Marshal(out)
				if err != nil {
					t.Fatal(err)
				}
				for _, internal := range []string{test.cause, "http://", "100.80.75.9", "11234", "GPU", "1186MB", "336MB", "--ctx-size", "wardrobe_initial_plans_missing", "deadline exceeded"} {
					if strings.Contains(string(body), internal) {
						t.Fatalf("recovery response exposes internal error %q: %s", internal, body)
					}
				}
			})
		}
	}
}

// Exercise the real recovery function without a database or a model service.
type wardrobeRecoveryErrorTx struct {
	pgx.Tx
	cause, status string
}

func (tx *wardrobeRecoveryErrorTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (tx *wardrobeRecoveryErrorTx) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	if strings.Contains(query, "p.intent_type='persona.compile'") {
		return wardrobeRecoveryErrorRow(func(...any) error { return pgx.ErrNoRows })
	}
	if strings.Contains(query, "SELECT status FROM public.fluctlights") {
		return wardrobeRecoveryErrorRow(func(dest ...any) error {
			*dest[0].(*string) = "active"
			return nil
		})
	}
	if strings.Contains(query, "FROM public.fluctlight_wardrobe_preparations") {
		return wardrobeRecoveryErrorRow(func(dest ...any) error {
			for index, value := range []string{"preparation", "wardrobe_initialize:fl-1", "wardrobe_initialize:fl-1", tx.status, "failed"} {
				*dest[index].(*string) = value
			}
			*dest[5].(**string) = &tx.cause
			return nil
		})
	}
	panic("unexpected recovery query: " + query)
}

type wardrobeRecoveryErrorRow func(...any) error

func (row wardrobeRecoveryErrorRow) Scan(dest ...any) error { return row(dest...) }
