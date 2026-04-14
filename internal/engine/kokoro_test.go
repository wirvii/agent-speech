package engine

import (
	"testing"
)

func TestKokoro_Name(t *testing.T) {
	k := &Kokoro{}
	if k.Name() != "kokoro" {
		t.Errorf("Name: got %q, want %q", k.Name(), "kokoro")
	}
}

func TestDefaultVoiceKokoro(t *testing.T) {
	cases := []struct {
		lang string
		want string
	}{
		{"es", "ef_dora"},
		{"en", "af_heart"},
		{"fr", "ef_dora"}, // default para idiomas no mapeados
		{"", "ef_dora"},
	}
	for _, tc := range cases {
		got := DefaultVoiceKokoro(tc.lang)
		if got != tc.want {
			t.Errorf("DefaultVoiceKokoro(%q): got %q, want %q", tc.lang, got, tc.want)
		}
	}
}

func TestKokoroLangCode(t *testing.T) {
	cases := []struct {
		lang string
		want string
	}{
		{"es", "es"},
		{"en", "en-us"},
		{"fr", "es"}, // default
		{"", "es"},
	}
	for _, tc := range cases {
		got := kokoroLangCode(tc.lang)
		if got != tc.want {
			t.Errorf("kokoroLangCode(%q): got %q, want %q", tc.lang, got, tc.want)
		}
	}
}

func TestKokoro_Available_DoesNotPanic(t *testing.T) {
	k := &Kokoro{}
	// Solo verificamos que se puede llamar sin panic
	_ = k.Available()
}

func TestKokoro_Speak_EmptyText(t *testing.T) {
	k := &Kokoro{}
	// Texto vacio debe retornar nil sin ejecutar nada
	err := k.Speak(nil, "", SpeakOpts{Lang: "es"}) //nolint:staticcheck
	if err != nil {
		t.Errorf("Speak con texto vacio: esperaba nil, got %v", err)
	}
}
