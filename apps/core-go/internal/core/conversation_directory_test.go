package core

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestDirectSessionDirectoryPersistsSelectionScopesSearchAndReadPositions(t *testing.T) {
	ctx, repo := isolatedCoreTestRepository(t)
	seedLifeContextFluctlight(t, ctx, repo, "owner-a", "persona-a")
	seedLifeContextFluctlight(t, ctx, repo, "owner-b", "persona-b")
	// A second persona belongs to the same owner; it must still have separate history.
	seedLifeContextFluctlight(t, ctx, repo, "owner-c", "persona-c")
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.fluctlights SET created_by_actor_id='owner-a' WHERE id='persona-c'`); err != nil {
		t.Fatal(err)
	}
	app := newTestApp(t, repo, nil)
	original, err := app.EnsureDirectConversation(ctx, "owner-a", "persona-a")
	if err != nil {
		t.Fatal(err)
	}
	var before string
	if err := repo.Pool().QueryRow(ctx, `SELECT row_to_json(f)::text || row_to_json(s)::text FROM public.fluctlights f JOIN public.fluctlight_inner_states s ON s.fluctlight_id=f.id WHERE f.id='persona-a'`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	sessions := []string{original}
	for i := 0; i < 4; i++ {
		page, err := app.NewDirectConversation(ctx, "owner-a", "persona-a", fmt.Sprintf("Session %d", i), fmt.Sprintf("session-%d", i))
		if err != nil {
			t.Fatal(err)
		}
		sessions = append(sessions, page.Conversation.ID)
	}
	if _, err := app.NewDirectConversation(ctx, "owner-a", "persona-a", "changed", "session-0"); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflicting command = %v", err)
	}
	if _, err := app.SelectDirectConversation(ctx, "owner-a", "persona-a", sessions[1]); err != nil {
		t.Fatal(err)
	}
	restart := newTestApp(t, repo, nil)
	selected, err := restart.EnsureDirectConversation(ctx, "owner-a", "persona-a")
	if err != nil || selected != sessions[1] {
		t.Fatalf("restart selected %s %v", selected, err)
	}
	replay, err := restart.NewDirectConversation(ctx, "owner-a", "persona-a", "Session 0", "session-0")
	if err != nil || replay.Conversation.ID != sessions[1] {
		t.Fatalf("replay %#v %v", replay, err)
	}
	if _, err := restart.SelectDirectConversation(ctx, "owner-a", "persona-a", original); err != nil {
		t.Fatal(err)
	}
	if _, err := restart.NewDirectConversation(ctx, "owner-a", "persona-a", "Session 0", "session-0"); err != nil {
		t.Fatal(err)
	}
	selected, _ = repo.DirectConversationID(ctx, "owner-a", "persona-a")
	if selected != original {
		t.Fatal("old command replay changed newer selection")
	}
	for _, scope := range [][2]string{{"owner-b", "persona-b"}, {"owner-a", "persona-c"}} {
		if _, err := app.SelectDirectConversation(ctx, scope[0], scope[1], original); !errors.Is(err, ErrNotFound) {
			t.Fatalf("cross scope select %v", err)
		}
	}
	if _, err := app.ConversationDirectory(ctx, "owner-b", "persona-a", 2, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross owner list %v", err)
	}
	seen := map[string]bool{}
	cursor := ""
	for {
		page, err := app.ConversationDirectory(ctx, "owner-a", "persona-a", 2, cursor)
		if err != nil {
			t.Fatal(err)
		}
		if page.CurrentConversationID != original {
			t.Fatal("current pointer missing")
		}
		for _, item := range page.Items {
			if seen[item.Conversation.ID] {
				t.Fatal("directory duplicate")
			}
			seen[item.Conversation.ID] = true
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if len(seen) != 5 {
		t.Fatalf("directory lost sessions: %d", len(seen))
	}
	if _, err := app.ConversationDirectory(ctx, "owner-a", "persona-c", 2, cursor); !errors.Is(err, ErrConversationInvalid) {
		t.Fatalf("foreign cursor: %v", err)
	}
	// Acceptance goes through the real durable user-turn command; no Provider runs.
	acceptedIDs := map[string]bool{}
	for i := 0; i < 13; i++ {
		cid := sessions[i%len(sessions)]
		result, err := app.AcceptTurn(ctx, "owner-a", cid, map[string]any{"fluctlight_id": "persona-a", "text": fmt.Sprintf("needle historical %02d", i), "idempotency_key": fmt.Sprintf("history-%d", i), "turn_id": fmt.Sprintf("history-turn-%d", i), "attachment_refs": []any{}})
		if err != nil {
			t.Fatal(err)
		}
		acceptedIDs[stringValue(result.UserMessage["id"])] = true
	}
	historyIDs := map[string]bool{}
	for _, cid := range sessions {
		var beforeSequence *int
		for {
			page, err := repo.History(ctx, cid, "owner-a", beforeSequence, 1)
			if err != nil {
				t.Fatal(err)
			}
			for _, message := range page.Messages {
				if historyIDs[message.ID] {
					t.Fatal("history repeated a message")
				}
				historyIDs[message.ID] = true
			}
			if page.NextBefore == nil {
				break
			}
			beforeSequence = page.NextBefore
		}
	}
	if len(historyIDs) != len(acceptedIDs) {
		t.Fatalf("history pages lost messages %d", len(historyIDs))
	}
	found := map[string]bool{}
	cursor = ""
	for {
		page, err := app.SearchConversationMessages(ctx, "owner-a", "persona-a", "needle", 3, cursor)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range page.Items {
			if found[m.ID] || !acceptedIDs[m.ID] {
				t.Fatalf("invalid search identity %#v", m)
			}
			found[m.ID] = true
			exact, err := app.ConversationMessage(ctx, "owner-a", m.ConversationID, m.ID)
			if err != nil || exact.Text != m.Text || exact.Sequence != m.Sequence {
				t.Fatalf("source mismatch %#v %v", exact, err)
			}
			if _, err := app.ConversationMessage(ctx, "owner-b", m.ConversationID, m.ID); !errors.Is(err, ErrNotFound) {
				t.Fatal("foreign source visible")
			}
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if len(found) != 13 {
		t.Fatalf("search lost messages %d", len(found))
	}
	other, err := app.SearchConversationMessages(ctx, "owner-a", "persona-c", "needle", 3, "")
	if err != nil || len(other.Items) != 0 {
		t.Fatalf("other persona search %#v %v", other, err)
	}
	if _, err := app.SearchConversationMessages(ctx, "owner-a", "persona-a", "different", 3, cursor); !errors.Is(err, ErrConversationInvalid) {
		t.Fatalf("reused query cursor %v", err)
	}
	// Incoming messages are fixed initial history fixtures; the read command itself
	// and all counts/selection/reloads above and below use production services.
	for i := 0; i < 3; i++ {
		if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES($1,$2,$3,'persona-a','assistant','initial incoming fixture','[]',$1)`, fmt.Sprintf("incoming-%d", i), original, 100+i); err != nil {
			t.Fatal(err)
		}
	}
	checkUnread := func(want int) {
		t.Helper()
		p, err := app.ConversationDirectory(ctx, "owner-a", "persona-a", 50, "")
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range p.Items {
			if item.Conversation.ID == original && item.UnreadCount != want {
				t.Fatalf("unread %d want %d", item.UnreadCount, want)
			}
		}
		list, err := repo.ListFluctlights(ctx, "owner-a")
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range list {
			if f.ID == "persona-a" && f.UnreadCount != want {
				t.Fatalf("persona unread %d want %d", f.UnreadCount, want)
			}
		}
	}
	checkUnread(3)
	for i := 0; i < 2; i++ {
		if err := app.MarkRead(ctx, "owner-a", original, map[string]any{"read_sequence": 101}); err != nil {
			t.Fatal(err)
		}
	}
	checkUnread(1)
	if err := app.MarkRead(ctx, "owner-a", original, map[string]any{"read_sequence": 10000}); err == nil {
		t.Fatal("future cursor accepted")
	}
	checkUnread(1)
	if err := app.MarkRead(ctx, "owner-b", original, map[string]any{"read_sequence": 101}); err == nil {
		t.Fatal("foreign read accepted")
	}
	var after string
	if err := repo.Pool().QueryRow(ctx, `SELECT row_to_json(f)::text || row_to_json(s)::text FROM public.fluctlights f JOIN public.fluctlight_inner_states s ON s.fluctlight_id=f.id WHERE f.id='persona-a'`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("session operations changed long-lived persona or inner state")
	}
}

func TestEnsureDirectConversationConcurrentFirstUse(t *testing.T) {
	ctx, repo := isolatedCoreTestRepository(t)
	seedLifeContextFluctlight(t, ctx, repo, "concurrent-owner", "concurrent-persona")
	app := newTestApp(t, repo, nil)
	ids := make(chan string, 6)
	errs := make(chan error, 6)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := app.EnsureDirectConversation(ctx, "concurrent-owner", "concurrent-persona")
			ids <- id
			errs <- err
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	first := ""
	for id := range ids {
		if first == "" {
			first = id
		}
		if id != first {
			t.Fatal("concurrent legacy call created another session")
		}
	}
}
