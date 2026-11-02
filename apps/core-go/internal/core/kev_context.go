package core

import (
	"context"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
)

// Optional selection must finish well before the observed 30s KEV gateway
// timeout, even when saved request/stage settings allow 60s or more. This is a
// wall-clock bound for the whole selection, including settings, queue and audit.
const kevContextSelectionWait = 250 * time.Millisecond

type kevContextSelectionResult struct {
	input WorkingMemoryInput
	err   error
}

type kevContextSelection struct {
	ctx      context.Context
	cancel   context.CancelFunc
	original WorkingMemoryInput
	ready    chan kevContextSelectionResult
	adopted  chan bool
	finished chan struct{}
}

func (a *App) startKevContextSelection(ctx context.Context, projection ContextProjection, surface ProviderContextSurface, currentInput string, input WorkingMemoryInput) *kevContextSelection {
	child, cancel := context.WithTimeout(ctx, kevContextSelectionWait)
	selection := &kevContextSelection{
		ctx: child, cancel: cancel, original: input,
		ready: make(chan kevContextSelectionResult), adopted: make(chan bool, 1), finished: make(chan struct{}),
	}
	// The worker can outlive prompt assembly while its audit write completes.
	// Freeze its data so a late response cannot read or alter the live prompt.
	snapshot := input
	for _, section := range []*[]PromptFragment{&snapshot.ActiveCandidates, &snapshot.ResidentCandidates, &snapshot.RetrievedMemories, &snapshot.Summaries, &snapshot.RuntimeFacts, &snapshot.RecentMessages} {
		if *section == nil {
			continue
		}
		fragments := append([]PromptFragment{}, (*section)...)
		for i := range fragments {
			fragments[i].Content = copyPromptData(fragments[i].Content)
			fragments[i].SourceRefs = append([]string(nil), fragments[i].SourceRefs...)
		}
		*section = fragments
	}
	scope := a.kevScope(ctx, projection, string(surface))
	go func() {
		defer close(selection.finished)
		defer cancel()
		selected, pending, err := a.filterKevContext(decision.WithStageBudget(child, kevContextSelectionWait), scope, currentInput, snapshot)
		accepted := false
		select {
		case selection.ready <- kevContextSelectionResult{selected, err}:
			accepted = <-selection.adopted
		case <-child.Done():
		}
		// Raw answers were durably recorded before Admit. Only the consumer's
		// handoff can mark them applied; discarded late/partial selections keep
		// the entire original context and must never claim application.
		for i := range pending {
			status, reason := "applied", ""
			if !accepted {
				status, reason = "original_used", "context_selection_unavailable"
				if ctx.Err() != nil {
					reason = "request_cancelled"
				} else if child.Err() != nil {
					reason = "context_selection_timeout"
				}
			} else if pending[i].Record.Outcome == decision.Original || pending[i].Record.ApplicationStatus == "original_used" {
				status = "original_used"
			}
			_ = pending[i].Finish(child, status, reason, "")
		}
	}()
	return selection
}

func (s *kevContextSelection) wait(ctx context.Context) (WorkingMemoryInput, error) {
	select {
	case result := <-s.ready:
		accepted := result.err == nil && s.ctx.Err() == nil && ctx.Err() == nil
		s.adopted <- accepted
		if !accepted {
			return s.original, ctx.Err()
		}
		// Fast audits settle before returning. Slow final diagnostic writes do
		// not extend the wait or revoke an already accepted selection.
		select {
		case <-s.finished:
		case <-s.ctx.Done():
		case <-ctx.Done():
		}
		return result.input, ctx.Err()
	case <-s.ctx.Done():
		return s.original, ctx.Err()
	case <-ctx.Done():
		return s.original, ctx.Err()
	}
}

func (a *App) selectKevContext(ctx context.Context, projection ContextProjection, surface ProviderContextSurface, currentInput string, input WorkingMemoryInput) (WorkingMemoryInput, error) {
	selection := a.startKevContextSelection(ctx, projection, surface, currentInput, input)
	defer selection.cancel()
	return selection.wait(ctx)
}
