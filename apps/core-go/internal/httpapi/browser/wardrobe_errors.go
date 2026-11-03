package browser

import (
	"errors"
	"fmt"
	"net/http"
)

func wardrobeRouteError(status int, code, message string) routeError {
	return func(w http.ResponseWriter, err error) {
		if writeWardrobePublicError(w, err) {
			return
		}
		var coreErr *CoreError
		if errors.As(err, &coreErr) && coreErr.Status == http.StatusNotFound {
			if code == "wardrobe_update_failed" || code == "wardrobe_delete_failed" {
				writeError(w, http.StatusNotFound, "wardrobe_item_not_found", "未找到该衣橱物品")
			} else {
				writeError(w, http.StatusNotFound, "wardrobe_not_found", "未找到衣橱或对应角色")
			}
			return
		}
		// A storage/provider failure must never become a public diagnostic.
		writeError(w, status, code, message)
	}
}

func writeWardrobePublicError(w http.ResponseWriter, err error) bool {
	var coreErr *CoreError
	if !errors.As(err, &coreErr) || coreErr == nil {
		return false
	}
	if coreErr.Code == "wardrobe_item_invalid" && coreErr.Status == http.StatusUnprocessableEntity {
		message, details := wardrobeValidationDetails(coreErr.Details)
		if coreErr.Replay {
			w.Header().Set("Idempotent-Replayed", "true")
		}
		writeErrorWithDetails(w, http.StatusUnprocessableEntity, coreErr.Code, message, details)
		return true
	}
	type publicError struct {
		status  int
		message string
	}
	known := map[string]publicError{
		"wardrobe_item_not_found":   {404, "未找到该衣橱物品"},
		"wardrobe_not_found":        {404, "未找到衣橱"},
		"wardrobe_item_unavailable": {409, "该物品当前不属于角色或不可用"},
		"wardrobe_slot_mismatch":    {422, "该衣橱物品与指定的穿戴位置不匹配"},
		"wardrobe_cursor_invalid":   {400, "衣橱分页参数无效，请刷新后重试"},
	}
	if value, ok := known[coreErr.Code]; ok && coreErr.Status == value.status {
		if coreErr.Replay {
			w.Header().Set("Idempotent-Replayed", "true")
		}
		writeError(w, value.status, coreErr.Code, value.message)
		return true
	}
	return false
}

// Reconstruct validation messages from a closed vocabulary. CoreError.Message
// and unrecognized details may contain submitted values or internal errors.
func wardrobeValidationDetails(raw map[string]any) (string, map[string]any) {
	field, _ := raw["field"].(string)
	reason, _ := raw["reason"].(string)
	fields := map[string]bool{
		"items": true, "item": true, "id": true, "item_kind": true,
		"category": true, "slot": true, "description": true, "ownership": true,
		"availability": true, "worn": true, "clothing_type": true, "seasons": true,
	}
	reasons := map[string]string{
		"required": "不能为空", "invalid": "不合法", "must_be_text": "必须为文本",
		"must_be_array": "必须为数组", "must_be_object": "必须为对象",
		"must_be_boolean": "必须为布尔值", "must_be_empty": "必须为空（普通物品不可设置穿戴位置）",
		"cannot_be_worn": "不能为 true（普通物品不能穿戴）", "slot_mismatch": "必须与 slot（穿戴位置）匹配",
	}
	text, ok := reasons[reason]
	if !fields[field] || !ok {
		return "衣橱物品字段校验失败，请检查填写内容", nil
	}
	message := field + " 字段" + text
	details := map[string]any{"field": field, "reason": reason}
	index := 0
	switch value := raw["item_index"].(type) {
	case int:
		index = value
	case float64:
		if float64(int(value)) == value {
			index = int(value)
		}
	}
	if index > 0 {
		details["item_index"] = index
		message = fmt.Sprintf("第 %d 件物品：%s", index, message)
	}
	return message, details
}
