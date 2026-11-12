package core

import (
	"context"
	"net/url"
)

type momentPluginProbe struct {
	registryChecked bool
	dataReadTested  bool
	errorCode       string
}

// TestMomentPluginHealth is a read-only, point-in-time check of the saved
// plugin setting, this process's registrations, and the Owner-scoped data path.
// It does not create a Moment, invoke a model, or test event delivery.
func (a *App) TestMomentPluginHealth(ctx context.Context, owner string) (map[string]any, error) {
	return a.testMomentPluginHealth(ctx, owner, func(ctx context.Context) momentPluginProbe {
		registry := a.capabilityRegistry()
		for _, name := range []string{"moment.publish", momentInspectCapabilityName} {
			definition, defined := registry.Definition(name)
			_, installed := registry.LookupCapability(name)
			if !defined || !installed || !definition.SupportsSurface(CapabilitySurfaceConversation) {
				return momentPluginProbe{registryChecked: true, errorCode: "moment_capability_unavailable"}
			}
		}
		_, err := a.ownerMomentFeed(ctx, owner, url.Values{"limit": {"1"}})
		if err != nil {
			return momentPluginProbe{registryChecked: true, errorCode: "moment_data_read_failed"}
		}
		return momentPluginProbe{registryChecked: true, dataReadTested: true}
	})
}

func (a *App) testMomentPluginHealth(ctx context.Context, owner string, probe func(context.Context) momentPluginProbe) (map[string]any, error) {
	saved, err := a.ReadSettings(ctx, owner)
	if err != nil {
		return nil, err
	}
	raw := mapValue(saved["values"])[domainPluginsSettingKey]
	tested, err := decodeDomainPluginSettings(jsonBytes(raw))
	if err != nil {
		return nil, err
	}
	testedVersion, _ := parseRevision(mapValue(saved["versions"])[domainPluginsSettingKey])
	result := probe(ctx)
	// A check result belongs to the saved configuration it observed. Re-read
	// even on probe failure so an old failure cannot describe a new setting.
	latest, err := a.ReadSettings(ctx, owner)
	if err != nil {
		return nil, err
	}
	currentRaw := mapValue(latest["values"])[domainPluginsSettingKey]
	currentVersion, _ := parseRevision(mapValue(latest["versions"])[domainPluginsSettingKey])
	if currentVersion != testedVersion || jsonString(currentRaw) != jsonString(raw) {
		return map[string]any{"state": "configuration_changed", "tested_config_version": testedVersion, "config_version": currentVersion, "publication_tested": false, "model_tested": false}, nil
	}
	state := "ready"
	if result.errorCode != "" || !result.registryChecked || !result.dataReadTested {
		state = "unavailable"
	}
	return map[string]any{"state": state, "config_version": currentVersion, "plugin_enabled": tested.Moment,
		"registry_checked": result.registryChecked, "data_read_tested": result.dataReadTested, "error_code": result.errorCode,
		"publication_tested": false, "model_tested": false}, nil
}
