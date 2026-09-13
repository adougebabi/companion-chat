package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fakeRepository struct{}

type testPublicDetailsError struct {
	code    string
	details map[string]any
}

func (e testPublicDetailsError) Error() string                 { return e.code }
func (e testPublicDetailsError) PublicDetails() map[string]any { return e.details }

func (fakeRepository) Ping(_ context.Context) error { return nil }
func (fakeRepository) ResolveSession(_ context.Context, token string) (string, error) {
	if token == "session" {
		return "human-1", nil
	}
	return "", core.ErrUnauthorized
}

func TestReadJSONRejectsTrailingValues(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"ok":true}{"extra":true}`))
	if _, ok := readJSON(request); ok {
		t.Fatal("expected concatenated JSON to be rejected")
	}
}

func TestLifecycleDiagnosticsFilterMapsAllQueryFields(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/internal/diagnostics/lifecycle?limit=42&fluctlight_id=fl-1&correlation_id=corr-1&intent_id=intent-1&workflow_id=go%3Awake&run_id=run-1&surface=wake_up&status=retry", nil)
	filter := lifecycleDiagnosticsFilter(request, 100)
	if filter.Limit != 42 || filter.FluctlightID != "fl-1" || filter.CorrelationID != "corr-1" || filter.IntentID != "intent-1" || filter.WorkflowID != "go:wake" || filter.RunID != "run-1" || filter.Surface != "wake_up" || filter.Status != "retry" {
		t.Fatalf("lifecycle filter = %#v", filter)
	}
}

func TestLifecycleDiagnosticsExportKeepsItsLargerBoundedLimit(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/internal/diagnostics/export", nil)
	if got := lifecycleDiagnosticsFilter(request, 500).Limit; got != 500 {
		t.Fatalf("default export limit = %d, want 500", got)
	}
	request = httptest.NewRequest(http.MethodGet, "/internal/diagnostics/export?limit=900", nil)
	if got := lifecycleDiagnosticsFilter(request, 500).Limit; got != 500 {
		t.Fatalf("bounded export limit = %d, want 500", got)
	}
}

func TestDiagnosticsQueryFailureDetailsKeepOnlyBoundedCorrelation(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/internal/diagnostics/lifecycle?correlation_id=corr-1", nil)
	details := diagnosticsQueryFailureDetails(request, "lifecycle_query")
	if details["stage"] != "lifecycle_query" || details["correlation_id"] != "corr-1" {
		t.Fatalf("diagnostics failure details = %#v", details)
	}
	request = httptest.NewRequest(http.MethodGet, "/internal/diagnostics/lifecycle?correlation_id="+strings.Repeat("x", 129), nil)
	if _, exists := diagnosticsQueryFailureDetails(request, "lifecycle_query")["correlation_id"]; exists {
		t.Fatal("oversized correlation leaked into diagnostics failure details")
	}
}

func TestLifecycleDiagnosticsRouteAndExportUseOneFilterContract(t *testing.T) {
	serverSource, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(serverSource), `GET /internal/diagnostics/lifecycle`) {
		t.Fatal("Core lifecycle diagnostics route is not registered")
	}
	domainSource, err := os.ReadFile("domain.go")
	if err != nil {
		t.Fatal(err)
	}
	domain := string(domainSource)
	for _, required := range []string{"s.app.LifecycleDiagnostics", "s.app.DiagnosticsExportFiltered", "lifecycleDiagnosticsFilter(r,", "diagnostics_filter_invalid", "validation_error"} {
		if !strings.Contains(domain, required) {
			t.Fatalf("Core diagnostics handler missing %q", required)
		}
	}
}

func TestWriteErrorDetailsKeepsBoundedOperationReason(t *testing.T) {
	response := httptest.NewRecorder()
	writeErrorDetails(response, http.StatusConflict, "diagnostics_media_retry_failed", map[string]any{"reason": "workflow restart failed"})
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusConflict)
	}
	if !strings.Contains(response.Body.String(), `"reason":"workflow restart failed"`) {
		t.Fatalf("reason missing from error body: %s", response.Body.String())
	}
}

func TestProviderRoleErrorCodePreservesPreflightReason(t *testing.T) {
	cases := map[string]string{
		"provider_model_not_available":                                   "provider_model_not_available",
		"provider_models_unavailable: provider models returned HTTP 401": "provider_models_unavailable",
		"provider_endpoint_not_found":                                    "provider_endpoint_not_found",
		"provider_role_invalid":                                          "provider_role_invalid",
	}
	for message, want := range cases {
		if got := providerRoleErrorCode(errors.New(message)); got != want {
			t.Fatalf("providerRoleErrorCode(%q) = %q, want %q", message, got, want)
		}
	}
}

func TestActivationFailureDetailsSeparatePersonaConflictAndPersistence(t *testing.T) {
	personaCode, personaDetails := activationFailureDetails(testPublicDetailsError{
		code:    "initialization_persona_invalid",
		details: map[string]any{"validation_error": map[string]any{"type": "reference_invalid", "path": "initial_intentions[0].goal_index"}},
	}, "activation:fl-1")
	if personaCode != "activation_persona_invalid" || stringValue(personaDetails["correlation_id"]) != "activation:fl-1" || stringValue(mapValue(personaDetails["validation_error"])["path"]) != "initial_intentions[0].goal_index" {
		t.Fatalf("persona activation failure=%q %#v", personaCode, personaDetails)
	}
	conflictCode, _ := activationFailureDetails(core.ErrConflict, "activation:fl-1")
	persistenceCode, persistenceDetails := activationFailureDetails(errors.New("database unavailable"), "activation:fl-1")
	if conflictCode != "activation_request_conflict" || persistenceCode != "activation_persistence_failed" || stringValue(persistenceDetails["correlation_id"]) != "activation:fl-1" {
		t.Fatalf("activation failure codes conflict=%q persistence=%q details=%#v", conflictCode, persistenceCode, persistenceDetails)
	}
}

func TestActivationFailureDetailsPreserveWrappedPersonaAndBoundLogCode(t *testing.T) {
	wrapped := fmt.Errorf("activation validation: %w", testPublicDetailsError{
		code:    "initialization_persona_invalid",
		details: map[string]any{"validation_error": map[string]any{"type": "reference_invalid", "path": "initial_intentions[0].goal_index"}},
	})
	code, details := activationFailureDetails(wrapped, "activation:fl-1")
	if code != "activation_persona_invalid" || stringValue(mapValue(details["validation_error"])["path"]) != "initial_intentions[0].goal_index" {
		t.Fatalf("wrapped persona activation failure=%q %#v", code, details)
	}
	if got := activationFailureLogCode(fmt.Errorf("agency: %w", errors.New("initial_goal_profile_invalid"))); got != "initial_goal_profile_invalid" {
		t.Fatalf("wrapped activation log code=%q", got)
	}
	if got := activationFailureLogCode(errors.New("database rejected password=secret")); got != "unclassified" {
		t.Fatalf("unsafe activation log code=%q", got)
	}
}

func TestActivationAnalysisFailureDetailsAndStatusesRemainDistinct(t *testing.T) {
	cases := []struct {
		err        error
		wantCode   string
		wantStatus int
	}{
		{core.ErrActivationAnalysisRequired, "activation_analysis_required", http.StatusUnprocessableEntity},
		{core.ErrActivationAnalysisInvalid, "activation_analysis_invalid", http.StatusUnprocessableEntity},
		{core.ErrActivationAnalysisStale, "activation_analysis_stale", http.StatusConflict},
		{core.ErrActivationAnalysisConflict, "activation_analysis_conflict", http.StatusConflict},
		{core.ErrConflict, "activation_request_conflict", http.StatusConflict},
		{errors.New("database unavailable"), "activation_persistence_failed", http.StatusInternalServerError},
	}
	for _, item := range cases {
		code, details := activationFailureDetails(item.err, "activation:fl-1")
		if code != item.wantCode || activationFailureStatus(item.err) != item.wantStatus || stringValue(details["correlation_id"]) != "activation:fl-1" {
			t.Fatalf("activation failure err=%v code=%q status=%d details=%#v", item.err, code, activationFailureStatus(item.err), details)
		}
	}
}

func TestInitializationAnalysisFailureStatusDistinguishesRuntimeAndSemanticErrors(t *testing.T) {
	cases := []struct {
		validationType string
		want           int
	}{
		{"provider", http.StatusServiceUnavailable},
		{"persistence", http.StatusInternalServerError},
		{"semantic_invalid", http.StatusUnprocessableEntity},
		{"semantic_empty", http.StatusUnprocessableEntity},
	}
	for _, item := range cases {
		err := testPublicDetailsError{code: "initialization_failed", details: map[string]any{"validation_error": map[string]any{"type": item.validationType, "path": "initialization"}}}
		if got := initializationAnalysisFailureStatus(err); got != item.want {
			t.Fatalf("initializationAnalysisFailureStatus(%q) = %d, want %d", item.validationType, got, item.want)
		}
	}
}

func TestInitializationDescriptionLimitUsesUTF8Bytes(t *testing.T) {
	for _, item := range []struct {
		name        string
		description string
		want        bool
	}{
		{"ascii_at_limit", strings.Repeat("a", core.InitializationDescriptionMaxBytes), true},
		{"ascii_over_limit", strings.Repeat("a", core.InitializationDescriptionMaxBytes+1), false},
		{"multibyte_at_limit", strings.Repeat("你", core.InitializationDescriptionMaxBytes/3), true},
		{"multibyte_over_limit", strings.Repeat("你", core.InitializationDescriptionMaxBytes/3+1), false},
		{"blank", " \n\t ", false},
	} {
		t.Run(item.name, func(t *testing.T) {
			if got := validInitializationDescriptionRequest(item.description); got != item.want {
				t.Fatalf("validInitializationDescriptionRequest bytes=%d = %v, want %v", len([]byte(item.description)), got, item.want)
			}
		})
	}
	bffSource, err := os.ReadFile("../../../gateway-go/internal/bff/routes.go")
	if err != nil {
		t.Fatal(err)
	}
	webSource, err := os.ReadFile("../../../web/src/views/InstancesView.vue")
	if err != nil {
		t.Fatal(err)
	}
	bff := string(bffSource)
	if !strings.Contains(bff, "const initializationDescriptionMaxBytes = 60000") || !strings.Contains(bff, "len(text) <= initializationDescriptionMaxBytes") || !strings.Contains(bff, "initialization_description_too_large") || !strings.Contains(bff, "http.StatusRequestEntityTooLarge") {
		t.Fatal("BFF does not enforce the shared UTF-8 byte limit")
	}
	if !strings.Contains(bff, `response.Header().Set("Cache-Control", "no-store, private")`) {
		t.Fatal("BFF Fluctlight detail does not disable caching for Owner-only source text")
	}
	if !strings.Contains(string(webSource), "new TextEncoder().encode(description).byteLength") || strings.Contains(string(webSource), `maxlength="12000"`) {
		t.Fatal("Web does not enforce the shared UTF-8 byte limit")
	}
}

func TestInitializationCreationContractBindsExplicitLatestAnalysis(t *testing.T) {
	appSource, err := os.ReadFile("../core/app.go")
	if err != nil {
		t.Fatal(err)
	}
	serverSource, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatal(err)
	}
	app := string(appSource)
	server := string(serverSource)
	for _, required := range []string{
		`AnalyzeDescription(ctx context.Context, actorID, description string)`,
		`response["analysis_id"] = analysisID`,
		`response["correlation_id"] = correlationID`,
		`WHERE human_actor_id=$1 FOR UPDATE`,
		`WHERE id=$1 FOR UPDATE`,
		`ORDER BY created_at DESC,id DESC LIMIT 1 FOR UPDATE`,
		`latestSourceID != sourceID`,
		`return ErrActivationAnalysisStale`,
		`analysisStartedAt := time.Now().UTC()`,
		`safeInitializationErrorCode(code)`,
	} {
		if !strings.Contains(app, required) {
			t.Fatalf("Core initialization authority contract missing %q", required)
		}
	}
	for _, required := range []string{
		`AnalyzeDescription(request.Context(), actorID, description)`,
		`analysisID := stringValue(body["analysis_id"])`,
		`mode == "llm_defined" && analysisID == ""`,
	} {
		if !strings.Contains(server, required) {
			t.Fatalf("Core HTTP initialization authority contract missing %q", required)
		}
	}
}

func TestPostgresInitializationAnalysisAuthoritySerializesLatestSourceAndReplay(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("GO_CORE_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is required for the S12 disposable PostgreSQL gate")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	adminConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	adminConfig.ConnConfig.Database = "postgres"
	adminPool, err := pgxpool.NewWithConfig(ctx, adminConfig)
	if err != nil {
		t.Fatal(err)
	}
	databaseName := fmt.Sprintf("lac_s11_analysis_%d", time.Now().UnixNano())
	if !regexp.MustCompile(`^[a-z0-9_]+$`).MatchString(databaseName) {
		adminPool.Close()
		t.Fatalf("unsafe temporary database name %q", databaseName)
	}
	databaseIdentifier := pgx.Identifier{databaseName}.Sanitize()
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+databaseIdentifier); err != nil {
		adminPool.Close()
		t.Fatal(err)
	}
	var repository *core.PostgresRepository
	var testPool *pgxpool.Pool
	t.Cleanup(func() {
		if repository != nil {
			repository.Close()
		}
		if testPool != nil {
			testPool.Close()
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if _, dropErr := adminPool.Exec(cleanupCtx, "DROP DATABASE "+databaseIdentifier); dropErr != nil {
			t.Errorf("drop isolated PostgreSQL database %s: %v", databaseName, dropErr)
		}
		adminPool.Close()
	})
	testConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	testConfig.ConnConfig.Database = databaseName
	testPool, err = pgxpool.NewWithConfig(ctx, testConfig)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrations.New(testPool).Apply(ctx); err != nil {
		testPool.Close()
		t.Fatal(err)
	}
	testPool.Close()
	testPool = nil
	testDatabaseURL, err := url.Parse(databaseURL)
	if err != nil || testDatabaseURL.Scheme == "" {
		t.Fatalf("GO_CORE_TEST_DATABASE_URL must be a PostgreSQL URL: %v", err)
	}
	testDatabaseURL.Path = "/" + databaseName
	repository, err = core.NewPostgresRepository(ctx, testDatabaseURL.String())
	if err != nil {
		t.Fatal(err)
	}

	const ownerActorID = "human-s11-owner"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type) VALUES($1,'human')`, ownerActorID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'test-only','credential-s11')`, ownerActorID); err != nil {
		t.Fatal(err)
	}
	olderSourceID := "initialization_source_" + strings.Repeat("a", 32)
	latestSourceID := "initialization_source_" + strings.Repeat("b", 32)
	startedAt := time.Now().UTC().Add(-time.Minute)
	for index, sourceID := range []string{olderSourceID, latestSourceID} {
		correlationID := fmt.Sprintf("initialization-analysis:s11-%d", index)
		createdAt := startedAt.Add(time.Duration(index) * time.Second)
		if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_initialization_sources(id,owner_actor_id,correlation_id,source_text,source_digest,prompt_version,schema_version,classification,classification_evidence,field_derivations,coverage,structured_projection,projection_digest,created_at) VALUES($1,$2,$3,'private test card',$4,'initialization-prompt.v1','initialization.v2','single','{}','{}','{"extracted_semantic_units":1}','{"schema_version":2}',$5,$6)`, sourceID, ownerActorID, correlationID, strings.Repeat(fmt.Sprintf("%d", index+1), 32), strings.Repeat(fmt.Sprintf("%d", index+3), 32), createdAt); err != nil {
			t.Fatal(err)
		}
	}
	app := &core.App{DB: repository}
	if _, err := app.CreateFluctlight(ctx, ownerActorID, "fluctlight-s11-stale", "S11", "llm_defined", olderSourceID, s11InitializationFoundation("S11"), nil, nil); !errors.Is(err, core.ErrActivationAnalysisStale) {
		t.Fatalf("older analysis activation error = %v, want activation_analysis_stale", err)
	}

	const concurrentFluctlightID = "fluctlight-s11-concurrent"
	results := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	start := make(chan struct{})
	for range 2 {
		go func() {
			ready.Done()
			<-start
			_, createErr := app.CreateFluctlight(ctx, ownerActorID, concurrentFluctlightID, "S11", "llm_defined", latestSourceID, s11InitializationFoundation("S11"), nil, nil)
			results <- createErr
		}()
	}
	ready.Wait()
	close(start)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatalf("concurrent idempotent activation failed: %v", err)
		}
	}
	var linkCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_initialization_source_links WHERE source_id=$1 AND fluctlight_id=$2`, latestSourceID, concurrentFluctlightID).Scan(&linkCount); err != nil || linkCount != 1 {
		t.Fatalf("initialization source link count=%d err=%v", linkCount, err)
	}
	if _, err := app.CreateFluctlight(ctx, ownerActorID, "fluctlight-s11-blank", "Blank A", "blank_slate", "", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := app.CreateFluctlight(ctx, ownerActorID, "fluctlight-s11-blank", "Blank B", "blank_slate", "", nil, nil, nil); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("changed blank-slate replay error=%v, want conflict", err)
	}
}

func s11InitializationFoundation(name string) map[string]any {
	return map[string]any{
		"schema_version":        2,
		"core_persona":          map[string]any{"identity": map[string]any{"name": name}},
		"developing_self":       map[string]any{"claims": []any{}},
		"extensions":            map[string]any{},
		"initial_relationships": []any{},
		"initial_goals":         []any{},
		"initial_intentions":    []any{},
	}
}

func (fakeRepository) ListFluctlights(_ context.Context, _ string) ([]core.Fluctlight, error) {
	return []core.Fluctlight{{ID: "fl-1", Status: "active"}}, nil
}
func (fakeRepository) GetFluctlight(_ context.Context, _, _ string) (core.Fluctlight, error) {
	return core.Fluctlight{ID: "fl-1", Status: "active"}, nil
}
func (fakeRepository) DirectConversationID(_ context.Context, _, _ string) (string, error) {
	return "conversation-1", nil
}
func (fakeRepository) History(_ context.Context, _, _ string, _ *int, _ int) (core.ConversationPage, error) {
	return core.ConversationPage{}, nil
}

func TestServerProtectsCoreRoutesAndReturnsSnakeCase(t *testing.T) {
	server := New(fakeRepository{}, "service-key", nil)
	request := httptest.NewRequest(http.MethodGet, "/internal/fluctlights", nil)
	request.Header.Set(serviceKeyHeader, "service-key")
	request.Header.Set(humanSessionHeader, "session")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	request = httptest.NewRequest(http.MethodGet, "/internal/platform/ping", nil)
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("missing service key status = %d, want 401", response.Code)
	}
}
