package core

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// Legacy Reflection tests script only the Reflection response. A new accepted
// stable overlay now invokes its separate compiler before publication. This
// adapter supplies a labeled, controlled compiler response while preserving
// the existing Reflection Provider script and call-count assertions.
func withControlledPersonaCompilation(inner http.RoundTripper) http.RoundTripper {
	return projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		request.Body = io.NopCloser(strings.NewReader(string(body)))
		payload := decodeObject(body)
		if providerWireSchemaName(payload) != "persona_compilation_response" {
			return inner.RoundTrip(request)
		}
		profileID := ""
		for _, raw := range arrayValue(payload["messages"]) {
			message := mapValue(raw)
			if stringValue(message["role"]) != "user" {
				continue
			}
			for _, line := range strings.Split(stringValue(message["content"]), "\n") {
				if strings.HasPrefix(line, "profile_id:") {
					profileID = strings.TrimSpace(strings.TrimPrefix(line, "profile_id:"))
					break
				}
			}
		}
		if profileID == "" {
			return embeddingHTTPResponse(request, http.StatusBadRequest, `{"error":"fixture_profile_missing"}`), nil
		}
		result := map[string]any{"portrait_text": "保留" + profileID + "人格的稳定机制"}
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": jsonString(result)}}}}
		return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(response))), nil
	})
}

// Older integration fixtures insert accepted foundation rows directly instead
// of going through CreateFluctlight. Keep their persona assertions meaningful
// by explicitly seeding a source-matched derived row in the test database.
// Production has no equivalent missing-portrait fallback.
func seedLegacyTestWorkingPersonas(t *testing.T, app *App) {
	t.Helper()
	if app == nil || app.DB == nil || app.DB.Pool() == nil {
		return
	}
	ctx := context.Background()
	rows, err := app.DB.Pool().Query(ctx, `SELECT id,current_revision,initialization_mode,core_persona FROM public.fluctlights`)
	if err != nil {
		return
	}
	type sourceRow struct {
		id       string
		revision int
		mode     string
		raw      []byte
	}
	sources := make([]sourceRow, 0)
	for rows.Next() {
		var row sourceRow
		if err := rows.Scan(&row.id, &row.revision, &row.mode, &row.raw); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		sources = append(sources, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatal(err)
	}
	rows.Close()
	for _, row := range sources {
		core := decodeObject(row.raw)
		for _, profileID := range declaredPersonaProfileIDs(core) {
			habits := arrayValue(mapValue(core["life_profile"])["life_habits"])
			if habits == nil {
				habits = []any{}
			}
			if _, err := app.DB.Pool().Exec(ctx, `INSERT INTO public.fluctlight_profile_habits(fluctlight_id,profile_id,revision,habits_json,source_kind,source_ref) VALUES($1,$2,0,$3,'initialization',$4) ON CONFLICT(fluctlight_id,profile_id) DO NOTHING`, row.id, profileID, jsonBytes(habits), "fixture:"+row.id); err != nil {
				t.Fatal(err)
			}
			input, err := app.personaCompilationInputForProfile(ctx, row.id, core, row.revision, profileID, true)
			if err != nil {
				continue
			}
			source, err := personaCompilationSource(input)
			if err != nil {
				continue
			}
			hash := stableDigest(jsonString(source))
			var savedHash string
			readErr := app.DB.Pool().QueryRow(ctx, `SELECT source_hash FROM public.fluctlight_working_personas WHERE fluctlight_id=$1 AND profile_id=$2`, row.id, profileID).Scan(&savedHash)
			if readErr == nil && savedHash == hash {
				continue
			}
			if readErr != nil && !errors.Is(readErr, pgx.ErrNoRows) {
				t.Fatal(readErr)
			}
			facts := []PersonaPortraitFact{{Category: "core_mechanisms", Text: jsonString(source["profile"]), SourceRefs: []string{"profile"}}}
			if identity := mapValue(source["identity"]); len(identity) > 0 {
				facts = append(facts, PersonaPortraitFact{Category: "identity", Text: jsonString(identity), SourceRefs: []string{"identity"}})
			}
			if prefs := mapValue(mapValue(source["life_profile"])["preferences"]); len(prefs) > 0 {
				facts = append(facts, PersonaPortraitFact{Category: "stable_preferences", Text: jsonString(prefs), SourceRefs: []string{"life_profile.preferences"}})
			}
			compiled := CompiledWorkingPersona{ProfileID: profileID, Facts: facts, SourceRevision: row.revision, SourceHash: hash, OverlayRevision: input.OverlayRevision, RulesVersion: personaCompilationRulesVersion, BudgetRunes: input.TargetBudgetRunes}
			if err := withTransaction(ctx, app.DB.Pool(), func(tx pgx.Tx) error {
				return insertCompiledWorkingPersonasTx(ctx, tx, row.id, []CompiledWorkingPersona{compiled})
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func systemPersonaForLegacyProjectionForTest(projection ContextProjection, schemaName string) map[string]any {
	working, _ := projectWorkingPersona(projection, "")
	return systemPersonaForProjectionWithWorking(projection, schemaName, working)
}
