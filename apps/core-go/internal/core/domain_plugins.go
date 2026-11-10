package core

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
)

const domainPluginsSettingKey = "plugins.domains"

// These declarations group existing domain services; they do not create a
// second runtime or give a Tool authority to publish outside Core.
type domainPluginDeclaration struct {
	ID, Version          string
	Capabilities         []string
	NewWorkCapabilities  []string
	Agents               []string
	Settings             []string
	Events               []string
	PersistentTasks      []string
	OptionalDependencies []string
	Register             func(*App, *ToolPublicationService) []Capability
}

var builtInDomainPlugins = []domainPluginDeclaration{
	{ID: "image", Version: "1", Capabilities: []string{"media.image.generate", replyImageRequestName, "visual_identity.initialize", visualIdentityGenerateCandidateCapabilityName, visualIdentityCommitReviewCapabilityName, visualIdentityFinalizeCapabilityName}, NewWorkCapabilities: []string{"media.image.generate", replyImageRequestName, "visual_identity.initialize", visualIdentityGenerateCandidateCapabilityName}, Agents: []string{"media_prompt", "media_quality", "visual_identity", "visual_identity_vision", "visual_identity_patch"}, Settings: []string{domainPluginsSettingKey, "media.comfyui"}, Events: []string{"media.intent.created", "media.asset.ready"}, PersistentTasks: []string{"media.generation", "visual_identity.initialize", "studio.generate:image_preview"}, Register: imagePluginCapabilities},
	{ID: "moment", Version: "1", Capabilities: []string{"moment.publish", "moment.inspect"}, NewWorkCapabilities: []string{"moment.publish", "moment.inspect"}, Agents: []string{"conversation_reply:moment", "npc_response:npc_moment_response_v1"}, Settings: []string{domainPluginsSettingKey}, Events: []string{"moment.published", "moment.comment.created", "moment.reaction.created", npcMomentFactType}, PersistentTasks: []string{"cognition.processing:npc_moment"}, OptionalDependencies: []string{"image"}, Register: momentPluginCapabilities},
}

// Registration reuses the existing capability runtime and domain services.
// The private reply handoff is constructed by the Writer, not globally registered.
func imagePluginCapabilities(app *App, publication *ToolPublicationService) []Capability {
	var image imageCapabilityService
	var identity visualIdentityCapabilityService
	if app != nil {
		if app.Media != nil {
			image, identity = app.Media, app.Media
		} else {
			image, identity = app, app
		}
	}
	capabilities := []Capability{imageGenerateCapability{service: image, publication: publication}, visualIdentityInitializeCapability{service: identity}}
	return append(capabilities, visualIdentityAgentCapabilities(app)...)
}

func momentPluginCapabilities(app *App, publication *ToolPublicationService) []Capability {
	var repository *PostgresRepository
	if app != nil {
		repository = app.DB
	}
	return []Capability{momentPublishCapability{publication: publication}, momentInspectCapability{repository: repository}}
}

type domainPluginSettings struct {
	Image  bool `json:"image"`
	Moment bool `json:"moment"`
}

func defaultDomainPluginSettings() domainPluginSettings {
	return domainPluginSettings{Image: true, Moment: true}
}

func decodeDomainPluginSettings(raw []byte) (domainPluginSettings, error) {
	value, err := decodeReplyVisionSetting(raw, defaultDomainPluginSettings())
	object := decodeObject(raw)
	if err != nil || len(object) != 2 {
		return value, errors.New("domain_plugins_config_invalid")
	}
	if _, ok := object["image"].(bool); !ok {
		return value, errors.New("domain_plugins_config_invalid")
	}
	if _, ok := object["moment"].(bool); !ok {
		return value, errors.New("domain_plugins_config_invalid")
	}
	return value, nil
}

func (s domainPluginSettings) enabled(id string) bool {
	switch id {
	case "image":
		return s.Image
	case "moment":
		return s.Moment
	default:
		return false
	}
}

