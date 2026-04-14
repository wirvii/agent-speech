package hook_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/wirvii/agent-speech/internal/hook"
)

func setupClaudeDir(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	claudeDir := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return claudeDir
}

func TestEnable_CreatesHook(t *testing.T) {
	setupClaudeDir(t)

	if err := hook.Enable(); err != nil {
		t.Fatalf("Enable: %v", err)
	}

	enabled, err := hook.IsEnabled()
	if err != nil {
		t.Fatalf("IsEnabled: %v", err)
	}
	if !enabled {
		t.Error("IsEnabled: got false, want true despues de Enable")
	}
}

func TestEnable_Idempotent(t *testing.T) {
	setupClaudeDir(t)

	if err := hook.Enable(); err != nil {
		t.Fatalf("Enable 1: %v", err)
	}
	if err := hook.Enable(); err != nil {
		t.Fatalf("Enable 2: %v", err)
	}

	// Verificar que no hay hooks duplicados
	home, _ := os.UserHomeDir()
	data, _ := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))

	var raw map[string]any
	_ = json.Unmarshal(data, &raw)

	count := countAgentSpeechHooks(raw)
	if count != 1 {
		t.Errorf("Enable idempotente: got %d hooks, want 1", count)
	}
}

func TestDisable_RemovesHook(t *testing.T) {
	setupClaudeDir(t)

	if err := hook.Enable(); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if err := hook.Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}

	enabled, err := hook.IsEnabled()
	if err != nil {
		t.Fatalf("IsEnabled: %v", err)
	}
	if enabled {
		t.Error("IsEnabled: got true, want false despues de Disable")
	}
}

func TestDisable_PreservesOtherHooks(t *testing.T) {
	claudeDir := setupClaudeDir(t)

	// Crear settings.json con hooks existentes
	settings := map[string]any{
		"hooks": map[string]any{
			"Stop": []any{
				map[string]any{
					"matcher": "",
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "otro-comando",
							"timeout": 60,
						},
					},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	_ = os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0o644)

	// Activar agent-speech
	if err := hook.Enable(); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	// Desactivar
	if err := hook.Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}

	// El hook de otro-comando debe seguir existiendo
	rawData, _ := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	var raw map[string]any
	_ = json.Unmarshal(rawData, &raw)

	hooks := raw["hooks"].(map[string]any)
	stop := hooks["Stop"].([]any)
	found := false
	for _, entry := range stop {
		m := entry.(map[string]any)
		innerHooks := m["hooks"].([]any)
		for _, h := range innerHooks {
			hm := h.(map[string]any)
			if hm["command"] == "otro-comando" {
				found = true
			}
		}
	}
	if !found {
		t.Error("Disable: elimino el hook de otro-comando (no debia)")
	}
}

func TestIsEnabled_NoFile(t *testing.T) {
	setupClaudeDir(t) // sin settings.json

	enabled, err := hook.IsEnabled()
	if err != nil {
		t.Fatalf("IsEnabled sin archivo: %v", err)
	}
	if enabled {
		t.Error("IsEnabled sin archivo: got true, want false")
	}
}

// countAgentSpeechHooks cuenta cuantas veces aparece agent-speech --from-hook en los hooks.
func countAgentSpeechHooks(raw map[string]any) int {
	count := 0
	hooksRaw, ok := raw["hooks"]
	if !ok {
		return 0
	}
	hooks, ok := hooksRaw.(map[string]any)
	if !ok {
		return 0
	}
	stopRaw, ok := hooks["Stop"]
	if !ok {
		return 0
	}
	stopList, ok := stopRaw.([]any)
	if !ok {
		return 0
	}
	for _, entry := range stopList {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		innerHooks, ok := m["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range innerHooks {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, _ := hm["command"].(string); cmd == "agent-speech --from-hook" {
				count++
			}
		}
	}
	return count
}
