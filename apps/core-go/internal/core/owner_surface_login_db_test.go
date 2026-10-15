package core

import (
	"testing"
)

func TestDisposableDatabaseOwnerLogin(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	app := newTestApp(t, repository, nil)
	token := "setup_live_owner_token_0001"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_setup_tokens(id, token_hash, expires_at) VALUES($1,$2,now()+interval '30 minutes')`, "setup_live_owner", digestToken(token)); err != nil {
		t.Fatal(err)
	}
	actorID, session, err := app.Setup(ctx, token, "long-enough-password")
	if err != nil || actorID == "" || session == "" {
		t.Fatalf("setup actor=%q session=%q err=%v", actorID, session, err)
	}
	again, next, err := app.Login(ctx, "long-enough-password")
	if err != nil || again != actorID || next == "" {
		t.Fatalf("login actor=%q session=%q err=%v", again, next, err)
	}
	resolved, err := app.ResolveSession(ctx, next)
	if err != nil || resolved != actorID {
		t.Fatalf("refresh session actor=%q err=%v", resolved, err)
	}
	if _, _, err := app.Login(ctx, "wrong-password"); err == nil {
		t.Fatal("wrong password was accepted")
	}
}
