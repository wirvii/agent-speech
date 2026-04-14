package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config almacena la configuracion del plugin.
type Config struct {
	Lang          string `toml:"lang"`
	Voice         string `toml:"voice"`
	Rate          int    `toml:"rate"`
	Engine        string `toml:"engine"`
	Verbose       bool   `toml:"verbose"`
	PiperModelDir string `toml:"piper_model_dir"`
}

// Defaults retorna la configuracion por defecto.
func Defaults() *Config {
	return &Config{
		Lang:          "es",
		Voice:         "",
		Rate:          25,
		Engine:        "auto",
		Verbose:       false,
		PiperModelDir: "~/.local/share/agent-speech/models",
	}
}

// Load lee ~/.config/agent-speech/config.toml si existe.
// Si no existe, retorna defaults. No falla por archivo ausente.
func Load() (*Config, error) {
	cfg := Defaults()

	cfgPath, err := configPath()
	if err != nil {
		return cfg, nil
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if _, err := toml.Decode(string(data), cfg); err != nil {
		return nil, err
	}

	if cfg.PiperModelDir == "" {
		cfg.PiperModelDir = "~/.local/share/agent-speech/models"
	}

	return cfg, nil
}

// configPath retorna la ruta al archivo de configuracion.
func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "agent-speech", "config.toml"), nil
}

// ConfigPath retorna la ruta al archivo de configuracion (exportado).
func ConfigPath() (string, error) {
	return configPath()
}

// WriteDefaults escribe el archivo de configuracion con valores por defecto.
func WriteDefaults() error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	content := `# Idioma por defecto: "es" o "en"
lang = "es"

# Voz especifica (vacio = default por idioma y motor)
# edge-tts: "es-MX-DaliaNeural", "en-US-JennyNeural", etc.
# kokoro: "ef_dora", "af_heart", etc.
# macOS: "Paulina", "Juan", "Samantha", "Alex"
# piper: "es_MX-claude-high", "es_MX-ald-medium", "en_US-lessac-medium"
voice = ""

# Velocidad: porcentaje de incremento (25 = +25% mas rapido, 0 = velocidad normal)
rate = 25

# Motor TTS: "auto" (detecta por OS), "say", "edge-tts", "kokoro", "piper"
engine = "auto"

# Directorio de modelos piper (solo si usas piper)
piper_model_dir = "~/.local/share/agent-speech/models"

# Logs detallados a stderr
verbose = false
`
	return os.WriteFile(path, []byte(content), 0o644)
}

// ExpandPath expande ~ al home del usuario.
func ExpandPath(p string) (string, error) {
	if len(p) == 0 || p[0] != '~' {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, p[1:]), nil
}
