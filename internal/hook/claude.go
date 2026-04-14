package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	hookCommand = "agent-speech --from-hook"
	hookTimeout = 120
)


// Init configura el hook Stop en ~/.claude/settings.json.
func Init() error {
	return Enable()
}

// Enable activa el hook de agent-speech (idempotente).
func Enable() error {
	raw, err := loadSettings()
	if err != nil {
		return err
	}

	if raw == nil {
		raw = make(map[string]any)
	}

	if !setHook(raw) {
		// Ya estaba configurado
		return nil
	}

	return saveSettings(raw)
}

// Disable remueve el hook de agent-speech, preservando el resto.
func Disable() error {
	raw, err := loadSettings()
	if err != nil {
		return err
	}

	if raw == nil {
		return nil
	}

	removeHook(raw)
	return saveSettings(raw)
}

// IsEnabled retorna true si el hook de agent-speech esta configurado.
func IsEnabled() (bool, error) {
	raw, err := loadSettings()
	if err != nil {
		return false, err
	}
	if raw == nil {
		return false, nil
	}

	return findHook(raw), nil
}

// settingsPath retorna la ruta al settings.json de Claude Code.
func settingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// loadSettings lee settings.json como mapa generico para preservar campos desconocidos.
func loadSettings() (map[string]any, error) {
	path, err := settingsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("leer settings.json: %w", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsear settings.json: %w", err)
	}

	return raw, nil
}

// saveSettings escribe el mapa como settings.json con indentacion.
func saveSettings(raw map[string]any) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("crear directorio .claude: %w", err)
	}

	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("serializar settings: %w", err)
	}

	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// setHook agrega el hook de agent-speech al mapa raw.
// Retorna true si lo agrego (no existia antes).
func setHook(raw map[string]any) bool {
	hooks := ensureHooksMap(raw)
	stopHooks := ensureStopList(hooks)

	// Verificar si ya existe
	for _, entry := range stopHooks {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		innerHooks := getInnerHooks(m)
		for _, h := range innerHooks {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, _ := hm["command"].(string); cmd == hookCommand {
				return false // ya existe
			}
		}
	}

	// Agregar el nuevo hook
	newEntry := map[string]any{
		"matcher": "",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": hookCommand,
				"timeout": hookTimeout,
			},
		},
	}

	stopHooks = append(stopHooks, newEntry)
	hooks["Stop"] = stopHooks
	raw["hooks"] = hooks
	return true
}

// removeHook elimina el hook de agent-speech del mapa raw.
func removeHook(raw map[string]any) {
	hooksRaw, ok := raw["hooks"]
	if !ok {
		return
	}
	hooks, ok := hooksRaw.(map[string]any)
	if !ok {
		return
	}

	stopRaw, ok := hooks["Stop"]
	if !ok {
		return
	}
	stopList, ok := stopRaw.([]any)
	if !ok {
		return
	}

	var filtered []any
	for _, entry := range stopList {
		m, ok := entry.(map[string]any)
		if !ok {
			filtered = append(filtered, entry)
			continue
		}

		innerHooks := getInnerHooks(m)
		var filteredInner []any
		for _, h := range innerHooks {
			hm, ok := h.(map[string]any)
			if !ok {
				filteredInner = append(filteredInner, h)
				continue
			}
			if cmd, _ := hm["command"].(string); cmd != hookCommand {
				filteredInner = append(filteredInner, h)
			}
		}

		if len(filteredInner) > 0 {
			m["hooks"] = filteredInner
			filtered = append(filtered, m)
		}
	}

	if len(filtered) == 0 {
		delete(hooks, "Stop")
	} else {
		hooks["Stop"] = filtered
	}

	if len(hooks) == 0 {
		delete(raw, "hooks")
	}
}

// findHook retorna true si el hook de agent-speech esta en raw.
func findHook(raw map[string]any) bool {
	hooksRaw, ok := raw["hooks"]
	if !ok {
		return false
	}
	hooks, ok := hooksRaw.(map[string]any)
	if !ok {
		return false
	}

	stopRaw, ok := hooks["Stop"]
	if !ok {
		return false
	}
	stopList, ok := stopRaw.([]any)
	if !ok {
		return false
	}

	for _, entry := range stopList {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		innerHooks := getInnerHooks(m)
		for _, h := range innerHooks {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, _ := hm["command"].(string); cmd == hookCommand {
				return true
			}
		}
	}
	return false
}

// ensureHooksMap obtiene o crea el mapa "hooks" en raw.
func ensureHooksMap(raw map[string]any) map[string]any {
	hooksRaw, ok := raw["hooks"]
	if !ok {
		hooks := make(map[string]any)
		raw["hooks"] = hooks
		return hooks
	}
	hooks, ok := hooksRaw.(map[string]any)
	if !ok {
		hooks = make(map[string]any)
		raw["hooks"] = hooks
		return hooks
	}
	return hooks
}

// ensureStopList obtiene o crea la lista "Stop" en hooks.
func ensureStopList(hooks map[string]any) []any {
	stopRaw, ok := hooks["Stop"]
	if !ok {
		return []any{}
	}
	stopList, ok := stopRaw.([]any)
	if !ok {
		return []any{}
	}
	return stopList
}

// getInnerHooks obtiene la lista de hooks internos de un matcher.
func getInnerHooks(m map[string]any) []any {
	hRaw, ok := m["hooks"]
	if !ok {
		return nil
	}
	h, ok := hRaw.([]any)
	if !ok {
		return nil
	}
	return h
}
