package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// This wrapper is applied only to known reads before the preparation model call.
// Stage names are fixed internal labels; raw database causes stay in the error chain.
type wardrobePreparationContextReadError struct {
	stage string
	cause error
}

func (e *wardrobePreparationContextReadError) Error() string {
	return fmt.Sprintf("wardrobe preparation context %s: %v", e.stage, e.cause)
}

func (e *wardrobePreparationContextReadError) Unwrap() error { return e.cause }

func wardrobeContextReadFailure(stage string, cause error) error {
	return &wardrobePreparationContextReadError{stage: stage, cause: cause}
}

func wardrobeContextReadStep(stage string) string {
	switch stage {
	case "resource":
		return "角色资料"
	case "effective_profile":
		return "当前人格"
	case "persona_detail":
		return "人格详情"
	case "life_snapshot":
		return "生活状态"
	case "wardrobe_items":
		return "衣物目录"
	case "wardrobe_revision":
		return "衣柜版本"
	case "wardrobe_plans":
		return "穿搭方案"
	default:
		return "角色资料或状态"
	}
}

func wardrobePreparationFailure(err error) (string, string) {
	var truncated *structuredResponseTruncatedError
	var contextRead *wardrobePreparationContextReadError
	var unassigned *providerRoleUnassignedError
	var roleRead *providerRoleLookupReadError
	terminal := wardrobeRepairCause(err)
	switch {
	case errors.Is(err, context.Canceled):
		return "wardrobe_preparation_cancelled", "本次初始衣柜准备已中断，原资料和已完成步骤仍保留。"
	case errors.As(err, &contextRead):
		return "wardrobe_preparation_context_failed", "初始衣柜准备在调用模型前读取" + wardrobeContextReadStep(contextRead.stage) + "失败，原资料和已完成步骤仍保留。请查看原任务中的诊断记录，确认资料或状态可读取后恢复。"
	case errors.As(err, &unassigned):
		purpose := "模型用途"
		if unassigned.role == "generic_llm" {
			purpose = "通用语言模型用途"
		}
		return "wardrobe_provider_unconfigured", "初始衣柜准备所需的" + purpose + "尚未分配可用服务和模型。请在管理中心的模型用途中完成对应配置后恢复。"
	case errors.As(err, &roleRead):
		return "wardrobe_provider_assignment_unavailable", "初始衣柜准备在读取模型用途配置时失败，尚未确认发出模型请求。原资料会保留，请确认工作区配置可读取后重试此环节。"
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
		return "wardrobe_preparation_failed", "初始衣柜准备未完成，原资料和已完成步骤仍保留。请查看原任务中的诊断记录，确认原因后恢复。"
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
