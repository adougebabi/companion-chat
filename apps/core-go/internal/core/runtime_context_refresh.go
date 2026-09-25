package core

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/cloudwego/eino/schema"
)

type runtimeContextRefreshPlanKey struct{}

type modelContextRefreshContent struct {
	System      string
	Runtime     string
	Projection  ContextProjection
	BudgetTrace PromptAssemblyTrace
}

// The plan is bound by the host surface. It rebuilds only Provider data, using
// the same authorization and memory scope as the first physical request.
func withRuntimeContextRefreshPlan(ctx context.Context, refresh func(context.Context) (modelContextRefreshContent, error)) context.Context {
	return context.WithValue(ctx, runtimeContextRefreshPlanKey{}, refresh)
}

func runtimeContextRefreshPlan(ctx context.Context) func(context.Context) (modelContextRefreshContent, error) {
	refresh, _ := ctx.Value(runtimeContextRefreshPlanKey{}).(func(context.Context) (modelContextRefreshContent, error))
	return refresh
}

type runtimeContextRefresh struct {
	mu      sync.Mutex
	target  *schema.Message
	system  *schema.Message
	dirty   bool
	latest  *modelContextRefreshContent
	refresh func(context.Context) (modelContextRefreshContent, error)
	base    ContextProjection
}

func (r *runtimeContextRefresh) latestProjection() *ContextProjection {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.latest == nil {
		return nil
	}
	projection := r.latest.Projection
	return &projection
}

func (r *runtimeContextRefresh) latestBudgetTrace() *PromptAssemblyTrace {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.latest == nil {
		return nil
	}
	trace := r.latest.BudgetTrace
	return &trace
}

func (r *runtimeContextRefresh) markDirty() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.dirty = true
	r.mu.Unlock()
}

func isRuntimeContextMessage(message *schema.Message) bool {
	return message != nil && message.Role == schema.User && len(message.UserInputMultiContent) == 0 &&
		strings.HasPrefix(message.Content, "[RUNTIME CONTEXT]\n") && strings.HasSuffix(message.Content, "\n[/RUNTIME CONTEXT]")
}

func modelContextContent(messages []map[string]any) (modelContextRefreshContent, error) {
	content := modelContextRefreshContent{}
	for _, message := range messages {
		candidate := stringValue(message["content"])
		if stringValue(message["role"]) == "system" {
			if content.System != "" {
				return modelContextRefreshContent{}, errors.New("system_context_duplicate")
			}
			content.System = candidate
		}
		if stringValue(message["role"]) != "user" || !strings.HasPrefix(candidate, "[RUNTIME CONTEXT]\n") || !strings.HasSuffix(candidate, "\n[/RUNTIME CONTEXT]") {
			continue
		}
		if content.Runtime != "" {
			return modelContextRefreshContent{}, errors.New("runtime_context_duplicate")
		}
		content.Runtime = candidate
	}
	if content.System == "" || content.Runtime == "" {
		return modelContextRefreshContent{}, errors.New("model_context_refresh_content_missing")
	}
	return content, nil
}

// prepare changes only the outbound copy. Eino owns the original pointers and
// the assistant ToolCall/tool-result history; neither may be edited to refresh
// a runtime fact after a committed Tool mutation.
func (r *runtimeContextRefresh) prepare(ctx context.Context, original []*schema.Message) ([]*schema.Message, error) {
	copyMessages := normalizeEinoToolMessageNames(original)
	if r == nil {
		return copyMessages, nil
	}
	r.mu.Lock()
	if r.target == nil {
		for _, message := range original {
			if message != nil && message.Role == schema.System {
				if r.system != nil {
					r.mu.Unlock()
					return nil, errors.New("system_context_duplicate")
				}
				r.system = message
			}
			if !isRuntimeContextMessage(message) {
				continue
			}
			if r.target != nil {
				r.mu.Unlock()
				return nil, errors.New("runtime_context_duplicate")
			}
			r.target = message
		}
	}
	target := r.target
	system := r.system
	dirty := r.dirty
	latest := r.latest
	refresh := r.refresh
	r.mu.Unlock()
	if !dirty && latest == nil {
		return copyMessages, nil
	}
	if target == nil || system == nil || (dirty && refresh == nil) {
		return nil, errors.New("runtime_context_refresh_unavailable")
	}
	index := -1
	systemIndex := -1
	for position, message := range original {
		if message == system {
			systemIndex = position
		}
		if message == target {
			index = position
		}
	}
	if index < 0 || systemIndex < 0 || !isRuntimeContextMessage(target) {
		return nil, errors.New("runtime_context_target_missing")
	}
	var content modelContextRefreshContent
	if dirty {
		refreshed, err := refresh(ctx)
		if err != nil {
			return nil, err
		}
		content = refreshed
		content.Projection.AuthorityAtRunStart = &CognitionAuthorityAtRunStart{
			Foundation: r.base.ContextRevision, CurrentState: r.base.CurrentStateRevision,
			CurrentFacts: r.base.CurrentFactsRevision, LifeContext: r.base.LifeContextRevision,
		}
		if content.Projection.FluctlightID != "" && r.base.ReferenceIndex.expectedScope() == content.Projection.ReferenceIndex.expectedScope() {
			for ref, entry := range r.base.ReferenceIndex.ByRef {
				if _, present := content.Projection.ReferenceIndex.ByRef[ref]; !present {
					content.Projection.ReferenceIndex.ByRef[ref] = entry
				}
			}
			if err := content.Projection.ReferenceIndex.Validate(); err != nil {
				return nil, err
			}
		}
	} else {
		content = *latest
	}
	if content.System == "" || !strings.HasPrefix(content.Runtime, "[RUNTIME CONTEXT]\n") || !strings.HasSuffix(content.Runtime, "\n[/RUNTIME CONTEXT]") {
		return nil, errors.New("runtime_context_refresh_invalid")
	}
	copyMessages[systemIndex].Content = content.System
	copyMessages[index].Content = content.Runtime
	if dirty {
		r.mu.Lock()
		r.latest = &content
		r.dirty = false
		r.mu.Unlock()
	}
	return copyMessages, nil
}