func domainPluginForCapability(name string) string {
	for _, plugin := range builtInDomainPlugins {
		if containsString(plugin.NewWorkCapabilities, name) {
			return plugin.ID
		}
	}
	return ""
}

type domainPluginScopeKey struct{}
type domainPluginVisualReplayKey struct{}

func domainPluginScope(ctx context.Context) (domainPluginSettings, bool) {
	settings, ok := ctx.Value(domainPluginScopeKey{}).(domainPluginSettings)
	return settings, ok
}

// A logical run keeps the plugin admission it received at start. Existing
// accepted media intents continue to reconcile after a later disable.
func (a *App) withDomainPluginScope(ctx context.Context) (context.Context, error) {
	if _, ok := domainPluginScope(ctx); ok {
		return ctx, nil
	}
	settings := defaultDomainPluginSettings()
	if a != nil && a.DB != nil && a.DB.Pool() != nil {
		raw, _, err := a.readOptionalRuntimeSetting(ctx, domainPluginsSettingKey)
		if err != nil {
			return ctx, err
		}
		if len(raw) > 0 {
			settings, err = decodeDomainPluginSettings(raw)
			if err != nil {
				return ctx, err
			}
		}
	}
	return context.WithValue(ctx, domainPluginScopeKey{}, settings), nil
}

func domainCapabilityEnabled(ctx context.Context, name string) bool {
	if name == visualIdentityGenerateCandidateCapabilityName {
		if replay, _ := ctx.Value(domainPluginVisualReplayKey{}).(bool); replay {
			return true
		}
	}
	plugin := domainPluginForCapability(name)
	if plugin == "" {
		return true
	}
	settings, ok := domainPluginScope(ctx)
	return !ok || settings.enabled(plugin)
}

func domainPluginDisabledError() error {
	return newCapabilityError("domain_plugin_disabled", false, &SurfaceError{Status: http.StatusConflict, Code: "domain_plugin_disabled", Message: "该能力已停用；已受理任务可继续收尾"})
}

func requireDomainPluginEnabledTx(ctx context.Context, tx pgx.Tx, id string) error {
	settings, scoped := domainPluginScope(ctx)
	if !scoped {
		settings = defaultDomainPluginSettings()
		var raw string
		err := tx.QueryRow(ctx, `SELECT value_json FROM public.runtime_settings WHERE key=$1`, domainPluginsSettingKey).Scan(&raw)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil {
			settings, err = decodeDomainPluginSettings([]byte(raw))
			if err != nil {
				return err
			}
		}
	}
	if !settings.enabled(id) {
		return domainPluginDisabledError()
	}
	return nil
}

func filterDomainPluginDefinitions(ctx context.Context, definitions []CapabilityDefinition) []CapabilityDefinition {
	out := make([]CapabilityDefinition, 0, len(definitions))
	for _, definition := range definitions {
		if domainCapabilityEnabled(ctx, definition.Name) {
			out = append(out, definition)
		}
	}
	return out
}

func domainPluginCatalog(settings domainPluginSettings) []map[string]any {
	out := make([]map[string]any, 0, len(builtInDomainPlugins))
	for _, plugin := range builtInDomainPlugins {
		inFlight := "current_process_runs_and_accepted_media_continue_unaccepted_recovery_checks_current_setting"
		if plugin.ID == "moment" {
			inFlight = "current_process_runs_continue_committed_publications_replay_unaccepted_recovery_checks_current_setting"
		}
		out = append(out, map[string]any{
			"id": plugin.ID, "version": plugin.Version, "installed": true,
			"enabled": settings.enabled(plugin.ID), "health": "not_checked",
			"required_dependencies": []string{}, "optional_dependencies": append([]string{}, plugin.OptionalDependencies...),
			"capabilities": plugin.Capabilities, "agents": plugin.Agents, "settings": plugin.Settings,
			"events": plugin.Events, "persistent_tasks": plugin.PersistentTasks,
			"in_flight_policy": inFlight,
		})
	}
	return out
}
