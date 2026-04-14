package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	hookCommand        = "agent-speech --from-hook"
	hookTimeout        = 120
	watcherHookCommand = "agent-speech --start-watcher"
	watcherHookTimeout = 10
)

// commands define los slash commands que agent-speech instala en ~/.claude/commands/.
var commands = map[string]string{
	"speak-on.md": "Ejecuta el siguiente comando para activar agent-speech:\n\n```bash\nagent-speech on\n```\n\nMuestra el resultado al usuario.\n",
	"speak-off.md": "Ejecuta el siguiente comando para desactivar agent-speech:\n\n```bash\nagent-speech off\n```\n\nMuestra el resultado al usuario.\n",
	"speak-voices.md": "Ejecuta el siguiente comando para listar las voces disponibles de agent-speech:\n\n```bash\nagent-speech voices\n```\n\nMuestra el resultado al usuario.\n",
}

// commandsDir retorna la ruta al directorio de commands de Claude Code.
func commandsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "commands"), nil
}

// InstallCommands crea los slash commands en ~/.claude/commands/.
func InstallCommands() error {
	dir, err := commandsDir()
	if err != nil {
		return fmt.Errorf("obtener directorio de commands: %w", err)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("crear directorio commands: %w", err)
	}

	for name, content := range commands {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("escribir command %s: %w", name, err)
		}
	}

	return nil
}

// RemoveCommands elimina los slash commands de agent-speech de ~/.claude/commands/.
func RemoveCommands() error {
	dir, err := commandsDir()
	if err != nil {
		return fmt.Errorf("obtener directorio de commands: %w", err)
	}

	for name := range commands {
		path := filepath.Join(dir, name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("eliminar command %s: %w", name, err)
		}
	}

	return nil
}


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

// setHook agrega ambos hooks de agent-speech (Stop y SessionStart) al mapa raw.
// Retorna true si agrego al menos uno (no existia antes).
func setHook(raw map[string]any) bool {
	hooks := ensureHooksMap(raw)
	added := false

	// --- Hook Stop ---
	stopHooks := ensureEventList(hooks, "Stop")
	stopExists := false
	for _, entry := range stopHooks {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		for _, h := range getInnerHooks(m) {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, _ := hm["command"].(string); cmd == hookCommand {
				stopExists = true
			}
		}
	}
	if !stopExists {
		stopHooks = append(stopHooks, map[string]any{
			"matcher": "",
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": hookCommand,
					"timeout": hookTimeout,
				},
			},
		})
		hooks["Stop"] = stopHooks
		added = true
	}

	// --- Hook SessionStart ---
	sessionHooks := ensureEventList(hooks, "SessionStart")
	sessionExists := false
	for _, entry := range sessionHooks {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		for _, h := range getInnerHooks(m) {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, _ := hm["command"].(string); cmd == watcherHookCommand {
				sessionExists = true
			}
		}
	}
	if !sessionExists {
		sessionHooks = append(sessionHooks, map[string]any{
			"matcher": "",
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": watcherHookCommand,
					"timeout": watcherHookTimeout,
				},
			},
		})
		hooks["SessionStart"] = sessionHooks
		added = true
	}

	raw["hooks"] = hooks
	return added
}

// removeHook elimina ambos hooks de agent-speech (Stop y SessionStart) del mapa raw.
func removeHook(raw map[string]any) {
	hooksRaw, ok := raw["hooks"]
	if !ok {
		return
	}
	hooks, ok := hooksRaw.(map[string]any)
	if !ok {
		return
	}

	removeCommandFromEvent(hooks, "Stop", hookCommand)
	removeCommandFromEvent(hooks, "SessionStart", watcherHookCommand)

	if len(hooks) == 0 {
		delete(raw, "hooks")
	}
}

// removeCommandFromEvent elimina un comando especifico de una lista de hooks de evento.
func removeCommandFromEvent(hooks map[string]any, event, command string) {
	eventRaw, ok := hooks[event]
	if !ok {
		return
	}
	eventList, ok := eventRaw.([]any)
	if !ok {
		return
	}

	var filtered []any
	for _, entry := range eventList {
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
			if cmd, _ := hm["command"].(string); cmd != command {
				filteredInner = append(filteredInner, h)
			}
		}

		if len(filteredInner) > 0 {
			m["hooks"] = filteredInner
			filtered = append(filtered, m)
		}
	}

	if len(filtered) == 0 {
		delete(hooks, event)
	} else {
		hooks[event] = filtered
	}
}

// findHook retorna true si al menos uno de los hooks de agent-speech esta en raw.
// Verifica tanto el hook Stop como el SessionStart.
func findHook(raw map[string]any) bool {
	hooksRaw, ok := raw["hooks"]
	if !ok {
		return false
	}
	hooks, ok := hooksRaw.(map[string]any)
	if !ok {
		return false
	}

	return findCommandInEvent(hooks, "Stop", hookCommand) ||
		findCommandInEvent(hooks, "SessionStart", watcherHookCommand)
}

// findCommandInEvent retorna true si un comando especifico esta en la lista de hooks de un evento.
func findCommandInEvent(hooks map[string]any, event, command string) bool {
	eventRaw, ok := hooks[event]
	if !ok {
		return false
	}
	eventList, ok := eventRaw.([]any)
	if !ok {
		return false
	}

	for _, entry := range eventList {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		for _, h := range getInnerHooks(m) {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, _ := hm["command"].(string); cmd == command {
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

// ensureEventList obtiene o crea la lista de hooks para un evento dado.
func ensureEventList(hooks map[string]any, event string) []any {
	raw, ok := hooks[event]
	if !ok {
		return []any{}
	}
	list, ok := raw.([]any)
	if !ok {
		return []any{}
	}
	return list
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
