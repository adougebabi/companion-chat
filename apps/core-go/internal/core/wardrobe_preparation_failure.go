package core

import (
	"context"
	"errors"
	"strings"
)

func wardrobePreparationFailure(err error) (string, string) {
	var truncated *structuredResponseTruncatedError
	terminal := wardrobeRepairCause(err)
	switch {
	case errors.Is(err, context.Canceled):
		return "wardrobe_preparation_cancelled", "本次初始衣柜准备已中断，原资料和已完成步骤仍保留。"
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline exceeded"):
		return "wardrobe_provider_timeout", "初始衣柜模型响应超时，尚未确认准备完成。可以检查模型用途中的超时设置后恢复。"
	case errors.As(err, &truncated):
		return "wardrobe_response_truncated", "初始衣柜模型以长度终止状态结束，完整方案尚未确认。请查看原始生成记录后恢复。"
	case strings.Contains(terminal.Error(), "wardrobe_initial_plans_missing"):
		return "wardrobe_plan_unavailable", "模型没有提供可执行的初始穿搭方案，衣柜尚未准备完成。可补充已有的穿衣偏好或登记可用衣物后恢复；未知的身体资料无需编造。"
	case strings.Contains(terminal.Error(), "adk_final_output_invalid"):
		return "wardrobe_response_schema_invalid", "模型返回的衣柜字段没有通过契约校验。角色资料已保留，请查看原任务中的生成与修复记录后恢复。"
	case strings.Contains(terminal.Error(), "structured_response_invalid_json"):
		return "wardrobe_response_invalid_json", "模型没有返回完整可解析的衣柜 JSON。原资料已保留，可以恢复此环节。"
	default:
		return "wardrobe_preparation_failed", ownerRequestFailureReason(terminal)
	}
}

func wardrobeRepairCause(err error) error {
	var visit func(error, int) error
	visit = func(cause error, depth int) error {
		if cause == nil || depth > 12 {
			return nil
		}
		if many, ok := cause.(interface{ Unwrap() []error }); ok {
			children := many.Unwrap()
			for index := len(children) - 1; index >= 0; index-- {
				if found := visit(children[index], depth+1); found != nil {
					return found
				}
			}
		}
		if strings.HasPrefix(cause.Error(), "adk_final_repair_invalid:") || strings.HasPrefix(cause.Error(), "adk_final_repair_failed:") {
			if nested := errors.Unwrap(cause); nested != nil {
				return nested
			}
		}
		return visit(errors.Unwrap(cause), depth+1)
	}
	if repaired := visit(err, 0); repaired != nil {
		return repaired
	}
	return err
}
