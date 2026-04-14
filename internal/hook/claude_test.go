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

func TestInstallCommands_CreatesFiles(t *testing.T) {
	claudeDir := setupClaudeDir(t)

	if err := hook.InstallCommands(); err != nil {
		t.Fatalf("InstallCommands: %v", err)
	}

	expected := []string{"speak-on.md", "speak-off.md", "speak-voices.md"}
	for _, name := range expected {
		path := filepath.Join(claudeDir, "commands", name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("InstallCommands: archivo %s no existe: %v", name, err)
		}
	}
}

func TestInstallCommands_FileContent(t *testing.T) {
	claudeDir := setupClaudeDir(t)

	if err := hook.InstallCommands(); err != nil {
		t.Fatalf("InstallCommands: %v", err)
	}

	checks := map[string]string{
		"speak-on.md":     "agent-speech on",
		"speak-off.md":    "agent-speech off",
		"speak-voices.md": "agent-speech voices",
	}

	for name, want := range checks {
		path := filepath.Join(claudeDir, "commands", name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("leer %s: %v", name, err)
		}
		content := string(data)
		if !containsString(content, want) {
			t.Errorf("archivo %s no contiene %q", name, want)
		}
	}
}

func TestInstallCommands_Idempotent(t *testing.T) {
	claudeDir := setupClaudeDir(t)

	if err := hook.InstallCommands(); err != nil {
		t.Fatalf("InstallCommands 1: %v", err)
	}
	if err := hook.InstallCommands(); err != nil {
		t.Fatalf("InstallCommands 2: %v", err)
	}

	// Verificar que no hay duplicados (exactamente 3 archivos)
	entries, err := os.ReadDir(filepath.Join(claudeDir, "commands"))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("InstallCommands idempotente: got %d archivos, want 3", len(entries))
	}
}

func TestRemoveCommands_DeletesFiles(t *testing.T) {
	claudeDir := setupClaudeDir(t)

	if err := hook.InstallCommands(); err != nil {
		t.Fatalf("InstallCommands: %v", err)
	}
	if err := hook.RemoveCommands(); err != nil {
		t.Fatalf("RemoveCommands: %v", err)
	}

	removed := []string{"speak-on.md", "speak-off.md", "speak-voices.md"}
	for _, name := range removed {
		path := filepath.Join(claudeDir, "commands", name)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("RemoveCommands: archivo %s aun existe", name)
		}
	}
}

func TestRemoveCommands_NoErrorIfMissing(t *testing.T) {
	setupClaudeDir(t)

	// Llamar RemoveCommands sin haber instalado primero no debe fallar
	if err := hook.RemoveCommands(); err != nil {
		t.Fatalf("RemoveCommands sin archivos previos: %v", err)
	}
}

func TestInstallCommands_CreatesDirIfMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// No crear el directorio .claude/commands, InstallCommands debe crearlo

	if err := hook.InstallCommands(); err != nil {
		t.Fatalf("InstallCommands: %v", err)
	}

	commandsDir := filepath.Join(tmp, ".claude", "commands")
	if _, err := os.Stat(commandsDir); err != nil {
		t.Errorf("InstallCommands: directorio commands no fue creado: %v", err)
	}
}

// containsString verifica si s contiene substr.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
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
