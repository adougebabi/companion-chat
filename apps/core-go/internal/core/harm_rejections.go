package core

import (
	"context"
	"net/url"
	"strings"
	"time"
)

// The transport composition supplies an actual typed domain rejection. No
// rejected content or browser-supplied author is accepted as a counter source.
func (a *App) ObserveOwnerHarmRejection(ctx context.Context, owner, method, path, key string) {
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if err := a.requireOwner(recordCtx, owner); err != nil {
		return
	}
	if parsed, err := url.Parse(path); err == nil {
		path = parsed.Path
	}
	parts := surfaceParts(path)
	fid := ""
	if len(parts) >= 2 && parts[0] == "fluctlights" {
		var valid bool
		if err := a.DB.Pool().QueryRow(recordCtx, `SELECT EXISTS(SELECT 1 FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2)`, parts[1], owner).Scan(&valid); err != nil {
			recordDiagnosticPersistenceFailure("harm_rejection", "scope", "", err)
			return
		}
		if valid {
			fid = parts[1]
		}
	}
	if strings.TrimSpace(key) == "" {
		key = randomID("request_")
	}
	id := "harm_rejection_" + stableDigest(owner+":"+method+":"+path+":"+key)
	_, err := a.DB.Pool().Exec(recordCtx, `INSERT INTO public.permanent_harm_rejections(id,owner_actor_id,fluctlight_id,method,route,boundary,reason_code) VALUES($1,$2,$3,$4,$5,'owner_browser_domain','permanent_harm_forbidden') ON CONFLICT(id) DO NOTHING`, id, owner, nilIfEmpty(fid), method, path)
	if err != nil {
		recordDiagnosticPersistenceFailure("harm_rejection", "record", id, err)
	}
}
