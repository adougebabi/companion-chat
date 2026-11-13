package core

import "context"

// Raw diagnostic records can contain complete prompts, conversation history,
// tool arguments and provider responses. Owner authentication alone does not
// authorize reading them. The process operator must also enable debug reads.
func (a *App) requireDiagnosticDebugRead(ctx context.Context, owner string) error {
	if err := a.requireOwner(ctx, owner); err != nil {
		return err
	}
	if !a.DiagnosticDebugReadEnabled {
		return &SurfaceError{
			Status:  403,
			Code:    "diagnostic_debug_read_disabled",
			Message: "完整诊断读取未启用，请由服务管理员开启 FLUCTLIGHT_DIAGNOSTIC_DEBUG_READ 后重启 API",
		}
	}
	return nil
}

// Full-text filters inspect raw stored JSON before privacy-safe projection.
// Gate them even on metadata exports and clear receipts, which otherwise
// reveal private text through result counts.
func (a *App) requireDiagnosticSearchAccess(ctx context.Context, owner string, filter ownerDiagnosticFilter) error {
	if filter["q"] != "" || filter["qAny"] != "" {
		return a.requireDiagnosticDebugRead(ctx, owner)
	}
	return nil
}
