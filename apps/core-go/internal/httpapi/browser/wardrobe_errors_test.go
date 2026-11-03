package browser

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

type wardrobeErrorBackend struct {
	fakeBackend
	err error
}

func (b *wardrobeErrorBackend) DoJSON(ctx context.Context, method, endpoint, session string, body any) (map[string]any, error) {
	if strings.Contains(endpoint, "/wardrobe") {
		return nil, b.err
	}
	return b.fakeBackend.DoJSON(ctx, method, endpoint, session, body)
}

func TestWardrobeRoutesReturnChineseFieldValidation(t *testing.T) {
	for _, tc := range []struct{ field, reason, message string }{
		{"category", "required", "category 字段不能为空"},
		{"description", "required", "description 字段不能为空"},
		{"ownership", "invalid", "ownership 字段不合法"},
		{"seasons", "must_be_array", "seasons 字段必须为数组"},
		{"clothing_type", "slot_mismatch", "clothing_type 字段必须与 slot（穿戴位置）匹配"},
		{"worn", "must_be_boolean", "worn 字段必须为布尔值"},
	} {
		for _, method := range []string{http.MethodPost, http.MethodPut} {
			t.Run(method+"/"+tc.field, func(t *testing.T) {
				err := &CoreError{Status: 422, Code: "wardrobe_item_invalid", Message: "http://internal.invalid/private SQLSTATE secret", Details: map[string]any{
					"field": tc.field, "reason": tc.reason, "item_index": float64(2),
					"cause": "private database error", "value": "secret", "provider_url": "http://internal.invalid",
				}}
				h := newBrowserTestHandler(&wardrobeErrorBackend{err: fmt.Errorf("wrapped: %w", err)})
				path := "/api/fluctlights/fl-1/wardrobe/items"
				if method == http.MethodPut {
					path += "/coat"
				}
				r := surfaceInvoke(h, method, path, `{}`, true, true)
				value := r.json(t)
				details := mapValue(value["details"])
				if r.code != 422 || value["code"] != "wardrobe_item_invalid" || value["message"] != "第 2 件物品："+tc.message || details["field"] != tc.field || details["reason"] != tc.reason || details["item_index"] != float64(2) || len(details) != 3 {
					t.Fatalf("validation lost reason or leaked details: %d %s", r.code, r.body)
				}
				assertWardrobeResponseHasNoInternalData(t, r)
			})
		}
	}
}

func TestWardrobeRoutesLocalizeMissingItems(t *testing.T) {
	for _, tc := range []struct{ method, path, coreCode string }{
		{http.MethodPut, "/wardrobe/items/missing", "not_found"},
		{http.MethodDelete, "/wardrobe/items/missing", "not_found"},
		{http.MethodPost, "/wardrobe/wear", "wardrobe_item_not_found"},
		{http.MethodPost, "/wardrobe/items/missing/use", "wardrobe_item_not_found"},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			h := newBrowserTestHandler(&wardrobeErrorBackend{err: &CoreError{Status: 404, Code: tc.coreCode, Replay: true, Message: "Wardrobe item was not found; secret", Details: map[string]any{"cause": "private"}}})
			r := surfaceInvoke(h, tc.method, "/api/fluctlights/fl-1"+tc.path, `{}`, true, true)
			value := r.json(t)
			if r.code != 404 || value["code"] != "wardrobe_item_not_found" || value["message"] != "未找到该衣橱物品" || value["details"] != nil {
				t.Fatalf("missing item = %d %s", r.code, r.body)
			}
			if tc.coreCode == "wardrobe_item_not_found" && r.header.Get("Idempotent-Replayed") != "true" {
				t.Fatal("localized wardrobe error lost replay marker")
			}
			assertWardrobeResponseHasNoInternalData(t, r)
		})
	}
}

func TestWardrobeRoutesHideInternalFailures(t *testing.T) {
	for _, tc := range []struct {
		method, path, code, message string
		status                      int
	}{
		{http.MethodGet, "/wardrobe", "wardrobe_unavailable", "暂时无法读取衣橱，请稍后重试", 404},
		{http.MethodPost, "/wardrobe/items", "wardrobe_add_failed", "衣橱物品添加失败，请稍后重试", 422},
		{http.MethodPut, "/wardrobe/items/coat", "wardrobe_update_failed", "衣橱物品更新失败，请稍后重试", 422},
		{http.MethodDelete, "/wardrobe/items/coat", "wardrobe_delete_failed", "衣橱物品删除失败，请稍后重试", 422},
	} {
		for i, failure := range []error{
			errors.New("postgres://secret@internal/private SQLSTATE 23505"),
			&CoreError{Status: 502, Code: "private_failure", Message: "http://internal.invalid secret", Details: map[string]any{"cause": "private"}},
			&CoreError{Status: 502, Code: "wardrobe_item_invalid", Message: "http://internal.invalid secret", Details: map[string]any{"field": "category", "reason": "required"}},
		} {
			t.Run(fmt.Sprintf("%s%s/%d", tc.method, tc.path, i), func(t *testing.T) {
				h := newBrowserTestHandler(&wardrobeErrorBackend{err: failure})
				r := surfaceInvoke(h, tc.method, "/api/fluctlights/fl-1"+tc.path, `{}`, true, true)
				value := r.json(t)
				if r.code != tc.status || value["code"] != tc.code || value["message"] != tc.message || value["details"] != nil {
					t.Fatalf("internal failure = %d %s", r.code, r.body)
				}
				assertWardrobeResponseHasNoInternalData(t, r)
			})
		}
	}
	for _, details := range []map[string]any{
		{"field": "http://internal.invalid/secret", "reason": "required"},
		{"field": "category", "reason": "SQLSTATE private"},
		nil,
	} {
		h := newBrowserTestHandler(&wardrobeErrorBackend{err: &CoreError{Status: 422, Code: "wardrobe_item_invalid", Message: "secret", Details: details}})
		r := surfaceInvoke(h, http.MethodPost, "/api/fluctlights/fl-1/wardrobe/items", `{}`, true, true)
		value := r.json(t)
		if r.code != 422 || value["message"] != "衣橱物品字段校验失败，请检查填写内容" || value["details"] != nil {
			t.Fatalf("unrecognized validation details = %d %s", r.code, r.body)
		}
		assertWardrobeResponseHasNoInternalData(t, r)
	}
}

func TestWardrobeCursorValidationIsChinese(t *testing.T) {
	h := newBrowserTestHandler(&fakeBackend{})
	r := surfaceInvoke(h, http.MethodGet, "/api/fluctlights/fl-1/wardrobe?cursor="+strings.Repeat("x", 129), "", true, false)
	value := r.json(t)
	if r.code != 400 || value["code"] != "wardrobe_cursor_invalid" || value["message"] != "衣橱分页参数无效，请刷新后重试" {
		t.Fatalf("cursor = %d %s", r.code, r.body)
	}
}

func assertWardrobeResponseHasNoInternalData(t *testing.T, r *httpResponse) {
	t.Helper()
	for _, text := range []string{"http://", "postgres://", "SQLSTATE", "private", "secret", "provider_url"} {
		if strings.Contains(string(r.body), text) {
			t.Fatalf("response leaked %s: %s", text, r.body)
		}
	}
}
