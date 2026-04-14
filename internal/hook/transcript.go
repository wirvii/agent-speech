package hook

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
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
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry TranscriptLine
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		text := extractTextFromEntry(entry)
		if text != "" {
			lastText = text
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("leer transcript: %w", err)
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
