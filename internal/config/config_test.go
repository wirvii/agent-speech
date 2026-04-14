package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wirvii/agent-speech/internal/config"
)

func TestDefaults(t *testing.T) {
	cfg := config.Defaults()
	if cfg.Lang != "es" {
		t.Errorf("Lang default: got %q, want %q", cfg.Lang, "es")
	}
	if cfg.Engine != "auto" {
		t.Errorf("Engine default: got %q, want %q", cfg.Engine, "auto")
	}
	if cfg.Rate != 25 {
		t.Errorf("Rate default: got %d, want 25", cfg.Rate)
	}
	if cfg.Verbose {
		t.Error("Verbose default: got true, want false")
	}
	if cfg.PiperModelDir == "" {
		t.Error("PiperModelDir default: got empty string")
	}
}

func TestLoad_NoFile(t *testing.T) {
	// Redirigir HOME a un directorio temporal sin config.toml
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load con archivo ausente: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load retorno nil con archivo ausente")
	}
	// Debe retornar defaults
	if cfg.Lang != "es" {
		t.Errorf("Lang: got %q, want %q", cfg.Lang, "es")
	}
}

func TestLoad_WithFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfgDir := filepath.Join(tmp, ".config", "agent-speech")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `lang = "en"
voice = "Alex"
rate = 200
engine = "say"
verbose = true
piper_model_dir = "/tmp/models"
`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Lang != "en" {
		t.Errorf("Lang: got %q, want %q", cfg.Lang, "en")
	}
	if cfg.Voice != "Alex" {
		t.Errorf("Voice: got %q, want %q", cfg.Voice, "Alex")
	}
	if cfg.Rate != 200 {
		t.Errorf("Rate: got %d, want 200", cfg.Rate)
	}
	if cfg.Engine != "say" {
		t.Errorf("Engine: got %q, want %q", cfg.Engine, "say")
	}
	if !cfg.Verbose {
		t.Error("Verbose: got false, want true")
	}
	if cfg.PiperModelDir != "/tmp/models" {
		t.Errorf("PiperModelDir: got %q, want %q", cfg.PiperModelDir, "/tmp/models")
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		input string
		want  string
	}{
		{"~/.config/foo", filepath.Join(home, ".config/foo")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"", ""},
	}

	for _, tc := range cases {
		got, err := config.ExpandPath(tc.input)
		if err != nil {
			t.Errorf("ExpandPath(%q): %v", tc.input, err)
		}
		if got != tc.want {
			t.Errorf("ExpandPath(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestWriteDefaults(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if err := config.WriteDefaults(); err != nil {
		t.Fatalf("WriteDefaults: %v", err)
	}

	path := filepath.Join(tmp, ".config", "agent-speech", "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 {
		t.Error("WriteDefaults: archivo vacio")
	}
}
