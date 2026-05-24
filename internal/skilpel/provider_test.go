package skilpel

import "testing"

func TestProviderPluginMapMatchesOrderedPlugins(t *testing.T) {
	for _, plugin := range orderedProviderPlugins {
		resolved, err := resolveProviderPlugin(plugin.Name)
		if err != nil {
			t.Fatalf("resolve provider %q: %v", plugin.Name, err)
		}
		if resolved.Name != plugin.Name ||
			resolved.Description != plugin.Description ||
			resolved.DefaultBaseURL != plugin.DefaultBaseURL ||
			resolved.DefaultAPIKeyEnv != plugin.DefaultAPIKeyEnv ||
			resolved.BaseURLOverride != plugin.BaseURLOverride {
			t.Fatalf("provider %q map entry drifted from ordered registry", plugin.Name)
		}
		if resolved.New == nil {
			t.Fatalf("provider %q has no constructor", plugin.Name)
		}
	}
	if len(providerPlugins) != len(orderedProviderPlugins) {
		t.Fatalf("provider map has %d entries, ordered registry has %d", len(providerPlugins), len(orderedProviderPlugins))
	}
}

func TestDefaultProviderIsOpenAI(t *testing.T) {
	plugin, err := resolveProviderPlugin("")
	if err != nil {
		t.Fatalf("resolve provider: %v", err)
	}
	if plugin.Name != defaultProviderName {
		t.Fatalf("provider = %q, want %s", plugin.Name, defaultProviderName)
	}
}
