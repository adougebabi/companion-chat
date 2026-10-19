package core

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// NPC encounter Tools receive only the domain operations needed to validate and
// settle a shared presence. The independent response stays bound to its formal
// Agent at composition, rather than exposing App or a generic model dispatcher.
type npcEncounterService struct {
	repository *PostgresRepository
	clock      func() time.Time
	validate   func(context.Context, pgx.Tx, map[string]any, time.Time) (map[string]any, error)
	commit     func(context.Context, pgx.Tx, npcEncounterCommit) (map[string]any, error)
	respond    func(context.Context, []map[string]any, map[string]any) (map[string]any, error)
}

type npcEncounterCommit struct {
	owner, fid, world, npc, actor, name, place, placeName, summary string
	refs                                                           []any
	start, end                                                     time.Time
	lifeRevision, key, reason                                      string
	attribution                                                    npcEncounterAttribution
}

func newNPCEncounterCapability(app *App, boundID string) npcEncounterCapability {
	capability := npcEncounterCapability{boundID: boundID}
	if app == nil {
		return capability
	}
	capability.service = &npcEncounterService{
		repository: app.DB,
		clock:      app.now,
		validate:   app.validateNPCOpportunityTx,
		commit: func(ctx context.Context, tx pgx.Tx, c npcEncounterCommit) (map[string]any, error) {
			return app.commitNPCEncounterTx(ctx, tx, c.owner, c.fid, c.world, c.npc, c.actor, c.name, c.place, c.placeName, c.summary, c.refs, c.start, c.end, c.lifeRevision, c.key, c.reason, c.attribution)
		},
		respond: func(ctx context.Context, messages []map[string]any, schema map[string]any) (map[string]any, error) {
			run, err := app.runFormalStructuredTask(ctx, FormalAgentNPCResponse, messages, nil, "npc_response_v1", schema, false, nil)
			return run.Completion.Structured, err
		},
	}
	return capability
}
