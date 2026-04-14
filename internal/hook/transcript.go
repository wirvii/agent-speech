package hook

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// TranscriptLine es una linea del JSONL de transcript de Claude Code.
type TranscriptLine struct {
	Type    string          `json:"type"`
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
	Message *TranscriptLine `json:"message"`
}

// ContentItem es un elemento de contenido en un mensaje de Claude Code.
type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ExtractLastAssistantMessage lee el JSONL del transcript y retorna el ultimo mensaje del asistente.
func ExtractLastAssistantMessage(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("abrir transcript %s: %w", path, err)
	}
	defer f.Close()

	var lastText string
	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			// Pre-filtro: solo parsear lineas que puedan contener mensajes assistant.
			// Evita parsear JSON de lineas grandes de tool_result u otras entradas irrelevantes.
			if bytes.Contains(line, []byte(`"assistant"`)) {
				var entry TranscriptLine
				if jsonErr := json.Unmarshal(line, &entry); jsonErr == nil {
					text := extractTextFromEntry(entry)
					if text != "" {
						lastText = text
					}
				}
			}
		}
		if err != nil {
			if err != io.EOF {
				return "", fmt.Errorf("leer transcript: %w", err)
			}
			break
		}
	}

	return lastText, nil
}

// extractTextFromEntry extrae el texto de una linea del transcript.
func extractTextFromEntry(entry TranscriptLine) string {
	// Formato directo: {"type":"message","role":"assistant","content":[...]}
	if entry.Role == "assistant" && entry.Content != nil {
		return ExtractTextFromContent(entry.Content)
	}
	// Formato envuelto: {"type":"assistant","message":{"role":"assistant","content":[...]}}
	if entry.Type == "assistant" && entry.Message != nil {
		if entry.Message.Role == "assistant" && entry.Message.Content != nil {
			return ExtractTextFromContent(entry.Message.Content)
		}
	}
	return ""
}

// ExtractNewAssistantMessages lee el transcript JSONL desde el offset dado
// y retorna todos los mensajes assistant nuevos + el nuevo offset.
// Si fromOffset es mayor que el tamaño del archivo (transcript reiniciado), usa offset 0.
// Si fromOffset es 0 y el archivo ya tiene contenido, es una sesion nueva que se abre sobre
// un transcript existente: salta al final sin leer mensajes viejos.
func ExtractNewAssistantMessages(path string, fromOffset int64) (messages []string, newOffset int64, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("abrir transcript %s: %w", path, err)
	}
	defer f.Close()

	// Si es primera lectura (offset 0), saltar al final del archivo.
	// Solo queremos mensajes nuevos a partir de ahora.
	if fromOffset == 0 {
		endOffset, err := f.Seek(0, 2)
		if err != nil {
			return nil, 0, fmt.Errorf("seek al final del transcript: %w", err)
		}
		// Si el archivo ya tiene contenido, es una sesion nueva sobre transcript existente.
		// Saltar al final: no leer mensajes viejos.
		if endOffset > 0 {
			return nil, endOffset, nil
		}
		// Si esta vacio, continuar normal (transcript nuevo): seek de vuelta al inicio.
		if _, err := f.Seek(0, 0); err != nil {
			return nil, 0, fmt.Errorf("seek al inicio del transcript: %w", err)
		}
	}

	// Verificar si el archivo se trunco (nueva sesion) o el offset es valido.
	if fromOffset > 0 {
		info, err := f.Stat()
		if err != nil {
			return nil, 0, fmt.Errorf("stat transcript: %w", err)
		}
		if fromOffset > info.Size() {
			fromOffset = 0
		}
		if _, err := f.Seek(fromOffset, 0); err != nil {
			return nil, 0, fmt.Errorf("seek transcript al offset %d: %w", fromOffset, err)
		}
	}

	reader := bufio.NewReader(f)
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			// Pre-filtro: solo parsear lineas que puedan contener mensajes assistant.
			// Evita parsear JSON de lineas grandes de tool_result u otras entradas irrelevantes.
			if bytes.Contains(line, []byte(`"assistant"`)) {
				var entry TranscriptLine
				if jsonErr := json.Unmarshal(line, &entry); jsonErr == nil {
					text := extractTextFromEntry(entry)
					if text != "" {
						messages = append(messages, text)
					}
				}
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				return nil, 0, fmt.Errorf("leer transcript: %w", readErr)
			}
			break
		}
	}

	// Obtener posicion final del archivo como nuevo offset.
	newOffset, err = f.Seek(0, 2)
	if err != nil {
		return nil, 0, fmt.Errorf("obtener offset final del transcript: %w", err)
	}

	return messages, newOffset, nil
}

// OffsetDir retorna el directorio de offsets: ~/.local/share/agent-speech/offsets/
func OffsetDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".local", "share", "agent-speech", "offsets")
	}
	return filepath.Join(home, ".local", "share", "agent-speech", "offsets")
}

// LoadOffset lee el byte offset guardado para una sesion.
// Retorna 0 si el archivo no existe (primera lectura de la sesion).
func LoadOffset(sessionID string) (int64, error) {
	path := filepath.Join(OffsetDir(), sessionID)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("leer offset de sesion %s: %w", sessionID, err)
	}
	offset, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsear offset de sesion %s: %w", sessionID, err)
	}
	return offset, nil
}

// SaveOffset guarda el byte offset para una sesion.
func SaveOffset(sessionID string, offset int64) error {
	dir := OffsetDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("crear directorio de offsets: %w", err)
	}
	path := filepath.Join(dir, sessionID)
	data := strconv.FormatInt(offset, 10) + "\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		return fmt.Errorf("guardar offset de sesion %s: %w", sessionID, err)
	}
	return nil
}

// ExtractTextFromContent extrae y concatena todos los bloques de texto de un campo content JSON.
// content puede ser string o array de ContentItem.
func ExtractTextFromContent(raw json.RawMessage) string {
	// content puede ser string directamente
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	var items []ContentItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return ""
	}

	var parts []string
	for _, item := range items {
		if item.Type == "text" && item.Text != "" {
			parts = append(parts, item.Text)
		}
	}
	return strings.Join(parts, "")
}
