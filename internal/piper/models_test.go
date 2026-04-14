package piper_test

import (
	"testing"

	"github.com/wirvii/agent-speech/internal/piper"
)

func TestCatalog_NotEmpty(t *testing.T) {
	if len(piper.Catalog) == 0 {
		t.Error("Catalog: vacio")
	}
}

func TestCatalog_HasSpanish(t *testing.T) {
	found := false
	for _, m := range piper.Catalog {
		if m.Lang == "es" && m.Region == "MX" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Catalog: no hay modelo español MX")
	}
}

func TestCatalog_HasEnglish(t *testing.T) {
	found := false
	for _, m := range piper.Catalog {
		if m.Lang == "en" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Catalog: no hay modelo ingles")
	}
}

func TestResolve_ByLang(t *testing.T) {
	cases := []struct {
		lang    string
		wantID  string
		wantErr bool
	}{
		{"es", "es_MX-claude-high", false},
		{"en", "en_US-lessac-medium", false},
		{"fr", "", true},
	}
	for _, tc := range cases {
		info, err := piper.Resolve(tc.lang, "")
		if tc.wantErr {
			if err == nil {
				t.Errorf("Resolve(%q, ''): esperaba error", tc.lang)
			}
			continue
		}
		if err != nil {
			t.Errorf("Resolve(%q, ''): %v", tc.lang, err)
			continue
		}
		if info.ID != tc.wantID {
			t.Errorf("Resolve(%q, ''): got ID %q, want %q", tc.lang, info.ID, tc.wantID)
		}
	}
}

func TestResolve_ByVoice(t *testing.T) {
	cases := []struct {
		voice   string
		wantID  string
		wantErr bool
	}{
		{"es_MX-ald-medium", "es_MX-ald-medium", false},
		{"en_US-lessac-medium", "en_US-lessac-medium", false},
		{"modelo-inexistente", "", true},
	}
	for _, tc := range cases {
		info, err := piper.Resolve("es", tc.voice)
		if tc.wantErr {
			if err == nil {
				t.Errorf("Resolve('es', %q): esperaba error", tc.voice)
			}
			continue
		}
		if err != nil {
			t.Errorf("Resolve('es', %q): %v", tc.voice, err)
			continue
		}
		if info.ID != tc.wantID {
			t.Errorf("Resolve('es', %q): got ID %q, want %q", tc.voice, info.ID, tc.wantID)
		}
	}
}

func TestModelPath_NotFound(t *testing.T) {
	_, err := piper.ModelPath("es_MX-claude-high", "/tmp/nonexistent-dir-xyz")
	if err == nil {
		t.Error("ModelPath: esperaba error para modelo no descargado")
	}
}
