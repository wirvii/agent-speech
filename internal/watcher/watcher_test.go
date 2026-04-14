package watcher

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/agent-speech/internal/engine"
	"github.com/wirvii/agent-speech/internal/hook"
)

// mockEngine es un motor TTS de prueba que no habla nada.
type mockEngine struct {
	spoken []string
}

func (m *mockEngine) Speak(_ context.Context, text string, _ engine.SpeakOpts) error {
	m.spoken = append(m.spoken, text)
	return nil
}

func (m *mockEngine) Available() bool { return true }
func (m *mockEngine) Name() string    { return "mock" }

// writeTranscriptLine escribe una linea JSONL al transcript de test.
func writeTranscriptLine(t *testing.T, f *os.File, role, text string) {
	t.Helper()
	entry := map[string]any{
		"type": role,
		"role": role,
		"content": []map[string]any{
			{"type": "text", "text": text},
		},
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		t.Fatalf("escribir linea al transcript: %v", err)
	}
}

// writeWrappedAssistantLine escribe una linea en formato envuelto de Claude Code.
func writeWrappedAssistantLine(t *testing.T, f *os.File, text string) {
	t.Helper()
	entry := map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"role": "assistant",
			"content": []map[string]any{
				{"type": "text", "text": text},
			},
		},
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		t.Fatalf("escribir linea al transcript: %v", err)
	}
}

