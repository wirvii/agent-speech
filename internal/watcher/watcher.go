package watcher

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/wirvii/agent-speech/internal/engine"
	"github.com/wirvii/agent-speech/internal/hook"
	"github.com/wirvii/agent-speech/internal/markdown"
)

const (
	// pollInterval es el intervalo entre lecturas del transcript.
	pollInterval = 300 * time.Millisecond

	// idleTimeout es el tiempo maximo sin cambios antes de hacer shutdown graceful.
	idleTimeout = 5 * time.Minute
)

// Watcher monitorea un transcript JSONL y habla los mensajes nuevos del assistant.
// Es single-goroutine con un loop secuencial.
type Watcher struct {
	transcriptPath string
	sessionID      string
	eng            engine.Engine
	opts           engine.SpeakOpts
	verbose        bool

	// offset es el byte offset actual en el transcript.
	offset int64
}

// New crea un nuevo Watcher.
func New(transcriptPath, sessionID string, eng engine.Engine, opts engine.SpeakOpts, verbose bool) *Watcher {
	return &Watcher{
		transcriptPath: transcriptPath,
		sessionID:      sessionID,
		eng:            eng,
		opts:           opts,
		verbose:        verbose,
	}
}

// Run ejecuta el loop principal del watcher hasta que el contexto se cancele.
// Escribe el PID file al inicio y lo elimina al salir.
func Run(ctx context.Context, transcriptPath, sessionID string, eng engine.Engine, opts engine.SpeakOpts, verbose bool) error {
	w := New(transcriptPath, sessionID, eng, opts, verbose)

	// Escribir PID file.
	if err := WritePID(transcriptPath); err != nil {
		return fmt.Errorf("escribir PID file: %w", err)
	}
	defer RemovePID() //nolint:errcheck

	// Inicializar offset: ir al final del archivo si ya tiene contenido.
	if err := w.initOffset(); err != nil {
		// No fatal: arrancar desde 0.
		if verbose {
			log.Printf("watcher: advertencia al inicializar offset: %v", err)
		}
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	lastChange := time.Now()

	if verbose {
		log.Printf("watcher: iniciado, monitoreando %s desde offset %d", transcriptPath, w.offset)
	}

	for {
		select {
		case <-ctx.Done():
			if verbose {
				log.Printf("watcher: shutdown por context cancelado")
			}
			return nil

		case <-ticker.C:
			entries, err := w.poll()
			if err != nil {
				if verbose {
					log.Printf("watcher: error en poll: %v", err)
				}
				// No fatal: el archivo puede no existir todavia.
				continue
			}

			if len(entries) > 0 {
				lastChange = time.Now()
				for _, entry := range entries {
					if ctx.Err() != nil {
						return nil
					}
					w.speakEntry(ctx, entry)
				}
			} else {
				// Verificar idle timeout.
				if time.Since(lastChange) > idleTimeout {
					if verbose {
						log.Printf("watcher: idle timeout de %v alcanzado, shutdown graceful", idleTimeout)
					}
					return nil
				}
			}
		}
	}
}

// initOffset inicializa el byte offset del watcher.
// Usa el offset guardado para la sesion si existe, de lo contrario va al final del archivo.
func (w *Watcher) initOffset() error {
	if w.sessionID != "" {
		savedOffset, err := hook.LoadOffset(w.sessionID)
		if err == nil && savedOffset > 0 {
			w.offset = savedOffset
			return nil
		}
	}

	// Sin offset guardado: ir al final del archivo para no repetir mensajes viejos.
	f, err := os.Open(w.transcriptPath)
	if err != nil {
		if os.IsNotExist(err) {
			// El archivo no existe todavia: arrancar desde 0.
			w.offset = 0
			return nil
		}
		return fmt.Errorf("abrir transcript para inicializar offset: %w", err)
	}
	defer f.Close()

	endOffset, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("seek al final del transcript: %w", err)
	}

	w.offset = endOffset
	return nil
}

// poll lee las lineas nuevas del transcript desde el offset actual.
// Retorna las entradas assistant nuevas y actualiza w.offset.
func (w *Watcher) poll() ([]hook.TranscriptLine, error) {
	f, err := os.Open(w.transcriptPath)
	if err != nil {
		return nil, fmt.Errorf("abrir transcript: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat transcript: %w", err)
	}

	// Transcript se trunco/roto: resetear offset.
	if info.Size() < w.offset {
		w.offset = 0
	}

	// Nada nuevo.
	if info.Size() == w.offset {
		return nil, nil
	}

	if _, err := f.Seek(w.offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek al offset %d: %w", w.offset, err)
	}

	reader := bufio.NewReader(f)
	var entries []hook.TranscriptLine

	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			// Pre-filtro: solo parsear lineas que puedan contener mensajes assistant.
			if bytes.Contains(line, []byte(`"assistant"`)) {
				var entry hook.TranscriptLine
				if jsonErr := json.Unmarshal(line, &entry); jsonErr == nil {
					text := hook.ExtractTextFromEntry(entry)
					if text != "" {
						entries = append(entries, entry)
					}
				}
			}
		}
		if readErr != nil {
			break
		}
	}

	// Actualizar offset al final actual del archivo.
	newOffset, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, fmt.Errorf("obtener offset final: %w", err)
	}
	w.offset = newOffset

	return entries, nil
}

// speakEntry habla el contenido de una entrada del transcript.
// Divide el texto en parrafos y habla cada uno con markdown.Clean().
func (w *Watcher) speakEntry(ctx context.Context, entry hook.TranscriptLine) {
	text := hook.ExtractTextFromEntry(entry)
	if text == "" {
		return
	}

	complete, residual := SplitParagraphs(text)

	// Los parrafos completos se hablan secuencialmente.
	allParagraphs := complete
	if residual != "" {
		allParagraphs = append(allParagraphs, residual)
	}

	for _, p := range allParagraphs {
		if ctx.Err() != nil {
			return
		}

		clean := markdown.Clean(p)
		if clean == "" {
			continue
		}

		if w.verbose {
			log.Printf("watcher: hablando parrafo (%d chars): %s...", len(clean), truncateStr(clean, 50))
		}

		if err := w.eng.Speak(ctx, clean, w.opts); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("watcher: error hablando parrafo: %v", err)
			// No fatal: continuar con el siguiente parrafo.
		}
	}
}

// truncateStr trunca un string a maxLen caracteres para logs.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
