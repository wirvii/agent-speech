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

// TestExtractNewAssistantMessages_FromZero verifica lectura completa con offset 0.
func TestExtractNewAssistantMessages_FromZero(t *testing.T) {
	lines := []map[string]any{
		{
			"type": "message", "role": "user",
			"content": []map[string]any{{"type": "text", "text": "hola"}},
		},
		{
			"type": "message", "role": "assistant",
			"content": []map[string]any{{"type": "text", "text": "primer mensaje"}},
		},
		{
			"type": "message", "role": "assistant",
			"content": []map[string]any{{"type": "text", "text": "segundo mensaje"}},
		},
	}

	path := writeJSONL(t, lines)
	msgs, newOffset, err := hook.ExtractNewAssistantMessages(path, 0)
	if err != nil {
		t.Fatalf("ExtractNewAssistantMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d mensajes, want 2", len(msgs))
	}
	if msgs[0] != "primer mensaje" {
		t.Errorf("msgs[0] = %q, want %q", msgs[0], "primer mensaje")
	}
	if msgs[1] != "segundo mensaje" {
		t.Errorf("msgs[1] = %q, want %q", msgs[1], "segundo mensaje")
	}
	if newOffset <= 0 {
		t.Errorf("newOffset = %d, debe ser > 0", newOffset)
	}
}

// TestExtractNewAssistantMessages_FromOffset verifica lectura parcial desde offset > 0.
func TestExtractNewAssistantMessages_FromOffset(t *testing.T) {
	// Primera parte del transcript (2 mensajes assistant).
	firstLines := []map[string]any{
		{
			"type": "message", "role": "assistant",
			"content": []map[string]any{{"type": "text", "text": "msg1"}},
		},
		{
			"type": "message", "role": "assistant",
			"content": []map[string]any{{"type": "text", "text": "msg2"}},
		},
	}

	path := writeJSONL(t, firstLines)

	// Primera lectura: offset 0 -> obtiene 2 mensajes y nuevo offset.
	_, offsetAfterFirst, err := hook.ExtractNewAssistantMessages(path, 0)
	if err != nil {
		t.Fatalf("primera lectura: %v", err)
	}

	// Agregar 3 mensajes nuevos al archivo.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("abrir para append: %v", err)
	}
	enc := json.NewEncoder(f)
	for _, msg := range []string{"msg3", "msg4", "msg5"} {
		line := map[string]any{
			"type": "message", "role": "assistant",
			"content": []map[string]any{{"type": "text", "text": msg}},
		}
		if err := enc.Encode(line); err != nil {
			f.Close()
			t.Fatalf("encode: %v", err)
		}
	}
	f.Close()

	// Segunda lectura desde el offset guardado: solo debe retornar los 3 nuevos.
	msgs, _, err := hook.ExtractNewAssistantMessages(path, offsetAfterFirst)
	if err != nil {
		t.Fatalf("segunda lectura: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("got %d mensajes, want 3; msgs: %v", len(msgs), msgs)
	}
	for i, want := range []string{"msg3", "msg4", "msg5"} {
		if msgs[i] != want {
			t.Errorf("msgs[%d] = %q, want %q", i, msgs[i], want)
		}
	}
}

// TestLoadSaveOffset_RoundTrip verifica que guardar y cargar el offset produce el mismo valor.
func TestLoadSaveOffset_RoundTrip(t *testing.T) {
	// Usar directorio temporal para offsets durante el test.
	origOffsetDir := hook.OffsetDir()
	_ = origOffsetDir // no usamos override, usamos tmp session ID unico

	sessionID := "test-session-" + t.Name()
	// Primero cargar: debe retornar 0 porque no existe.
	got, err := hook.LoadOffset(sessionID)
	if err != nil {
		t.Fatalf("LoadOffset inicial: %v", err)
	}
	if got != 0 {
		t.Errorf("LoadOffset inicial = %d, want 0", got)
	}

	// Guardar un offset.
	want := int64(12345)
	if err := hook.SaveOffset(sessionID, want); err != nil {
		t.Fatalf("SaveOffset: %v", err)
	}
	defer func() {
		// Limpiar el archivo creado.
		os.Remove(filepath.Join(hook.OffsetDir(), sessionID))
	}()

	// Cargar de nuevo: debe retornar el valor guardado.
	got, err = hook.LoadOffset(sessionID)
	if err != nil {
		t.Fatalf("LoadOffset post-save: %v", err)
	}
	if got != want {
		t.Errorf("LoadOffset = %d, want %d", got, want)
	}
}

// TestExtractNewAssistantMessages_IncrementalReadFlow verifica el flujo completo incremental:
// primera lectura de 2 mensajes, luego agregar 3 y que la segunda lectura retorne solo los nuevos.
func TestExtractNewAssistantMessages_IncrementalReadFlow(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "transcript.jsonl")

	// Escribir 2 mensajes iniciales.
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	enc := json.NewEncoder(f)
	for _, msg := range []string{"inicial-1", "inicial-2"} {
		line := map[string]any{
			"type": "message", "role": "assistant",
			"content": []map[string]any{{"type": "text", "text": msg}},
		}
		if err := enc.Encode(line); err != nil {
			f.Close()
			t.Fatal(err)
		}
	}
	f.Close()

	// Primera lectura.
	msgs1, offset1, err := hook.ExtractNewAssistantMessages(path, 0)
	if err != nil {
		t.Fatalf("primera lectura: %v", err)
	}
	if len(msgs1) != 2 {
		t.Fatalf("primera lectura: got %d mensajes, want 2", len(msgs1))
	}

	// Agregar 3 mensajes nuevos.
	f, err = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	enc = json.NewEncoder(f)
	for _, msg := range []string{"nuevo-1", "nuevo-2", "nuevo-3"} {
		line := map[string]any{
			"type": "message", "role": "assistant",
			"content": []map[string]any{{"type": "text", "text": msg}},
		}
		if err := enc.Encode(line); err != nil {
			f.Close()
			t.Fatal(err)
		}
	}
	f.Close()

	// Segunda lectura desde offset guardado.
	msgs2, _, err := hook.ExtractNewAssistantMessages(path, offset1)
	if err != nil {
		t.Fatalf("segunda lectura: %v", err)
	}
	if len(msgs2) != 3 {
		t.Fatalf("segunda lectura: got %d mensajes, want 3; msgs: %v", len(msgs2), msgs2)
	}
	for i, want := range []string{"nuevo-1", "nuevo-2", "nuevo-3"} {
		if msgs2[i] != want {
			t.Errorf("msgs2[%d] = %q, want %q", i, msgs2[i], want)
		}
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
