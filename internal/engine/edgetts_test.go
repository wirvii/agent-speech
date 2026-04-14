package engine

import (
	"testing"
)

func TestEdgeTTS_Name(t *testing.T) {
	e := &EdgeTTS{}
	if e.Name() != "edge-tts" {
		t.Errorf("Name: got %q, want %q", e.Name(), "edge-tts")
	}
}

func TestDefaultVoiceEdgeTTS(t *testing.T) {
	cases := []struct {
		lang string
		want string
	}{
		{"es", "es-MX-DaliaNeural"},
		{"en", "en-US-JennyNeural"},
		{"fr", "es-MX-DaliaNeural"}, // default para idiomas no mapeados
		{"", "es-MX-DaliaNeural"},
	}
	for _, tc := range cases {
		got := DefaultVoiceEdgeTTS(tc.lang)
		if got != tc.want {
			t.Errorf("DefaultVoiceEdgeTTS(%q): got %q, want %q", tc.lang, got, tc.want)
		}
	}
}

func TestEdgeTTS_Available_DoesNotPanic(t *testing.T) {
	e := &EdgeTTS{}
	// Solo verificamos que se puede llamar sin panic
	_ = e.Available()
}

func TestEdgeTTS_Speak_EmptyText(t *testing.T) {
	e := &EdgeTTS{}
	// Texto vacio debe retornar nil sin ejecutar nada
	err := e.Speak(nil, "", SpeakOpts{Lang: "es"}) //nolint:staticcheck
	if err != nil {
		t.Errorf("Speak con texto vacio: esperaba nil, got %v", err)
	}
}
