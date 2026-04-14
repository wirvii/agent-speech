package engine

import (
	"testing"
)

func TestSay_Name(t *testing.T) {
	s := &Say{}
	if s.Name() != "say" {
		t.Errorf("Name: got %q, want %q", s.Name(), "say")
	}
}

func TestSplitText_ShortText(t *testing.T) {
	text := "texto corto"
	chunks := splitText(text, 200*1024)
	if len(chunks) != 1 {
		t.Errorf("splitText short: got %d chunks, want 1", len(chunks))
	}
	if chunks[0] != text {
		t.Errorf("splitText short: got %q, want %q", chunks[0], text)
	}
}

func TestSplitText_LongText(t *testing.T) {
	// Crear texto de 3 parrafos que supera el limite
	para := "Este es un parrafo de texto bastante largo que queremos dividir correctamente. "
	para = para + para + para + para + para // ~380 chars

	text := para + "\n\n" + para + "\n\n" + para
	chunks := splitText(text, 500)

	if len(chunks) < 2 {
		t.Errorf("splitText long: got %d chunks, want >= 2", len(chunks))
	}

	// Verificar que ningun chunk supera el limite (aprox)
	for i, c := range chunks {
		if len(c) > 1000 { // margen generoso
			t.Errorf("chunk[%d] demasiado grande: %d bytes", i, len(c))
		}
	}
}

func TestSplitText_EmptyText(t *testing.T) {
	chunks := splitText("", 1000)
	if len(chunks) != 1 || chunks[0] != "" {
		t.Errorf("splitText empty: got %v", chunks)
	}
}

func TestDefaultVoiceSay(t *testing.T) {
	cases := []struct {
		lang string
		want string
	}{
		{"es", "Paulina"},
		{"en", "Samantha"},
		{"fr", "Paulina"}, // default para idiomas no mapeados
		{"", "Paulina"},
	}
	for _, tc := range cases {
		got := DefaultVoiceSay(tc.lang)
		if got != tc.want {
			t.Errorf("DefaultVoiceSay(%q): got %q, want %q", tc.lang, got, tc.want)
		}
	}
}
