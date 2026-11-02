package browser

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

type wardrobeRecoveryBackend struct {
	fakeBackend
	body              map[string]any
	endpoint, session string
}

func (b *wardrobeRecoveryBackend) DoJSON(ctx context.Context, method, endpoint, session string, body any) (map[string]any, error) {
	if strings.HasSuffix(endpoint, "/wardrobe/recover") {
		b.body, b.endpoint, b.session = body.(map[string]any), endpoint, session
		return map[string]any{"intent_id": "wardrobe_initialize:fl-1", "workflow_id": "wardrobe_initialize:fl-1", "status": "pending", "restarted": true}, nil
	}
	return b.fakeBackend.DoJSON(ctx, method, endpoint, session, body)
}

func TestWardrobeRecoveryBoundaryValidatesAndForwardsOwnerCommand(t *testing.T) {
	b := &wardrobeRecoveryBackend{}
	h := newBrowserTestHandler(b)
	path := "https://api.test/api/fluctlights/fl-1/wardrobe/recover"
	body := `{"reason":"模型已恢复","idempotencyKey":"original-command"}`
	headers := map[string]string{"Origin": "https://fluctlight.test", "X-CSRF-Token": "csrf"}
	cookies := map[string]string{sessionCookieName: "opaque", csrfCookieName: "csrf"}
	if r := invokeBrowser(h, http.MethodPost, path, body, headers, cookies); r.Code != 200 || b.endpoint != "/internal/fluctlights/fl-1/wardrobe/recover" || b.session != "opaque" || b.body["idempotency_key"] != "original-command" || b.body["reason"] != "模型已恢复" || !strings.Contains(r.Body.String(), `"restarted":true`) {
		t.Fatal("recovery transport lost identity", r.Code, r.Body.String(), b)
	}
	for _, invalid := range []string{`{}`, `{"reason":"retry"}`, `{"reason":" ","idempotencyKey":"key"}`, `{"reason":"retry","idempotencyKey":"key","actorId":"forged"}`, `{"reason":1,"idempotencyKey":"key"}`, `{"reason":"retry","idempotencyKey":"` + strings.Repeat("k", 257) + `"}`} {
		b.endpoint = ""
		if r := invokeBrowser(h, http.MethodPost, path, invalid, headers, cookies); r.Code != 400 || b.endpoint != "" {
			t.Fatal("invalid recovery reached Core", invalid, r.Code, b.endpoint)
		}
	}
	b.endpoint = ""
	if r := invokeBrowser(h, http.MethodPost, path, body, headers, map[string]string{csrfCookieName: "csrf"}); r.Code != 401 || b.endpoint != "" {
		t.Fatal("recovery allowed missing session", r.Code)
	}
	if r := invokeBrowser(h, http.MethodPost, path, body, nil, cookies); r.Code != 403 || b.endpoint != "" {
		t.Fatal("recovery allowed missing CSRF", r.Code)
	}
}
