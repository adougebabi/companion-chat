package core

import (
	"os"
	"strings"
	"testing"
)

func TestProviderRuntimeSupportDoesNotConstructAppForDiagnosticsOrQueue(t *testing.T) {
	for _, file := range []string{"provider.go", "provider_queue.go", "eino_model_runtime.go", "model_tasks.go"} {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(source)
		for _, forbidden := range []string{"&App{DB", "NewApp(", "App{DB:"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s contains forbidden business App construction %q", file, forbidden)
			}
		}
	}
}

func TestProviderRuntimeSupportIsNarrowAndWiredAtCompositionRoot(t *testing.T) {
	providerSource, err := os.ReadFile("provider_runtime_support.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(providerSource)
	for _, required := range []string{"type ProviderRuntimeSupport interface", "RecordQueuedModelRun", "UpdateModelRunState", "RecordLifecycleDiagnosticBestEffort"} {
		if !strings.Contains(text, required) {
			t.Fatalf("runtime support contract missing %q", required)
		}
	}
	appSource, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(appSource), "SetRuntimeSupport(newProviderRuntimeSupport(repository))") {
		t.Fatal("ProviderRuntimeSupport is not wired at the composition root")
	}
}
