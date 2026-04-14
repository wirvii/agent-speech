package hook_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/wirvii/agent-speech/internal/hook"
)

// TestExtractLastAssistantMessage_DirectFormat verifica extraccion con formato directo.
func TestExtractLastAssistantMessage_DirectFormat(t *testing.T) {
	lines := []map[string]any{
		{
			"type": "message",
			"role": "user",
			"content": []map[string]any{
				{"type": "text", "text": "hola"},
			},
		},
		{
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{
				{"type": "text", "text": "primer mensaje"},
			},
		},
		{
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{
				{"type": "text", "text": "segundo mensaje"},
			},
		},
	}

	path := writeJSONL(t, lines)
	got, err := hook.ExtractLastAssistantMessage(path)
	if err != nil {
		t.Fatalf("ExtractLastAssistantMessage: %v", err)
	}
	if got != "segundo mensaje" {
		t.Errorf("got %q, want %q", got, "segundo mensaje")
	}
}

// TestExtractLastAssistantMessage_WrappedFormat verifica extraccion con formato envuelto.
func TestExtractLastAssistantMessage_WrappedFormat(t *testing.T) {
	lines := []map[string]any{
		{
			"type": "assistant",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "respuesta envuelta"},
				},
			},
		},
	}

	path := writeJSONL(t, lines)
	got, err := hook.ExtractLastAssistantMessage(path)
	if err != nil {
		t.Fatalf("ExtractLastAssistantMessage wrapped: %v", err)
	}
	if got != "respuesta envuelta" {
		t.Errorf("got %q, want %q", got, "respuesta envuelta")
	}
}

// TestExtractLastAssistantMessage_EmptyTranscript verifica que retorna vacio con transcript vacio.
func TestExtractLastAssistantMessage_EmptyTranscript(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "transcript.jsonl")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := hook.ExtractLastAssistantMessage(path)
	if err != nil {
		t.Fatalf("ExtractLastAssistantMessage empty: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// TestExtractLastAssistantMessage_NoAssistant verifica que retorna vacio si no hay mensajes assistant.
func TestExtractLastAssistantMessage_NoAssistant(t *testing.T) {
	lines := []map[string]any{
		{
			"type": "message",
			"role": "user",
			"content": []map[string]any{
				{"type": "text", "text": "solo usuario"},
			},
		},
	}

	path := writeJSONL(t, lines)
	got, err := hook.ExtractLastAssistantMessage(path)
	if err != nil {
		t.Fatalf("ExtractLastAssistantMessage no assistant: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// TestExtractTextFromContent_MultipleBlocks verifica concatenacion de multiples bloques text.
func TestExtractTextFromContent_MultipleBlocks(t *testing.T) {
	items := []map[string]any{
		{"type": "text", "text": "primer bloque"},
		{"type": "tool_use", "id": "tu_123"},
		{"type": "text", "text": " segundo bloque"},
	}
	raw, _ := json.Marshal(items)

	got := hook.ExtractTextFromContent(raw)
	want := "primer bloque segundo bloque"
	if got != want {
		t.Errorf("ExtractTextFromContent multi: got %q, want %q", got, want)
	}
}

// TestExtractTextFromContent_SingleBlock verifica un solo bloque de texto.
func TestExtractTextFromContent_SingleBlock(t *testing.T) {
	items := []map[string]any{
		{"type": "text", "text": "unico bloque"},
	}
	raw, _ := json.Marshal(items)

	got := hook.ExtractTextFromContent(raw)
	if got != "unico bloque" {
		t.Errorf("got %q, want %q", got, "unico bloque")
	}
}

// TestExtractTextFromContent_StringContent verifica que acepta content como string directo.
func TestExtractTextFromContent_StringContent(t *testing.T) {
	raw, _ := json.Marshal("texto directo")

	got := hook.ExtractTextFromContent(raw)
	if got != "texto directo" {
		t.Errorf("got %q, want %q", got, "texto directo")
	}
}

// TestExtractTextFromContent_OnlyToolUse verifica que retorna vacio si no hay bloques text.
func TestExtractTextFromContent_OnlyToolUse(t *testing.T) {
	items := []map[string]any{
		{"type": "tool_use", "id": "tu_456"},
	}
	raw, _ := json.Marshal(items)

	got := hook.ExtractTextFromContent(raw)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// writeJSONL escribe una lista de objetos como JSONL en un archivo temporal.
func writeJSONL(t *testing.T, lines []map[string]any) string {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "transcript.jsonl")

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, l := range lines {
		if err := enc.Encode(l); err != nil {
			t.Fatal(err)
		}
	}
	return path
}