func TestWatcherPollDetectsNewLines(t *testing.T) {
	dir := t.TempDir()
	transcriptPath := filepath.Join(dir, "transcript.jsonl")

	// Crear transcript vacio.
	f, err := os.Create(transcriptPath)
	if err != nil {
		t.Fatalf("crear transcript: %v", err)
	}

	mock := &mockEngine{}
	w := New(transcriptPath, "", mock, engine.SpeakOpts{}, false)
	w.offset = 0

	// Poll inicial: nada.
	entries, err := w.poll()
	if err != nil {
		t.Fatalf("poll inicial: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("poll inicial deberia retornar 0 entradas, got %d", len(entries))
	}

	// Escribir mensaje no-assistant.
	writeTranscriptLine(t, f, "user", "Pregunta del usuario")

	entries, err = w.poll()
	if err != nil {
		t.Fatalf("poll despues de user: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("mensaje user no deberia generar entradas, got %d", len(entries))
	}

	// Escribir mensaje assistant.
	writeTranscriptLine(t, f, "assistant", "Respuesta del asistente.")

	entries, err = w.poll()
	if err != nil {
		t.Fatalf("poll despues de assistant: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("deberia detectar 1 mensaje assistant, got %d", len(entries))
	}

	f.Close()
}

func TestWatcherPollWrappedFormat(t *testing.T) {
	dir := t.TempDir()
	transcriptPath := filepath.Join(dir, "transcript.jsonl")

	f, err := os.Create(transcriptPath)
	if err != nil {
		t.Fatalf("crear transcript: %v", err)
	}
	defer f.Close()

	mock := &mockEngine{}
	w := New(transcriptPath, "", mock, engine.SpeakOpts{}, false)
	w.offset = 0

	// Escribir en formato envuelto (como lo hace Claude Code).
	writeWrappedAssistantLine(t, f, "Hola desde formato envuelto.")

	entries, err := w.poll()
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("deberia detectar 1 entrada, got %d", len(entries))
	}
}

func TestWatcherPollResetsOnTruncation(t *testing.T) {
	dir := t.TempDir()
	transcriptPath := filepath.Join(dir, "transcript.jsonl")

	// Crear transcript con contenido.
	f, err := os.Create(transcriptPath)
	if err != nil {
		t.Fatalf("crear transcript: %v", err)
	}
	writeTranscriptLine(t, f, "assistant", "Mensaje 1")
	f.Close()

	mock := &mockEngine{}
	w := New(transcriptPath, "", mock, engine.SpeakOpts{}, false)

	// Primera poll: leer mensaje.
	entries, err := w.poll()
	if err != nil {
		t.Fatalf("poll inicial: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("deberia detectar 1 entrada inicial, got %d", len(entries))
	}

	// Simular truncacion: reescribir el archivo con contenido menor.
	// El offset actual es mayor que el nuevo tamano del archivo.
	savedOffset := w.offset
	if savedOffset == 0 {
		t.Fatal("offset deberia ser > 0 despues de poll")
	}

	// Truncar el archivo.
	if err := os.WriteFile(transcriptPath, []byte{}, 0o644); err != nil {
		t.Fatalf("truncar transcript: %v", err)
	}

	// Poll: debe resetear offset y no retornar errores.
	entries, err = w.poll()
	if err != nil {
		t.Fatalf("poll despues de truncacion: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("transcript vacio no deberia retornar entradas, got %d", len(entries))
	}
	if w.offset != 0 {
		t.Errorf("offset deberia ser 0 despues de truncacion y archivo vacio, got %d", w.offset)
	}
}

func TestWatcherSpeakEntryMultipleParagraphs(t *testing.T) {
	dir := t.TempDir()
	transcriptPath := filepath.Join(dir, "transcript.jsonl")

	f, err := os.Create(transcriptPath)
	if err != nil {
		t.Fatalf("crear transcript: %v", err)
	}
	defer f.Close()

	mock := &mockEngine{}
	w := New(transcriptPath, "", mock, engine.SpeakOpts{}, false)
	w.offset = 0

	// Mensaje con multiples parrafos.
	text := "Primer parrafo del mensaje.\n\nSegundo parrafo con mas informacion.\n\nTercer parrafo final."
	writeTranscriptLine(t, f, "assistant", text)

	entries, err := w.poll()
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("deberia detectar 1 entrada, got %d", len(entries))
	}

	ctx := context.Background()
	w.speakEntry(ctx, entries[0])

	// Debe haber hablado los parrafos del texto.
	if len(mock.spoken) == 0 {
		t.Error("el mock engine deberia haber recibido texto para hablar")
	}

	// Verificar que el contenido hablado coincide con el texto original.
	allSpoken := strings.Join(mock.spoken, " ")
	if !strings.Contains(allSpoken, "Primer parrafo") {
		t.Errorf("el primer parrafo deberia haberse hablado, spoken: %v", mock.spoken)
	}
}

func TestWatcherIdleTimeoutTriggersShutdown(t *testing.T) {
	dir := t.TempDir()
	transcriptPath := filepath.Join(dir, "transcript.jsonl")

	// Crear transcript vacio.
	if _, err := os.Create(transcriptPath); err != nil {
		t.Fatalf("crear transcript: %v", err)
	}

	// Usar un watcher con idle timeout muy corto para el test.
	// No podemos cambiar la constante idleTimeout directamente,
	// pero podemos verificar la logica comprobando que Run() termina
	// cuando el contexto se cancela.
	mock := &mockEngine{}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := Run(ctx, transcriptPath, "", mock, engine.SpeakOpts{}, false)
	if err != nil {
		t.Errorf("Run deberia terminar sin error por timeout de contexto, got: %v", err)
	}
}

// TestWatcherPollSavesSharedOffset verifica que poll() guarda el offset compartido
// cuando el watcher tiene un sessionID. Esto permite que el Stop hook lea desde
// donde el watcher se quedo y atrape mensajes que el watcher no alcanzo a hablar.
func TestWatcherPollSavesSharedOffset(t *testing.T) {
	dir := t.TempDir()
	transcriptPath := filepath.Join(dir, "transcript.jsonl")

	// Redirigir OffsetDir a un directorio temporal para no contaminar el sistema.
	originalOffsetDir := hook.OffsetDir()
	offsetDir := filepath.Join(dir, "offsets")
	t.Setenv("HOME", dir) // hook.OffsetDir usa os.UserHomeDir -> HOME
	_ = originalOffsetDir

	f, err := os.Create(transcriptPath)
	if err != nil {
		t.Fatalf("crear transcript: %v", err)
	}
	defer f.Close()

	sessionID := "test-session-save-offset"
	mock := &mockEngine{}
	w := New(transcriptPath, sessionID, mock, engine.SpeakOpts{}, false)
	w.offset = 0

	// Escribir un mensaje assistant.
	writeTranscriptLine(t, f, "assistant", "Mensaje para el offset compartido.")

	// Poll: debe detectar el mensaje Y guardar el offset.
	entries, err := w.poll()
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("deberia detectar 1 entrada, got %d", len(entries))
	}

	// Verificar que el offset compartido fue guardado.
	savedOffset, err := hook.LoadOffset(sessionID)
	if err != nil {
		t.Fatalf("cargar offset guardado: %v", err)
	}
	if savedOffset == 0 {
		t.Error("el offset compartido deberia ser > 0 despues del poll")
	}
	if savedOffset != w.offset {
		t.Errorf("offset compartido %d != offset del watcher %d", savedOffset, w.offset)
	}

	// Limpiar el archivo de offset.
	os.Remove(filepath.Join(offsetDir, sessionID)) //nolint:errcheck
}

// TestWatcherPollNoSharedOffsetWithoutSessionID verifica que poll() NO guarda
// el offset compartido cuando no hay sessionID (sin efectos secundarios).
func TestWatcherPollNoSharedOffsetWithoutSessionID(t *testing.T) {
	dir := t.TempDir()
	transcriptPath := filepath.Join(dir, "transcript.jsonl")

	// Sin HOME modificado: solo verificamos que no falla sin sessionID.
	f, err := os.Create(transcriptPath)
	if err != nil {
		t.Fatalf("crear transcript: %v", err)
	}
	defer f.Close()

	mock := &mockEngine{}
	w := New(transcriptPath, "", mock, engine.SpeakOpts{}, false) // sessionID vacio
	w.offset = 0

	writeTranscriptLine(t, f, "assistant", "Mensaje sin session.")

	// Poll no debe fallar aunque no haya sessionID.
	entries, err := w.poll()
	if err != nil {
		t.Fatalf("poll sin sessionID: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("deberia detectar 1 entrada, got %d", len(entries))
	}
}

func TestWatcherRunWritesAndRemovesPIDFile(t *testing.T) {
	dir := t.TempDir()
	transcriptPath := filepath.Join(dir, "transcript.jsonl")

	// Crear transcript vacio.
	if _, err := os.Create(transcriptPath); err != nil {
		t.Fatalf("crear transcript: %v", err)
	}

	pidPath := PIDFilePath()
	// Limpiar el PID file si existe antes del test.
	os.Remove(pidPath) //nolint:errcheck

	mock := &mockEngine{}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	// Ejecutar Run en goroutine separada para no bloquear.
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, transcriptPath, "", mock, engine.SpeakOpts{}, false)
	}()

	// Esperar un poco para que el watcher escriba el PID file.
	time.Sleep(50 * time.Millisecond)

	// Verificar que el PID file existe mientras corre.
	if _, err := os.Stat(pidPath); os.IsNotExist(err) {
		t.Error("PID file deberia existir mientras el watcher corre")
	}

	// Esperar a que termine.
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run retorno error inesperado: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("Run no termino en tiempo esperado")
	}

	// Verificar que el PID file fue eliminado.
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("PID file deberia haber sido eliminado al terminar")
		os.Remove(pidPath) //nolint:errcheck
	}
}
